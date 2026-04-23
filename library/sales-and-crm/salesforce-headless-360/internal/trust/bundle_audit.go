package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

const BundleAuditPath = "/services/data/" + APIVersion + "/sobjects/SF360_Bundle_Audit__c/"

type BundleAuditClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

type BundleAuditRequest struct {
	KID             string
	GeneratedBy     string
	GeneratedAt     time.Time
	AccountID       string
	BundleJTI       string
	SourcesUsed     []string
	RedactionCounts map[string]int
	ClientHost      string
	TraceID         string
	HIPAAMode       bool
}

type BundleAuditOptions struct {
	Client  BundleAuditClient
	DBPath  string
	Sync    bool
	LogWarn func(format string, args ...any)
}

func RecordBundleAudit(ctx context.Context, req BundleAuditRequest, opts BundleAuditOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	req = normalizeBundleAuditRequest(req)
	if err := writeBundleAuditLocal(opts.DBPath, req, "pending", ""); err != nil {
		if opts.Sync || req.HIPAAMode {
			return fmt.Errorf("BUNDLE_AUDIT_LOCAL_WRITE_FAILED: %w", err)
		}
		warnBundleAudit(opts, "bundle audit local mirror write failed: %v", err)
	}

	if opts.Sync || req.HIPAAMode {
		return postAndMirrorBundleAudit(ctx, req, opts)
	}

	go func() {
		if err := postAndMirrorBundleAudit(context.Background(), req, opts); err != nil {
			warnBundleAudit(opts, "bundle audit write failed: %v", err)
		}
	}()
	return nil
}

func normalizeBundleAuditRequest(req BundleAuditRequest) BundleAuditRequest {
	if req.GeneratedAt.IsZero() {
		req.GeneratedAt = time.Now().UTC()
	} else {
		req.GeneratedAt = req.GeneratedAt.UTC()
	}
	if req.AccountID == "" {
		req.AccountID = "unknown"
	}
	if req.BundleJTI == "" {
		req.BundleJTI = req.TraceID
	}
	if req.TraceID == "" {
		req.TraceID = req.BundleJTI
	}
	if req.RedactionCounts == nil {
		req.RedactionCounts = map[string]int{}
	}
	if req.SourcesUsed == nil {
		req.SourcesUsed = []string{}
	}
	if req.ClientHost == "" {
		req.ClientHost, _ = os.Hostname()
	}
	return req
}

func postAndMirrorBundleAudit(ctx context.Context, req BundleAuditRequest, opts BundleAuditOptions) error {
	if opts.Client == nil {
		err := fmt.Errorf("bundle audit client required")
		_ = writeBundleAuditLocal(opts.DBPath, req, "failed", err.Error())
		return fmt.Errorf("BUNDLE_AUDIT_WRITE_FAILED: %w", err)
	}
	body, err := bundleAuditSObject(req)
	if err != nil {
		_ = writeBundleAuditLocal(opts.DBPath, req, "failed", err.Error())
		return fmt.Errorf("BUNDLE_AUDIT_WRITE_FAILED: %w", err)
	}

	done := make(chan auditPostResult, 1)
	go func() {
		raw, status, postErr := opts.Client.Post(BundleAuditPath, body)
		done <- auditPostResult{raw: raw, status: status, err: postErr}
	}()

	var result auditPostResult
	select {
	case <-ctx.Done():
		err := ctx.Err()
		_ = writeBundleAuditLocal(opts.DBPath, req, "failed", err.Error())
		return fmt.Errorf("BUNDLE_AUDIT_WRITE_FAILED: %w", err)
	case result = <-done:
	}

	if result.err != nil {
		_ = writeBundleAuditLocal(opts.DBPath, req, "failed", result.err.Error())
		return fmt.Errorf("BUNDLE_AUDIT_WRITE_FAILED: %w", result.err)
	}
	if result.status < 200 || result.status > 299 {
		err := fmt.Errorf("remote returned HTTP %d: %s", result.status, string(result.raw))
		_ = writeBundleAuditLocal(opts.DBPath, req, "failed", err.Error())
		return fmt.Errorf("BUNDLE_AUDIT_WRITE_FAILED: %w", err)
	}
	if err := writeBundleAuditLocal(opts.DBPath, req, "ok", ""); err != nil {
		return fmt.Errorf("BUNDLE_AUDIT_LOCAL_WRITE_FAILED: %w", err)
	}
	return nil
}

type auditPostResult struct {
	raw    json.RawMessage
	status int
	err    error
}

func bundleAuditSObject(req BundleAuditRequest) (map[string]any, error) {
	sources, err := json.Marshal(req.SourcesUsed)
	if err != nil {
		return nil, err
	}
	redactions, err := json.Marshal(req.RedactionCounts)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"GeneratedBy__c":     req.GeneratedBy,
		"GeneratedAt__c":     req.GeneratedAt.Format(time.RFC3339),
		"AccountId__c":       req.AccountID,
		"BundleJti__c":       req.BundleJTI,
		"SourcesUsed__c":     string(sources),
		"RedactionCounts__c": string(redactions),
		"ClientHost__c":      req.ClientHost,
		"TraceId__c":         req.TraceID,
		"HipaaMode__c":       req.HIPAAMode,
	}, nil
}

func writeBundleAuditLocal(dbPath string, req BundleAuditRequest, status, remoteError string) error {
	if dbPath == "" {
		return nil
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.RecordBundleAuditLocal(req.KID, req.BundleJTI, req.AccountID, req.GeneratedAt, status, remoteError)
}

func warnBundleAudit(opts BundleAuditOptions, format string, args ...any) {
	if opts.LogWarn != nil {
		opts.LogWarn(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

// HIPAAModeFromManifest reads the install-time provenance flag. Accepted keys
// are intentionally permissive so older scaffold manifests can opt in without
// another schema migration.
func HIPAAModeFromManifest(path string) bool {
	if strings.EqualFold(os.Getenv("SF360_HIPAA_MODE"), "true") || os.Getenv("SF360_HIPAA_MODE") == "1" {
		return true
	}
	if path == "" {
		path = ".printing-press.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	return boolKey(manifest, "hipaa_mode") || boolKey(manifest, "hipaa") || boolKey(manifest, "sync_audit_writes")
}

func boolKey(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
