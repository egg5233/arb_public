package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
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

const defaultBaseURL = "https://api.gateio.ws/api/v4"

// APIError represents a Gate.io API error response.
type APIError struct {
	Label   string `json:"label"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gateio API error label=%s message=%s", e.Label, e.Message)
}

// Client is a low-level HTTP client for the Gate.io API v4.
type Client struct {
	baseURL   string
	apiKey    string
	secretKey string
	http      *http.Client
}

// NewClient creates a new Gate.io API client.
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		baseURL:   defaultBaseURL,
		apiKey:    apiKey,
		secretKey: secretKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NewClientWithBase creates a client with a custom base URL (for testing).
func NewClientWithBase(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// sha512Hash computes SHA-512 hex digest of the given data.
func sha512Hash(data string) string {
	h := sha512.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// sign computes the HMAC-SHA512 signature for Gate.io API authentication.
// Signature = HMAC-SHA512(method + "\n" + path + "\n" + queryString + "\n" + SHA512(body) + "\n" + timestamp, secretKey)
func (c *Client) sign(method, path, queryString, body, timestamp string) string {
	bodyHash := sha512Hash(body)
	payload := method + "\n" + path + "\n" + queryString + "\n" + bodyHash + "\n" + timestamp
	mac := hmac.New(sha512.New, []byte(c.secretKey))
	mac.Write([]byte(payload))
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
func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	return c.retryDo("GET", path, params, "", 3)
}

// Post performs an authenticated POST request with JSON body and retry logic.
func (c *Client) Post(path string, body string) ([]byte, error) {
	return c.retryDo("POST", path, nil, body, 3)
}

// Delete performs an authenticated DELETE request with retry logic.
func (c *Client) Delete(path string, params map[string]string) ([]byte, error) {
	return c.retryDo("DELETE", path, params, "", 3)
}

// doRequest performs a single authenticated HTTP request.
func (c *Client) doRequest(method, path string, queryParams map[string]string, body string) ([]byte, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Separate any inline query string already in the path
	cleanPath := path
	inlineQS := ""
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		cleanPath = path[:idx]
		inlineQS = path[idx+1:]
	}

	// Build query string from params map
	paramQS := buildQueryString(queryParams)

	// Merge inline and param query strings
	queryString := inlineQS
	if paramQS != "" {
		if queryString != "" {
			queryString += "&" + paramQS
		} else {
			queryString = paramQS
		}
	}

	// The path for signing is the full API path (without query string).
	// Gate.io signs: /api/v4/futures/usdt/orders
	apiPath := "/api/v4" + cleanPath
	signature := c.sign(method, apiPath, queryString, body, timestamp)

	fullURL := c.baseURL + cleanPath
	if queryString != "" {
		fullURL += "?" + queryString
	}

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("KEY", c.apiKey)
	req.Header.Set("SIGN", signature)
	req.Header.Set("Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Gate.io returns 2xx for success; 4xx/5xx for errors.
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Label != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("gateio HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// isRetryable checks whether an error or API response is transient and worth retrying.
func isRetryable(err error, data []byte) bool {
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection reset") {
			return true
		}
		if apiErr, ok := err.(*APIError); ok {
			switch apiErr.Label {
			case "SERVER_ERROR", "TOO_MANY_REQUESTS", "INTERNAL_ERROR":
				return true
			}
		}
		return false
	}

	if len(data) > 0 {
		var apiErr APIError
		if json.Unmarshal(data, &apiErr) == nil {
			switch apiErr.Label {
			case "SERVER_ERROR", "TOO_MANY_REQUESTS", "INTERNAL_ERROR":
				return true
			}
		}
	}

	return false
}

// retryDo performs an HTTP request with exponential backoff retry on transient errors.
// Backoff schedule: 1s, 2s, 4s (doubling each attempt).
func (c *Client) retryDo(method, path string, queryParams map[string]string, body string, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		data, err := c.doRequest(method, path, queryParams, body)

		if err == nil && !isRetryable(nil, data) {
			return data, nil
		}

		shouldRetry := (err != nil && isRetryable(err, nil)) || (err == nil && isRetryable(nil, data))
		if attempt < maxRetries && shouldRetry {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
			lastErr = err
			continue
		}

		if err != nil {
			return nil, err
		}
		return data, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%s %s: exhausted %d retries, last error: %w", method, path, maxRetries, lastErr)
	}
	return nil, fmt.Errorf("%s %s: exhausted %d retries", method, path, maxRetries)
}
