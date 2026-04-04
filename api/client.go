package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OnTokenRefresh is called when the client auto-refreshes an expired token.
// The caller should persist the new tokens.
type OnTokenRefresh func(accessToken string, expiresIn int)

type Client struct {
	BaseURL        string
	Token          string
	RefreshTokenV  string         // stored refresh token
	OnRefresh      OnTokenRefresh // callback to persist new tokens
	HTTPClient     *http.Client
	triedRefresh   bool           // prevents infinite refresh loops
}

type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Detail)
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithRefresh configures the client for automatic token refresh on 401.
func (c *Client) WithRefresh(refreshToken string, onRefresh OnTokenRefresh) *Client {
	c.RefreshTokenV = refreshToken
	c.OnRefresh = onRefresh
	return c
}

func (c *Client) do(method, path string, body any, result any) error {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := string(respBody)
		// Try to extract a detail field from JSON error responses
		var errResp struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Detail != "" {
			detail = errResp.Detail
		}
		return &APIError{StatusCode: resp.StatusCode, Detail: detail}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}

func (c *Client) Get(path string, result any) error {
	return c.doWithRefresh(http.MethodGet, path, nil, result)
}

func (c *Client) Post(path string, body, result any) error {
	return c.doWithRefresh(http.MethodPost, path, body, result)
}

// doWithRefresh wraps do() with automatic token refresh on 401.
func (c *Client) doWithRefresh(method, path string, body any, result any) error {
	err := c.do(method, path, body, result)
	if err == nil {
		return nil
	}

	// Check if it's a 401 and we can refresh
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 401 {
		return err
	}
	if c.triedRefresh || c.RefreshTokenV == "" || c.OnRefresh == nil {
		return err
	}

	// Try refreshing the token
	c.triedRefresh = true
	refreshResp, refreshErr := c.RefreshToken(c.RefreshTokenV)
	if refreshErr != nil {
		return fmt.Errorf("token expired and refresh failed — run: yertle auth login")
	}

	// Update client token and notify caller to persist
	c.Token = refreshResp.AccessToken
	c.OnRefresh(refreshResp.AccessToken, refreshResp.ExpiresIn)

	// Retry the original request
	return c.do(method, path, body, result)
}
