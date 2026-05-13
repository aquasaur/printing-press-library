// Package fuelcheck is the hand-built client for the NSW FuelCheck
// (FuelPriceCheck) API. FuelCheck lives on a different host from the TfNSW
// Open Data Hub and uses OAuth2 client-credentials plus extra request headers,
// so it cannot share the generated client's api_key auth; the printed CLI's
// `fuel ...` commands call into this package instead.
package fuelcheck

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/cliutil"
)

const (
	tokenURL = "https://api.onegov.nsw.gov.au/oauth/client_credential/accesstoken?grant_type=client_credentials"
	apiBase  = "https://api.onegov.nsw.gov.au/FuelPriceCheck/v1/fuel"
)

// MissingCredsError signals that the FuelCheck credentials are not configured.
// The CLI surfaces this with config exit code 10.
type MissingCredsError struct{}

func (MissingCredsError) Error() string {
	return "FuelCheck credentials not configured: set NSW_FUELCHECK_API_KEY and NSW_FUELCHECK_API_SECRET (register a FuelCheck app at https://api.nsw.gov.au/Product/Index/22)"
}

// Client talks to the FuelCheck API. It caches the OAuth bearer token on disk
// so repeated invocations do not re-authenticate.
type Client struct {
	apiKey    string
	apiSecret string
	http      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	tokenPath string
}

// New builds a client from the environment. Returns MissingCredsError if either
// credential is unset.
func New(timeout time.Duration) (*Client, error) {
	return NewWithCreds(os.Getenv("NSW_FUELCHECK_API_KEY"), os.Getenv("NSW_FUELCHECK_API_SECRET"), timeout)
}

// NewWithCreds builds a client from explicitly-supplied credentials (typically
// resolved by the CLI layer from config file or env). Returns MissingCredsError
// if either is empty.
func NewWithCreds(key, secret string, timeout time.Duration) (*Client, error) {
	key = strings.TrimSpace(key)
	secret = strings.TrimSpace(secret)
	if key == "" || secret == "" {
		return nil, MissingCredsError{}
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	home, _ := os.UserHomeDir()
	return &Client{
		apiKey:    key,
		apiSecret: secret,
		http:      &http.Client{Timeout: timeout},
		limiter:   cliutil.NewAdaptiveLimiter(1.0),
		tokenPath: filepath.Join(home, ".config", "nsw-transport-pp-cli", "fuelcheck-token.json"),
	}, nil
}

type cachedToken struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (c *Client) token(ctx context.Context) (string, error) {
	if tok := c.readCachedToken(); tok != "" {
		return tok, nil
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(c.apiKey + ":" + c.apiSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("FuelCheck token request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("FuelCheck rejected the credentials (HTTP %d) — check NSW_FUELCHECK_API_KEY / NSW_FUELCHECK_API_SECRET", resp.StatusCode)
		}
		return "", fmt.Errorf("FuelCheck token request failed: HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("FuelCheck token response not understood: %s", truncate(body))
	}
	ttl := 11 * time.Hour
	if secs := parseSeconds(tr.ExpiresIn); secs > 0 {
		ttl = time.Duration(secs)*time.Second - 5*time.Minute
	}
	c.writeCachedToken(cachedToken{AccessToken: tr.AccessToken, ExpiresAt: time.Now().Add(ttl)})
	return tr.AccessToken, nil
}

func (c *Client) readCachedToken() string {
	b, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return ""
	}
	var ct cachedToken
	if json.Unmarshal(b, &ct) != nil {
		return ""
	}
	if ct.AccessToken == "" || time.Now().After(ct.ExpiresAt) {
		return ""
	}
	return ct.AccessToken
}

func (c *Client) writeCachedToken(ct cachedToken) {
	_ = os.MkdirAll(filepath.Dir(c.tokenPath), 0o755)
	if b, err := json.Marshal(ct); err == nil {
		_ = os.WriteFile(c.tokenPath, b, 0o600)
	}
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	tok, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	u := apiBase + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reqBody io.Reader
	if body != nil {
		bb, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(bb)
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("transactionid", newTransactionID())
	req.Header.Set("requesttimestamp", time.Now().Format("02/01/2006 03:04:05 PM"))
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "nsw-transport-pp-cli")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		c.limiter.OnSuccess()
		return rb, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp), Body: truncate(rb)}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// Drop the cached token so the next call re-authenticates.
		_ = os.Remove(c.tokenPath)
		return nil, fmt.Errorf("FuelCheck request to %s rejected (HTTP %d): %s", path, resp.StatusCode, truncate(rb))
	default:
		return nil, fmt.Errorf("FuelCheck request to %s failed: HTTP %d: %s", path, resp.StatusCode, truncate(rb))
	}
}

// --- response types ---

type Station struct {
	StationCode FlexStr `json:"code"`
	Brand       string  `json:"brand"`
	Name        string  `json:"name"`
	Address     string  `json:"address,omitempty"`
	State       string  `json:"state,omitempty"`
	Location    struct {
		// FuelCheck returns these as JSON numbers on /prices but as JSON-encoded
		// strings on /prices/nearby; FlexNum tolerates both.
		Latitude  FlexNum `json:"latitude"`
		Longitude FlexNum `json:"longitude"`
		Distance  FlexNum `json:"distance,omitempty"`
	} `json:"location"`
}

// FlexNum unmarshals from a JSON number or a JSON-encoded numeric string.
type FlexNum float64

