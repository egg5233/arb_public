package bybit

import (
	"arb/pkg/exchange"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.bybit.com"
	defaultRecvWindow = "5000"
)

// retryable Bybit API error codes.
var retryableCodes = map[int]bool{
	10006: true, // rate limit
	10016: true, // server error
	10018: true, // server busy
}

// APIResponse is the top-level Bybit v5 API response wrapper.
type APIResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

// APIError represents a Bybit API error.
type APIError struct {
	Code int    `json:"retCode"`
	Msg  string `json:"retMsg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bybit API error code=%d msg=%s", e.Code, e.Msg)
}

// Client is a low-level HTTP client for the Bybit v5 REST API.
type Client struct {
	baseURL         string
	apiKey          string
	secretKey       string
	recvWindow      string
	httpClient      *http.Client
	metricsCallback exchange.MetricsCallback
}

// NewClient creates a new Bybit REST API client.
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		secretKey:  secretKey,
		recvWindow: defaultRecvWindow,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetMetricsCallback(fn exchange.MetricsCallback) {
	c.metricsCallback = fn
}

// sign computes the HMAC-SHA256 signature (hex encoded).
// Bybit v5 signature payload: timestamp + apiKey + recvWindow + (queryString or jsonBody)
func (c *Client) sign(timestamp, payload string) string {
	message := timestamp + c.apiKey + c.recvWindow + payload
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
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
func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error) {
	return c.retryDo("GET", path, params, nil, 3)
}

// Post performs an authenticated POST request with retry logic.
func (c *Client) Post(path string, params map[string]string) (json.RawMessage, error) {
	return c.retryDo("POST", path, nil, params, 3)
}

// doRequest performs a single authenticated HTTP request.
func (c *Client) doRequest(method, path string, queryParams, bodyParams map[string]string) (json.RawMessage, error) {
	start := time.Now()
	var err error
	defer func() {
		if c.metricsCallback != nil {
			c.metricsCallback(path, time.Since(start), err)
		}
	}()

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	var fullURL string
	var reqBody io.Reader
	var signPayload string

	if method == "GET" {
		qs := buildQueryString(queryParams)
		signPayload = qs
		if qs != "" {
			fullURL = c.baseURL + path + "?" + qs
		} else {
			fullURL = c.baseURL + path
		}
	} else {
		fullURL = c.baseURL + path
		if len(bodyParams) > 0 {
			b, err := json.Marshal(bodyParams)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			signPayload = string(b)
			reqBody = bytes.NewReader(b)
		}
	}

	signature := c.sign(timestamp, signPayload)

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", c.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", c.recvWindow)

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
		err = fmt.Errorf("bybit: unmarshal response: %w (body=%s)", err, string(data))
		return nil, err
	}

	if apiResp.RetCode != 0 {
		err = &APIError{Code: apiResp.RetCode, Msg: apiResp.RetMsg}
		return nil, err
	}

	return apiResp.Result, nil
}

// isRetryable checks whether an error is transient and worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for Bybit API error codes.
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
func (c *Client) retryDo(method, path string, queryParams, bodyParams map[string]string, maxRetries int) (json.RawMessage, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var result json.RawMessage
		var err error

		if method == "GET" {
			result, err = c.doRequest("GET", path, queryParams, nil)
		} else {
			result, err = c.doRequest("POST", path, nil, bodyParams)
		}

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
