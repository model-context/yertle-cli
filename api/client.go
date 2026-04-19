package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		return &APIError{StatusCode: resp.StatusCode, Detail: parseErrorDetail(respBody)}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}

// parseErrorDetail extracts a short, human-readable message from a non-2xx
// response body. Handles two common shapes:
//   - {"detail": "some string"}                              (app-level errors)
//   - {"detail": [{"loc": [...], "msg": "..."}, ...]}         (FastAPI validation)
// Falls back to a truncated raw body so we don't dump kilobytes of HTML.
func parseErrorDetail(body []byte) string {
	// Shape 1: string detail
	var asString struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(body, &asString) == nil && asString.Detail != "" {
		return asString.Detail
	}

	// Shape 2: list of validation errors
	var asList struct {
		Detail []struct {
			Loc []any  `json:"loc"`
			Msg string `json:"msg"`
		} `json:"detail"`
	}
	if json.Unmarshal(body, &asList) == nil && len(asList.Detail) > 0 {
		parts := make([]string, 0, len(asList.Detail))
		for _, d := range asList.Detail {
			field := ""
			if len(d.Loc) > 0 {
				field = fmt.Sprintf("%v", d.Loc[len(d.Loc)-1])
			}
			if field != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", field, d.Msg))
			} else {
				parts = append(parts, d.Msg)
			}
		}
		return strings.Join(parts, "; ")
	}

	// Fallback: raw body (truncated)
	s := string(body)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
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
		return fmt.Errorf("token expired and refresh failed — run: yertle login")
	}

	// Update client token and notify caller to persist
	c.Token = refreshResp.AccessToken
	c.OnRefresh(refreshResp.AccessToken, refreshResp.ExpiresIn)

	// Retry the original request
	return c.do(method, path, body, result)
}