func (n *FlexNum) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` || s == "" {
		*n = 0
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		*n = 0
		return nil
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return fmt.Errorf("FlexNum: cannot parse %q", s)
	}
	*n = FlexNum(f)
	return nil
}

func (n FlexNum) Float() float64 { return float64(n) }

// FlexStr unmarshals from a JSON string OR a JSON number — FuelCheck returns
// station codes as strings on /prices but as numbers on /prices/nearby.
type FlexStr string

func (s *FlexStr) UnmarshalJSON(b []byte) error {
	t := strings.TrimSpace(string(b))
	if t == "null" {
		*s = ""
		return nil
	}
	*s = FlexStr(strings.Trim(t, `"`))
	return nil
}

func (s FlexStr) String() string { return string(s) }

type Price struct {
	StationCode FlexStr `json:"stationcode"`
	FuelType    string  `json:"fueltype"`
	Price       FlexNum `json:"price"`
	LastUpdated string  `json:"lastupdated,omitempty"`
}

type PricesResponse struct {
	Stations []Station `json:"stations"`
	Prices   []Price   `json:"prices"`
}

type StationResponse struct {
	Station Station `json:"station"`
	Prices  []Price `json:"prices"`
}

type LovsResponse struct {
	FuelTypes []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"fueltypes"`
	Brands []struct {
		BrandID any    `json:"brandid"`
		Name    string `json:"name"`
	} `json:"brands"`
	SortFields []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"sortfields"`
}

// --- endpoint methods ---

// Prices returns all current NSW fuel prices.
func (c *Client) Prices(ctx context.Context) (*PricesResponse, error) {
	b, err := c.do(ctx, http.MethodGet, "/prices", nil, nil)
	if err != nil {
		return nil, err
	}
	var pr PricesResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /prices: %w", err)
	}
	return &pr, nil
}

// PricesNew returns prices updated since this apikey's last call today.
func (c *Client) PricesNew(ctx context.Context) (*PricesResponse, error) {
	b, err := c.do(ctx, http.MethodGet, "/prices/new", nil, nil)
	if err != nil {
		return nil, err
	}
	var pr PricesResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /prices/new: %w", err)
	}
	return &pr, nil
}

// NearbyRequest is the body for POST /prices/nearby.
type NearbyRequest struct {
	FuelType      string   `json:"fueltype"`
	Latitude      float64  `json:"latitude"`
	Longitude     float64  `json:"longitude"`
	Radius        float64  `json:"radius"` // km
	Brand         []string `json:"brand,omitempty"`
	SortBy        string   `json:"sortby,omitempty"`
	SortAscending bool     `json:"sortascending"`
}

// PricesNearby returns stations + prices within a radius of a coordinate.
func (c *Client) PricesNearby(ctx context.Context, req NearbyRequest) (*PricesResponse, error) {
	if req.SortBy == "" {
		req.SortBy = "price"
	}
	b, err := c.do(ctx, http.MethodPost, "/prices/nearby", nil, req)
	if err != nil {
		return nil, err
	}
	var pr PricesResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /prices/nearby: %w", err)
	}
	return &pr, nil
}

// PricesByStation returns current prices for one station code.
func (c *Client) PricesByStation(ctx context.Context, code string) (*StationResponse, error) {
	b, err := c.do(ctx, http.MethodGet, "/prices/station/"+url.PathEscape(code), nil, nil)
	if err != nil {
		return nil, err
	}
	var sr StationResponse
	if err := json.Unmarshal(b, &sr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /prices/station: %w", err)
	}
	return &sr, nil
}

// LocationRequest is the body for POST /prices/location.
type LocationRequest struct {
	FuelType       string   `json:"fueltype"`
	NamedLocation  string   `json:"namedlocation,omitempty"`
	StateTerritory string   `json:"stateTerritory,omitempty"`
	Suburb         string   `json:"suburb,omitempty"`
	Postcode       string   `json:"postcode,omitempty"`
	Brands         []string `json:"brands,omitempty"`
}

// PricesByLocation returns prices for a named location (suburb/postcode).
func (c *Client) PricesByLocation(ctx context.Context, req LocationRequest) (*PricesResponse, error) {
	b, err := c.do(ctx, http.MethodPost, "/prices/location", nil, req)
	if err != nil {
		return nil, err
	}
	var pr PricesResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /prices/location: %w", err)
	}
	return &pr, nil
}

// Lovs returns the FuelCheck reference lists (fuel types, brands, sort fields).
func (c *Client) Lovs(ctx context.Context) (*LovsResponse, error) {
	b, err := c.do(ctx, http.MethodGet, "/lovs", nil, nil)
	if err != nil {
		return nil, err
	}
	var lr LovsResponse
	if err := json.Unmarshal(b, &lr); err != nil {
		return nil, fmt.Errorf("decoding FuelCheck /lovs: %w", err)
	}
	return &lr, nil
}

// Trends returns statewide (station empty) or per-station fuel price trends
// over a period (DAY, WEEK, MONTH, YEAR). The endpoint is documented in the
// NSW API portal but not cross-validated by community clients; if the response
// shape differs the raw bytes are returned for the caller to surface.
func (c *Client) Trends(ctx context.Context, fueltype, period, station string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("fueltype", fueltype)
	if period != "" {
		q.Set("period", strings.ToUpper(period))
	}
	path := "/prices/trends"
	if station != "" {
		path = "/prices/station/" + url.PathEscape(station) + "/trends"
	}
	b, err := c.do(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func newTransactionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func parseSeconds(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		var n int
		_, _ = fmt.Sscanf(t, "%d", &n)
		return n
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	}
	return 0
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
