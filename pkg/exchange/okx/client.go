package okx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	baseURL = "https://www.okx.com"
)

// retryable OKX response codes
var retryableCodes = map[string]bool{
	"50011": true, // Rate limit
	"50013": true, // System busy
}

// APIError represents an OKX API error response.
type APIError struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("okx API error code=%s msg=%s", e.Code, e.Msg)
}

// Response is the standard OKX v5 API response wrapper.
type Response struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Client is a REST client for the OKX v5 API.
// It handles HMAC-SHA256 + Base64 signing with passphrase support.
type Client struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OKX REST API client.
func NewClient(apiKey, secretKey, passphrase string) *Client {
	return &Client{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewClientWithBase creates a client with a custom base URL (for testing).
func NewClientWithBase(base string) *Client {
	return &Client{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// sign generates the HMAC-SHA256 + Base64 signature for OKX API authentication.
// The message format is: timestamp + METHOD + requestPath + body
func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// SignWS generates the HMAC-SHA256 + Base64 signature for OKX WebSocket login.
// The message format is: timestamp + "GET" + "/users/self/verify"
func (c *Client) SignWS(timestamp string) string {
	message := timestamp + "GET" + "/users/self/verify"
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
func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	return c.retryDo("GET", path, params, nil, 3)
}

// Post performs an authenticated POST request with retry logic.
func (c *Client) Post(path string, body interface{}) ([]byte, error) {
	return c.retryDo("POST", path, nil, body, 3)
}

// doRequest performs a single authenticated HTTP request.
func (c *Client) doRequest(method, path string, queryParams map[string]string, body interface{}) ([]byte, error) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

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
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			bodyStr = string(b)
			reqBody = bytes.NewReader(b)
		}
	}

	signature := c.sign(timestamp, method, requestPath, bodyStr)

	fullURL := c.baseURL + requestPath

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", c.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the standard response envelope
	var okxResp Response
	if err := json.Unmarshal(data, &okxResp); err != nil {
		return nil, fmt.Errorf("okx response unmarshal: %w (raw: %s)", err, string(data))
	}

	if okxResp.Code != "0" {
		// Try to extract the inner error detail from data array.
		// OKX envelope code=1 ("All operations failed") wraps per-order errors
		// in data[].sCode/sMsg which contain the actual failure reason.
		if len(okxResp.Data) > 0 {
			var inner []struct {
				SCode string `json:"sCode"`
				SMsg  string `json:"sMsg"`
			}
			if json.Unmarshal(okxResp.Data, &inner) == nil && len(inner) > 0 && inner[0].SCode != "0" {
				return nil, &APIError{Code: inner[0].SCode, Msg: inner[0].SMsg}
			}
		}
		return nil, &APIError{Code: okxResp.Code, Msg: okxResp.Msg}
	}

	return okxResp.Data, nil
}

// isRetryable checks whether an error or API response is transient.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*APIError); ok {
		return retryableCodes[apiErr.Code]
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "connection reset")
}

// retryDo performs an HTTP request with exponential backoff retry on transient errors.
func (c *Client) retryDo(method, path string, queryParams map[string]string, body interface{}, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		data, err := c.doRequest(method, path, queryParams, body)
		if err == nil {
			return data, nil
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
