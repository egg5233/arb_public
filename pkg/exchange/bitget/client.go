package bitget

import (
	"arb/pkg/exchange"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL                = "https://api.bitget.com"
	productTypeUSDTFutures = "USDT-FUTURES"
	marginCoinUSDT         = "USDT"
)

// retryable API response codes
var retryableCodes = map[string]bool{
	"429":   true, // HTTP rate limit
	"40900": true, // Bitget server-side throttling
}

// bitgetPassThroughCodes lists bitget error codes whose semantics are treated
// as success by existing adapter call sites (idempotent operations). Each must
// have a documented call site that relies on the code being preserved.
var bitgetPassThroughCodes = map[string]bool{
	"40872": true, // SetMarginMode: margin mode already set (adapter.go:401)
	"43011": true, // CancelOrder / CancelStopLoss: order already finalized (adapter.go:175, 1376)
	"43025": true, // CancelOrder / CancelStopLoss: same semantic family (adapter.go:175, 1376)
}

// APIError is a structured bitget API error, carrying the error code and
// message from the response envelope or synthesized from an HTTP status.
type APIError struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bitget API error %s: %s", e.Code, e.Msg)
}

// Client is a self-contained REST client for the Bitget v2 API.
// It handles HMAC-SHA256 + base64 signing with passphrase support.
type Client struct {
	apiKey          string
	secretKey       string
	passphrase      string
	baseURL         string
	httpClient      *http.Client
	metricsCallback exchange.MetricsCallback
}

// NewClient creates a new Bitget REST API client.
func NewClient(apiKey, secretKey, passphrase string) *Client {
	return &Client{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetMetricsCallback(fn exchange.MetricsCallback) {
	c.metricsCallback = fn
}

// sign generates the HMAC-SHA256 signature for Bitget API authentication.
// The message format is: timestamp + method + requestPath + body
func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// buildQueryString constructs a sorted query string from parameters.
func buildQueryString(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	return strings.Join(parts, "&")
}

// Get performs an authenticated GET request with retry logic.
func (c *Client) Get(path string, params map[string]string) (string, error) {
	return c.retryDo("GET", path, params, nil, 3)
}

// Post performs an authenticated POST request with retry logic.
func (c *Client) Post(path string, params map[string]string) (string, error) {
	return c.retryDo("POST", path, nil, params, 3)
}

// doRequest performs a single authenticated HTTP request.
func (c *Client) doRequest(method, path string, queryParams, bodyParams map[string]string) (string, error) {
	start := time.Now()
	var err error
	defer func() {
		if c.metricsCallback != nil {
			c.metricsCallback(path, time.Since(start), err)
		}
	}()

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	var requestPath string
	var bodyStr string
	var reqBody io.Reader

	if method == "GET" {
		qs := buildQueryString(queryParams)
		if qs != "" {
			requestPath = path + "?" + qs
		} else {
			requestPath = path
		}
		bodyStr = ""
	} else {
		requestPath = path
		if len(bodyParams) > 0 {
			var b []byte
			b, err = json.Marshal(bodyParams)
			if err != nil {
				return "", fmt.Errorf("marshal body: %w", err)
			}
			bodyStr = string(b)
			reqBody = bytes.NewReader(b)
		}
	}

	signature := c.sign(timestamp, method, requestPath, bodyStr)

	var req *http.Request
	// Use a context timeout as a hard safety net in addition to http.Client.Timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if method == "GET" {
		// Build request with base path only, then set RawQuery directly
		// to avoid http.NewRequest re-encoding percent-escaped non-ASCII
		// characters (e.g. Chinese symbols like 龙虾USDT).
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
		if err != nil {
			return "", err
		}
		qs := buildQueryString(queryParams)
		if qs != "" {
			req.URL.RawQuery = qs
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return "", err
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ACCESS-KEY", c.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", c.passphrase)
	req.Header.Set("locale", "en-US")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse envelope once for downstream branching.
	var envelope struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(data, &envelope)

	// Idempotent codes that existing adapter methods treat as success (line numbers
	// verified against HEAD adapter.go):
	// - 43011 "order already cancelled/finalized" — CancelOrder adapter.go:175,
	//   CancelStopLoss adapter.go:1376
	// - 40872 "margin mode already set" — SetMarginMode adapter.go:401
	// - 43025 same semantic family as 43011 — CancelOrder adapter.go:175,
	//   CancelStopLoss adapter.go:1376
	// Pass these through untouched so existing idempotent handling keeps working.
	if bitgetPassThroughCodes[envelope.Code] {
		return string(data), nil
	}

	// Non-2xx HTTP is always error. Synthesize APIError from status if body has
	// no usable code (e.g. HTML response, empty body).
	// IMPORTANT: assign to the local `err` variable captured by the metrics defer
	// above so bitget API errors are recorded for observability.
	if resp.StatusCode >= 400 {
		if envelope.Code != "" {
			err = &APIError{Code: envelope.Code, Msg: envelope.Msg}
			return "", err
		}
		err = &APIError{Code: strconv.Itoa(resp.StatusCode), Msg: fmt.Sprintf("bitget HTTP %d: %s", resp.StatusCode, string(data))}
		return "", err
	}

	// HTTP 2xx but logical failure (code != "00000").
	if envelope.Code != "" && envelope.Code != "00000" {
		err = &APIError{Code: envelope.Code, Msg: envelope.Msg}
		return "", err
	}

	return string(data), nil
}

// isRetryable checks whether an error or API response code is transient
// and worth retrying (network errors, rate limits, server-side throttling).
func isRetryable(err error, rawResp string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) && retryableCodes[apiErr.Code] {
		return true
	}
	// 5xx HTTP status synthesized into APIError via doRequest (#1): retry.
	if apiErr != nil {
		if n, convErr := strconv.Atoi(apiErr.Code); convErr == nil && n >= 500 && n <= 599 {
			return true
		}
	}
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection reset") {
			return true
		}
	}
	// Existing rawResp inspection retained for backward compat if rawResp
	// ever returned (e.g. passthrough codes above).
	if rawResp != "" {
		var base struct {
			Code string `json:"code"`
		}
		if json.Unmarshal([]byte(rawResp), &base) == nil {
			if retryableCodes[base.Code] {
				return true
			}
		}
	}
	return false
}

// retryDo performs an HTTP request with exponential backoff retry on transient errors.
// Backoff schedule: 1s, 2s, 4s (doubling each attempt).
func (c *Client) retryDo(method, path string, queryParams, bodyParams map[string]string, maxRetries int) (string, error) {
	var lastErr error
	var lastRaw string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var raw string
		var err error

		if method == "GET" {
			raw, err = c.doRequest("GET", path, queryParams, nil)
		} else {
			raw, err = c.doRequest("POST", path, nil, bodyParams)
		}

		if err == nil && !isRetryable(nil, raw) {
			return raw, nil
		}

		shouldRetry := (err != nil && isRetryable(err, "")) || (err == nil && isRetryable(nil, raw))
		if attempt < maxRetries && shouldRetry {
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s
			time.Sleep(backoff)
			lastErr = err
			lastRaw = raw
			continue
		}

		if err != nil {
			return "", err
		}
		return raw, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("%s %s: exhausted %d retries, last error: %w", method, path, maxRetries, lastErr)
	}
	return lastRaw, nil
}
