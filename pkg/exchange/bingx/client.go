package bingx

import (
	"arb/pkg/exchange"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

const (
	defaultBaseURL = "https://open-api.bingx.com"
)

// retryable BingX API error codes.
var retryableCodes = map[int]bool{
	100001: true, // signature failure (clock skew)
	100500: true, // internal server error
}

// APIResponse is the top-level BingX API response wrapper.
type APIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// APIError represents a BingX API error.
type APIError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bingx API error code=%d msg=%s", e.Code, e.Msg)
}

// Client is a low-level HTTP client for the BingX REST API.
type Client struct {
	baseURL         string
	apiKey          string
	secretKey       string
	httpClient      *http.Client
	metricsCallback exchange.MetricsCallback

	// Rate limiter: serialize signed API calls with minimum interval
	// to avoid BingX 100410 frequency limit bans.
	rateMu   sync.Mutex
	lastCall time.Time
}

// NewClient creates a new BingX REST API client.
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetMetricsCallback(fn exchange.MetricsCallback) {
	c.metricsCallback = fn
}

// sign computes the HMAC-SHA256 signature (hex encoded).
// BingX signature: sort params alphabetically, join with &, HMAC-SHA256 with secretKey.
func (c *Client) sign(paramStr string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(paramStr))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildParamString constructs a sorted query string from parameters (with timestamp).
// Returns the raw (unencoded) string for signing per BingX docs.
func buildParamString(params map[string]string) string {
	// Add timestamp
	params["timestamp"] = fmt.Sprintf("%d", time.Now().UnixMilli())

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

// encodeParamString URL-encodes a raw param string for use in request URLs/bodies.
func encodeParamString(raw string) string {
	pairs := strings.Split(raw, "&")
	encoded := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			encoded = append(encoded, url.QueryEscape(kv[0])+"="+url.QueryEscape(kv[1]))
		}
	}
	return strings.Join(encoded, "&")
}

// Get performs an authenticated GET request with retry logic.
func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error) {
	return c.retryDo("GET", path, params, 3)
}

// Post performs an authenticated POST request with retry logic.
func (c *Client) Post(path string, params map[string]string) (json.RawMessage, error) {
	return c.retryDo("POST", path, params, 3)
}

// Delete performs an authenticated DELETE request with retry logic.
func (c *Client) Delete(path string, params map[string]string) (json.RawMessage, error) {
	return c.retryDo("DELETE", path, params, 3)
}

// DoRequestRaw performs an authenticated HTTP request and returns the raw response body.
// Use for endpoints that don't follow the standard {"code":0,"data":...} wrapper.
func (c *Client) DoRequestRaw(method, path string, params map[string]string) ([]byte, error) {
	start := time.Now()
	var err error
	defer func() {
		if c.metricsCallback != nil {
			c.metricsCallback(path, time.Since(start), err)
		}
	}()

	if params == nil {
		params = make(map[string]string)
	}
	paramStr := buildParamString(params)
	signature := c.sign(paramStr)
	// URL-encode for the actual request
	encodedParams := encodeParamString(paramStr) + "&signature=" + signature

	var fullURL string
	var reqBody io.Reader

	switch method {
	case "GET", "DELETE":
		fullURL = c.baseURL + path + "?" + encodedParams
	case "POST", "PUT":
		fullURL = c.baseURL + path
		reqBody = strings.NewReader(encodedParams)
	default:
		fullURL = c.baseURL + path + "?" + encodedParams
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-BX-APIKEY", c.apiKey)
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// doRequest performs a single authenticated HTTP request.
func (c *Client) doRequest(method, path string, params map[string]string) (json.RawMessage, error) {
	start := time.Now()
	var err error
	defer func() {
		if c.metricsCallback != nil {
			c.metricsCallback(path, time.Since(start), err)
		}
	}()

	if params == nil {
		params = make(map[string]string)
	}

	// Build sorted param string with timestamp, then sign raw (unencoded) string
	paramStr := buildParamString(params)
	signature := c.sign(paramStr)
	// URL-encode for the actual request
	encodedParams := encodeParamString(paramStr) + "&signature=" + signature

	var fullURL string
	var reqBody io.Reader

	switch method {
	case "GET", "DELETE":
		fullURL = c.baseURL + path + "?" + encodedParams
	case "POST":
		fullURL = c.baseURL + path
		reqBody = strings.NewReader(encodedParams)
	default:
		fullURL = c.baseURL + path + "?" + encodedParams
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-BX-APIKEY", c.apiKey)
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		err = fmt.Errorf("bingx: unmarshal response: %w (body=%s)", err, string(data))
		return nil, err
	}

	if apiResp.Code != 0 {
		err = &APIError{Code: apiResp.Code, Msg: apiResp.Msg}
		return nil, err
	}

	return apiResp.Data, nil
}

// isRetryable checks whether an error is transient and worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for BingX API error codes.
	if apiErr, ok := err.(*APIError); ok {
		return retryableCodes[apiErr.Code]
	}

	// Check for network errors.
	errMsg := err.Error()
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "connection reset")
}

// retryDo performs an HTTP request with exponential backoff retry on transient errors.
// All calls are serialized with a minimum 150ms interval to stay under BingX's
// 10/s per-UID rate limit across concurrent goroutines.
func (c *Client) retryDo(method, path string, params map[string]string, maxRetries int) (json.RawMessage, error) {
	// Rate limit: serialize and enforce minimum interval between calls.
	c.rateMu.Lock()
	if elapsed := time.Since(c.lastCall); elapsed < 150*time.Millisecond {
		time.Sleep(150*time.Millisecond - elapsed)
	}
	c.lastCall = time.Now()
	c.rateMu.Unlock()

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Copy params for each attempt (buildParamString mutates the map by adding timestamp)
		p := make(map[string]string, len(params))
		for k, v := range params {
			p[k] = v
		}

		result, err := c.doRequest(method, path, p)
		if err == nil {
			return result, nil
		}

		if attempt < maxRetries && isRetryable(err) {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
			lastErr = err
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("%s %s: exhausted %d retries, last error: %w", method, path, maxRetries, lastErr)
}
