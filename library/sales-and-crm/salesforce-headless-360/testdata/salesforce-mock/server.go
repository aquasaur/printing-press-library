package sfmock

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

const (
	APIVersion = "v63.0"
	apiPrefix  = "/services/data/" + APIVersion

	FailRateLimit              = "rate_limit"
	FailShieldField            = "shield_field"
	FailSharingRestricted      = "sharing_restricted"
	FailCertificateUnavailable = "certificate_unavailable"
)

//go:embed fixtures
var fixtureFS embed.FS

// Server is an in-process Salesforce mock server.
type Server struct {
	URL string

	ts *httptest.Server

	mu       sync.RWMutex
	failMode string
}

type fixtureFile struct {
	Status        int             `json:"status"`
	Envelope      json.RawMessage `json:"envelope"`
	EmptyEnvelope json.RawMessage `json:"empty_envelope"`
}

type route struct {
	method  string
	pattern string
	handler func(http.ResponseWriter, *http.Request, map[string]string)
}

// Start starts a mock server for a test and registers cleanup with t.
func Start(t *testing.T) *Server {
	t.Helper()

	server, err := StartBackground()
	if err != nil {
		if isLocalBindDenied(err) {
			t.Skipf("local listener unavailable in this environment: %v", err)
		}
		t.Fatalf("starting Salesforce mock: %v", err)
	}
	t.Cleanup(server.Close)
	return server
}

func isLocalBindDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "bind: operation not permitted") || strings.Contains(msg, "permission denied")
}

// StartBackground starts a mock server for non-test callers such as doctor.
func StartBackground() (*Server, error) {
	server := &Server{}
	server.SetFailMode(os.Getenv("SF_MOCK_FAIL"))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server.ts = httptest.NewUnstartedServer(server)
	server.ts.Listener = listener
	server.ts.Start()
	server.URL = server.ts.URL
	return server, nil
}

// Close shuts down the mock server.
func (s *Server) Close() {
	if s == nil || s.ts == nil {
		return
	}
	s.ts.Close()
}

// SetFailMode sets a deterministic failure mode for future responses.
func (s *Server) SetFailMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failMode = strings.TrimSpace(mode)
}

func (s *Server) getFailMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failMode
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.getFailMode() == FailRateLimit {
		w.Header().Set("Sforce-Limit-Info", "api-usage=81000/100000")
	}

	if r.URL.Path == "/" {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{
				"mock":        true,
				"api_version": APIVersion,
			})
			return
		}
	}

	for _, rt := range s.routes() {
		if r.Method != rt.method {
			continue
		}
		params, ok := matchPattern(rt.pattern, r.URL.Path)
		if !ok {
			continue
		}
		rt.handler(w, r, params)
		return
	}

	writeSalesforceError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("No mock route for %s %s", r.Method, r.URL.Path))
}

func (s *Server) routes() []route {
	return []route{
		{http.MethodGet, apiPrefix + "/composite/graph", s.handleCompositeGraph},
		{http.MethodPost, apiPrefix + "/composite/graph", s.handleCompositeGraph},
		{http.MethodGet, apiPrefix + "/composite/graph/{fixture}", s.handleCompositeGraph},
		{http.MethodPost, apiPrefix + "/composite/graph/{fixture}", s.handleCompositeGraph},
		{http.MethodGet, apiPrefix + "/ui-api/records/{id}", s.handleUIRecord},
		{http.MethodGet, apiPrefix + "/sobjects/Account/{id}", s.handleSObject},
		{http.MethodGet, apiPrefix + "/sobjects/Contact/{id}", s.handleSObject},
		{http.MethodGet, apiPrefix + "/sobjects/Opportunity/{id}", s.handleSObject},
		{http.MethodGet, apiPrefix + "/sobjects/ContentVersion/{id}/VersionData", s.handleContentVersionData},
		{http.MethodGet, apiPrefix + "/tooling/query", s.handleToolingQuery},
		{http.MethodPost, apiPrefix + "/tooling/sobjects/Certificate", s.handleCertificatePost},
		{http.MethodPost, apiPrefix + "/sobjects/SF360_Bundle_Audit__c", s.handleBundleAuditPost},
		{http.MethodPost, apiPrefix + "/sobjects/SF360_Bundle_Audit__c/", s.handleBundleAuditPost},
		{http.MethodGet, apiPrefix + "/query", s.handleSOQLQuery},
		{http.MethodGet, apiPrefix + "/limits", s.handleLimits},
		{http.MethodPost, apiPrefix + "/connect/data-cloud/oauth2/token", s.handleDataCloudToken},
		{http.MethodGet, apiPrefix + "/connect/data-cloud/unified-profile/{id}", s.handleUnifiedProfile},
	}
}

func (s *Server) handleCompositeGraph(w http.ResponseWriter, r *http.Request, params map[string]string) {
	name := params["fixture"]
	if name == "" {
		name = r.URL.Query().Get("fixture")
	}
	if name == "" {
		name = r.URL.Query().Get("scenario")
	}
	if name == "" {
		name = "acme_small"
	}
	name = strings.TrimSuffix(name, ".json")
	if !strings.HasPrefix(name, "acme_") {
		name = "acme_" + name
	}
	s.writeFixture(w, r, "fixtures/composite_graph/"+name+".json", false)
}

