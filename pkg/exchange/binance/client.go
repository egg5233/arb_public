package binance

import (
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

const defaultBaseURL = "https://fapi.binance.com"

// APIError represents a Binance API error response.
type APIError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance API error code=%d msg=%s", e.Code, e.Msg)
}

// Client is a low-level HTTP client for the Binance USDT-M Futures API.
type Client struct {
	baseURL   string
	apiKey    string
	secretKey string
	http      *http.Client
}

// NewClient creates a new Binance futures API client.
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		baseURL:   defaultBaseURL,
		apiKey:    apiKey,
		secretKey: secretKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// sign computes the HMAC-SHA256 signature of the given query string.
func (c *Client) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildQuery converts a params map to a sorted query string and appends
// timestamp + signature.
func (c *Client) buildQuery(params map[string]string) string {
	if params == nil {
		params = make(map[string]string)
	}
	params["timestamp"] = fmt.Sprintf("%d", time.Now().UnixMilli())

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	qs := strings.Join(parts, "&")
	sig := c.sign(qs)
	return qs + "&signature=" + sig
}

// Get performs a signed GET request.
func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := c.baseURL + path + "?" + qs

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	return c.doRequest(req)
}

// Post performs a signed POST request (params sent as form-encoded body).
func (c *Client) Post(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := c.baseURL + path

	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(qs))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doRequest(req)
}

// Put performs a signed PUT request.
func (c *Client) Put(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := c.baseURL + path

	req, err := http.NewRequest(http.MethodPut, reqURL, strings.NewReader(qs))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doRequest(req)
}

// Delete performs a signed DELETE request.
func (c *Client) Delete(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := c.baseURL + path + "?" + qs

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	return c.doRequest(req)
}

// SpotPost performs a signed POST request against the Binance spot API (api.binance.com).
func (c *Client) SpotPost(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := "https://api.binance.com" + path

	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(qs))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doRequest(req)
}

// SpotGet performs a signed GET request against the Binance spot API (api.binance.com).
func (c *Client) SpotGet(path string, params map[string]string) ([]byte, error) {
	qs := c.buildQuery(params)
	reqURL := "https://api.binance.com" + path + "?" + qs

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	return c.doRequest(req)
}

// doRequest executes the HTTP request and checks for Binance error responses.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Binance returns 200 for success; 4xx/5xx for errors.
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Code != 0 {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("binance HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Some successful responses can still contain error codes (e.g. -4046)
	if len(body) > 0 && body[0] == '{' {
		var apiErr APIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Code < 0 {
			return nil, &apiErr
		}
	}

	return body, nil
}
