# Polish — bing-webmaster-tools-pp-cli

Scorecard 80→82. Verify 100%→100%. Dogfood FAIL→PASS. go vet clean. tools-audit 0 pending. Output review 1 WARN→fixed. ship_recommendation: ship; further_polish_recommended: no.

Fixes: removed dead helper `extractResponseData`; renamed `url info`→`url get` (read-verb rule; cleared dogfood naming FAIL); ran mcp-sync (generated tools-manifest.json, propagated rename); added mcp-descriptions.json overrides for 3 thin descriptions (links_remove-connected, page-preview_remove, sites_verify); `sites health` now emits `dataComplete` and suppresses score/grade ("unknown", 0) when ≥3 of 5 backing endpoints fail.

Skipped/structural: insight 2/10 + auth_protocol 4/10 (internal-YAML spec has no bot/bearer scheme to match; insight thresholds penalize this surface — no agent-grade fix); publish-validate printer/phase5 FAILs are promote-stage pipeline state resolved by the main SKILL. Retro candidate: generator should map `Get<Resource>Info` paths to a `get` verb (or accept `info` as a read verb).
