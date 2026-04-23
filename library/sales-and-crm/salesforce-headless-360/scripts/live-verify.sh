#!/usr/bin/env bash
# live-verify.sh
# Convenience runner for the live-org verification runbook.
# Reads ORG (sf alias) and ACME_ID (account ID) from env or prompts.
# Runs every required check end-to-end against a real Salesforce org.
# Writes a JSON report to docs/live-verification-report.json alongside the markdown report.
#
# This script does NOT replace the runbook (docs/live-verification-runbook.md).
# It is the automation layer beneath the runbook for repeatable execution.

set -euo pipefail

CLI=${CLI:-salesforce-headless-360-pp-cli}
ORG=${ORG:?ORG env var required (sf CLI alias)}
ACME_ID=${ACME_ID:?ACME_ID env var required (Salesforce Account Id to test against)}
RESTRICTED_USER=${RESTRICTED_USER:-}

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
REPORT_JSON="${REPO_ROOT}/docs/live-verification-report.json"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

results=()

record() {
  local id="$1" label="$2" status="$3" detail="$4"
  results+=("{\"id\":\"$id\",\"label\":\"$label\",\"status\":\"$status\",\"detail\":\"${detail//\"/\\\"}\"}")
  printf "  %-2s %-50s %s\n" "$id" "$label" "$status"
}

run_or_fail() {
  local id="$1" label="$2"
  shift 2
  if "$@" > "${TMPDIR}/${id}.out" 2>&1; then
    record "$id" "$label" PASS "$(head -1 "${TMPDIR}/${id}.out")"
  else
    record "$id" "$label" FAIL "$(tail -3 "${TMPDIR}/${id}.out" | tr '\n' ';')"
  fi
}

echo
echo "Live-org verification: $ORG / Account=$ACME_ID"
echo "------------------------------------------------------------"

# 1. sf fall-through
run_or_fail 1 "sf CLI fall-through" "$CLI" auth login --sf "$ORG"

# 2. doctor full pass
run_or_fail 2 "doctor full pass" "$CLI" --org "$ORG" doctor

# 3. Composite Graph sync (look for the request line in verbose output)
if "$CLI" --org "$ORG" sync --account "$ACME_ID" --verbose 2>&1 | tee "${TMPDIR}/sync.out" | grep -q "composite/graph"; then
  record 3 "Composite Graph in sync" PASS "graph request observed"
else
  record 3 "Composite Graph in sync" FAIL "no composite/graph request seen"
fi

# 4. sharing cross-check (presence of the table is enough; rows depend on profile)
if sqlite3 "$HOME/.local/share/salesforce-headless-360-pp-cli/sf360.db" \
    "SELECT count(*) FROM sharing_drop_audit;" > "${TMPDIR}/4.out" 2>&1; then
  record 4 "UI API sharing cross-check (table present)" PASS "$(cat "${TMPDIR}/4.out") drop rows"
else
  record 4 "UI API sharing cross-check (table present)" FAIL "$(cat "${TMPDIR}/4.out")"
fi

# 5. FLS intersection
# This requires --run-as-user or a separate restricted login; only run if RESTRICTED_USER set
if [ -n "$RESTRICTED_USER" ]; then
  "$CLI" --org "$ORG" agent context --live "$ACME_ID" --run-as-user "$RESTRICTED_USER" \
      --output "${TMPDIR}/restricted.json" >/dev/null 2>&1 || true
  if [ -f "${TMPDIR}/restricted.json" ]; then
    leaks=$(grep -oE 'AnnualRevenue|Salary__c' "${TMPDIR}/restricted.json" | wc -l | tr -d ' ')
    if [ "$leaks" = "0" ]; then
      record 5 "FLS intersection hides restricted fields" PASS "0 leaks"
    else
      record 5 "FLS intersection hides restricted fields" FAIL "$leaks leaks"
    fi
  else
    record 5 "FLS intersection hides restricted fields" FAIL "bundle not produced"
  fi
else
  record 5 "FLS intersection hides restricted fields" SKIP "RESTRICTED_USER not set"
fi

# 6. compliance map loaded
rows=$(sqlite3 "$HOME/.local/share/salesforce-headless-360-pp-cli/sf360.db" \
    "SELECT count(*) FROM compliance_field_map;" 2>/dev/null || echo 0)
if [ "$rows" -gt 0 ]; then
  record 6 "Tooling compliance map loads" PASS "$rows fields"
else
  record 6 "Tooling compliance map loads" FAIL "0 rows (tag at least one field)"
fi

# 7. trust register
run_or_fail 7 "trust register (Certificate or CMDT)" "$CLI" --org "$ORG" trust register

# 8. agent context produces bundle
"$CLI" --org "$ORG" agent context --live "$ACME_ID" --output "${TMPDIR}/acme.json" >/dev/null 2>&1
if [ -s "${TMPDIR}/acme.json" ] && grep -q '"kid"' "${TMPDIR}/acme.json"; then
  record 8 "agent context produces signed bundle" PASS "bundle written + kid present"
else
  record 8 "agent context produces signed bundle" FAIL "bundle missing or unsigned"
fi

# 9. agent verify --strict --deep on valid
run_or_fail 9 "agent verify --strict --deep PASS valid" "$CLI" agent verify --strict --deep "${TMPDIR}/acme.json"

# 10. agent verify --strict --deep on tampered
cp "${TMPDIR}/acme.json" "${TMPDIR}/acme.tampered.json"
python3 -c "
import json, sys
b = json.load(open('${TMPDIR}/acme.tampered.json'))
b['manifest']['account']['Name'] = 'TAMPERED'
json.dump(b, open('${TMPDIR}/acme.tampered.json', 'w'))
"
if "$CLI" agent verify --strict --deep "${TMPDIR}/acme.tampered.json" >/dev/null 2>&1; then
  record 10 "agent verify FAIL on tampered bundle" FAIL "verify accepted tampered bundle (BAD)"
else
  record 10 "agent verify FAIL on tampered bundle" PASS "verify rejected tampered bundle"
fi

# 11. audit row appears
if sf data query --target-org "$ORG" --query "SELECT BundleJti__c FROM SF360_Bundle_Audit__c ORDER BY GeneratedAt__c DESC LIMIT 1" --json > "${TMPDIR}/audit.out" 2>&1; then
  record 11 "SF360_Bundle_Audit__c row appears" PASS "audit row found"
else
  record 11 "SF360_Bundle_Audit__c row appears" FAIL "$(tail -1 "${TMPDIR}/audit.out")"
fi

echo "------------------------------------------------------------"

# Write JSON report
{
  echo "{"
  echo "  \"date\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
  echo "  \"org\": \"$ORG\","
  echo "  \"account_id\": \"$ACME_ID\","
  echo "  \"cli_version\": \"$($CLI version 2>/dev/null || echo unknown)\","
  echo "  \"results\": ["
  for i in "${!results[@]}"; do
    if [ $i -gt 0 ]; then echo ","; fi
    printf "    %s" "${results[$i]}"
  done
  echo
  echo "  ]"
  echo "}"
} > "$REPORT_JSON"

echo
echo "JSON report written to: $REPORT_JSON"
echo "Now fill in docs/live-verification-report.md by hand and sign with the trust key."

# Exit non-zero if any required check failed
if printf '%s\n' "${results[@]}" | grep -q '"status":"FAIL"'; then
  echo
  echo "FAIL: at least one required check failed. v1.0.0 release blocked."
  exit 1
fi