func (s *Server) handleUIRecord(w http.ResponseWriter, r *http.Request, params map[string]string) {
	id := params["id"]
	if s.getFailMode() == FailSharingRestricted && isRestrictedID(id) {
		writeSalesforceError(w, http.StatusForbidden, "INSUFFICIENT_ACCESS", "insufficient access rights on object id")
		return
	}

	switch {
	case id == "001ACME0001" || strings.HasPrefix(id, "001"):
		s.writeFixture(w, r, "fixtures/ui_api/record_account.json", false)
	case id == "003HIDDEN001":
		s.writeFixture(w, r, "fixtures/ui_api/record_contact_fls_hidden.json", false)
	case id == "003ACME0001" || strings.HasPrefix(id, "003"):
		s.writeFixture(w, r, "fixtures/ui_api/record_contact_fls_visible.json", true)
	default:
		s.writeFixture(w, r, "fixtures/ui_api/record_not_found.json", false)
	}
}

func (s *Server) handleSObject(w http.ResponseWriter, r *http.Request, params map[string]string) {
	id := params["id"]
	if s.getFailMode() == FailSharingRestricted && isRestrictedID(id) {
		writeSalesforceError(w, http.StatusForbidden, "INSUFFICIENT_ACCESS", "insufficient access rights on object id")
		return
	}

	switch {
	case strings.Contains(r.URL.Path, "/Account/"):
		writeJSON(w, http.StatusOK, map[string]any{
			"attributes": map[string]any{"type": "Account", "url": apiPrefix + "/sobjects/Account/001ACME0001"},
			"Id":         nonEmpty(id, "001ACME0001"),
			"Name":       "Acme Manufacturing",
			"Industry":   "Manufacturing",
		})
	case strings.Contains(r.URL.Path, "/Contact/"):
		writeJSON(w, http.StatusOK, map[string]any{
			"attributes": map[string]any{"type": "Contact", "url": apiPrefix + "/sobjects/Contact/" + nonEmpty(id, "003ACME0001")},
			"Id":         nonEmpty(id, "003ACME0001"),
			"AccountId":  "001ACME0001",
			"FirstName":  "Avery",
			"LastName":   "Morgan",
			"Email":      "avery.morgan@example.com",
		})
	case strings.Contains(r.URL.Path, "/Opportunity/"):
		writeJSON(w, http.StatusOK, map[string]any{
			"attributes": map[string]any{"type": "Opportunity", "url": apiPrefix + "/sobjects/Opportunity/" + nonEmpty(id, "006ACME0001")},
			"Id":         nonEmpty(id, "006ACME0001"),
			"AccountId":  "001ACME0001",
			"Name":       "Acme Expansion",
			"StageName":  "Negotiation/Review",
			"Amount":     125000,
		})
	default:
		writeSalesforceError(w, http.StatusNotFound, "NOT_FOUND", "requested resource does not exist")
	}
}

func (s *Server) handleToolingQuery(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	if strings.Contains(q, "certificate") {
		s.writeFixture(w, r, "fixtures/tooling/certificate_list.json", false)
		return
	}
	s.writeFixture(w, r, "fixtures/tooling/field_definitions.json", false)
}

func (s *Server) handleCertificatePost(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if s.getFailMode() == FailCertificateUnavailable {
		s.writeFixture(w, r, "fixtures/tooling/certificate_unavailable.json", false)
		return
	}
	s.writeFixture(w, r, "fixtures/tooling/certificate_register.json", false)
}

func (s *Server) handleBundleAuditPost(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      "a00SF360AUDIT",
		"success": true,
		"errors":  []string{},
	})
}

func (s *Server) handleContentVersionData(w http.ResponseWriter, r *http.Request, params map[string]string) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("mock ContentVersion bytes for " + params["id"]))
}

func (s *Server) handleSOQLQuery(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	if strings.Contains(q, "slackconversationrelation") {
		s.writeFixture(w, r, "fixtures/soql/slack_conversation_relation.json", false)
		return
	}

	switch {
	case strings.Contains(q, "contact"):
		writeQueryResponse(w, "Contact", []map[string]any{
			{"Id": "003ACME0001", "AccountId": "001ACME0001", "FirstName": "Avery", "LastName": "Morgan", "Email": "avery.morgan@example.com"},
			{"Id": "003ACME0002", "AccountId": "001ACME0001", "FirstName": "Jordan", "LastName": "Lee", "Email": "jordan.lee@example.com"},
		})
	case strings.Contains(q, "opportunity"):
		writeQueryResponse(w, "Opportunity", []map[string]any{
			{"Id": "006ACME0001", "AccountId": "001ACME0001", "Name": "Acme Expansion", "StageName": "Negotiation/Review", "Amount": 125000},
			{"Id": "006ACME0002", "AccountId": "001ACME0001", "Name": "Acme Renewal", "StageName": "Proposal/Price Quote", "Amount": 86000},
		})
	case strings.Contains(q, "case"):
		writeQueryResponse(w, "Case", []map[string]any{
			{"Id": "500ACME0001", "AccountId": "001ACME0001", "Subject": "Integration latency", "Status": "Working", "Priority": "High"},
			{"Id": "500ACME0002", "AccountId": "001ACME0001", "Subject": "Portal access", "Status": "New", "Priority": "Medium"},
		})
	case strings.Contains(q, "task"):
		writeQueryResponse(w, "Task", []map[string]any{
			{"Id": "00TACME0001", "WhatId": "001ACME0001", "Subject": "Call", "Status": "Completed", "ActivityDate": "2026-04-10"},
		})
	case strings.Contains(q, "event"):
		writeQueryResponse(w, "Event", []map[string]any{
			{"Id": "00UACME0001", "WhatId": "001ACME0001", "Subject": "Executive briefing", "ActivityDate": "2026-04-15"},
		})
	default:
		writeQueryResponse(w, "Account", []map[string]any{
			{"Id": "001ACME0001", "Name": "Acme Manufacturing", "Industry": "Manufacturing"},
		})
	}
}

func (s *Server) handleLimits(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"DailyApiRequests": map[string]any{"Max": 100000, "Remaining": 19000},
		"DailyBulkApiBatches": map[string]any{
			"Max": 15000, "Remaining": 14990,
		},
	})
}

func (s *Server) handleDataCloudToken(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if r.URL.Query().Get("scenario") == "unprovisioned" || strings.Contains(r.Header.Get("X-SF-Mock-Scenario"), "unprovisioned") {
		s.writeFixture(w, r, "fixtures/data_cloud/offcore_unprovisioned.json", false)
		return
	}
	s.writeFixture(w, r, "fixtures/data_cloud/offcore_token.json", false)
}

func (s *Server) handleUnifiedProfile(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	s.writeFixture(w, r, "fixtures/data_cloud/unified_profile.json", false)
}

func (s *Server) writeFixture(w http.ResponseWriter, r *http.Request, path string, allowShieldField bool) {
	data, err := fixtureFS.ReadFile(path)
	if err != nil {
		writeSalesforceError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("fixture %s not found", path))
		return
	}

	var fixture fixtureFile
	if err := json.Unmarshal(data, &fixture); err != nil {
		writeSalesforceError(w, http.StatusInternalServerError, "SERVER_ERROR", fmt.Sprintf("fixture %s is invalid JSON: %v", path, err))
		return
	}

	envelope := fixture.Envelope
	if len(envelope) == 0 {
		writeSalesforceError(w, http.StatusInternalServerError, "SERVER_ERROR", fmt.Sprintf("fixture %s has no envelope", path))
		return
	}
	if path == "fixtures/soql/slack_conversation_relation.json" && strings.Contains(strings.ToLower(r.URL.Query().Get("q")), "empty") && len(fixture.EmptyEnvelope) > 0 {
		envelope = fixture.EmptyEnvelope
	}
	if allowShieldField && s.getFailMode() == FailShieldField {
		envelope = withShieldField(envelope)
	}

	status := fixture.Status
	if status == 0 {
		status = http.StatusOK
	}
	writeRawJSON(w, status, envelope)
}

func writeQueryResponse(w http.ResponseWriter, recordType string, records []map[string]any) {
	for _, record := range records {
		if _, ok := record["attributes"]; !ok {
			id, _ := record["Id"].(string)
			record["attributes"] = map[string]any{
				"type": recordType,
				"url":  apiPrefix + "/sobjects/" + recordType + "/" + id,
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalSize": len(records),
		"done":      true,
		"records":   records,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeSalesforceError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	writeRawJSON(w, status, data)
}

func writeRawJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeSalesforceError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, []map[string]string{{
		"errorCode": code,
		"message":   message,
	}})
}

func matchPattern(pattern, path string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := map[string]string{}
	for i, patternPart := range patternParts {
		pathPart := pathParts[i]
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			params[strings.TrimSuffix(strings.TrimPrefix(patternPart, "{"), "}")] = pathPart
			continue
		}
		if patternPart != pathPart {
			return nil, false
		}
	}
	return params, true
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func withShieldField(envelope json.RawMessage) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(envelope, &obj); err != nil {
		return envelope
	}
	fields, _ := obj["fields"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
		obj["fields"] = fields
	}
	salary, _ := fields["Salary__c"].(map[string]any)
	if salary == nil {
		salary = map[string]any{"value": 142000, "displayValue": "$142,000"}
		fields["Salary__c"] = salary
	}
	salary["IsEncrypted"] = true
	data, err := json.Marshal(obj)
	if err != nil {
		return envelope
	}
	return data
}

func isRestrictedID(id string) bool {
	switch id {
	case "003RESTRICTED001", "003RESTRICTED002":
		return true
	default:
		return false
	}
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// ReadAll is a tiny helper for tests that want response bodies as strings.
func ReadAll(resp *http.Response) string {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}
