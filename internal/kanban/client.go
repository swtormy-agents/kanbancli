// Package kanban is a minimal REST client for a PLANKA-compatible kanban API (community-built; not affiliated with PLANKA Software GmbH).
package kanban

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const MinTokenLength = 16

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	secrets []string
}

type APIError struct {
	Status  int
	Message string
	Body    json.RawMessage
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("kanban HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("kanban HTTP %d", e.Status)
}

func (e *APIError) IsAuth() bool { return e.Status == 401 || e.Status == 403 }

func (c *Client) WithSecrets(secrets ...string) *Client {
	out := *c
	for _, s := range secrets {
		s = strings.TrimSpace(s)
		if s != "" {
			out.secrets = append(out.secrets, s)
		}
	}
	return &out
}

func (c *Client) base() string {
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}

func (c *Client) Redact(s string) string {
	s = StripURLUserinfo(s)
	if c.Token != "" && len(c.Token) >= MinTokenLength {
		s = strings.ReplaceAll(s, c.Token, "<API_TOKEN>")
	}
	for _, sec := range c.secrets {
		if len(sec) >= 8 {
			s = strings.ReplaceAll(s, sec, "<REDACTED>")
		}
	}
	return s
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader, withAuth bool) (*http.Request, error) {
	u := c.base() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, errors.New(c.Redact(err.Error()))
	}
	if withAuth && c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader, withAuth bool) (json.RawMessage, http.Header, error) {
	req, err := c.newRequest(ctx, method, path, query, body, withAuth)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request %s %s failed: %s", method, path, c.Redact(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %s", c.Redact(err.Error()))
	}
	if resp.StatusCode >= 400 {
		msg := c.Redact(parseMessage(data))
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: msg, Body: data}
	}
	if len(data) == 0 {
		return json.RawMessage("null"), resp.Header, nil
	}
	if !json.Valid(data) {
		return json.RawMessage(data), resp.Header, nil
	}
	return json.RawMessage(data), resp.Header, nil
}

func parseMessage(data []byte) string {
	var m struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(data, &m) == nil {
		if m.Message != "" {
			return m.Message
		}
		if m.Error != "" {
			return m.Error
		}
		if m.Code != "" {
			return m.Code
		}
	}
	return strings.TrimSpace(string(data))
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, http.Header, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil, true)
}

func (c *Client) JSON(ctx context.Context, method, path string, v any, withAuth bool) (json.RawMessage, http.Header, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return c.Do(ctx, method, path, nil, bytes.NewReader(b), withAuth)
}

func DryRun(method, baseURL, path string, query url.Values, body any) map[string]any {
	safeBase, _ := ValidateBaseURL(baseURL)
	if safeBase == "" {
		safeBase = StripURLUserinfo(strings.TrimSpace(baseURL))
	}
	u := strings.TrimRight(safeBase, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	out := map[string]any{
		"method": method,
		"url":    u,
		"header": map[string]string{"Authorization": "Bearer <API_TOKEN>"},
	}
	if body != nil {
		out["body"] = body
	}
	return out
}

func DryRunUnauthenticated(method, baseURL, path string, body any) map[string]any {
	safeBase, _ := ValidateBaseURL(baseURL)
	if safeBase == "" {
		safeBase = StripURLUserinfo(strings.TrimSpace(baseURL))
	}
	u := strings.TrimRight(safeBase, "/") + path
	out := map[string]any{"method": method, "url": u}
	if body != nil {
		safeBody := redactLoginBody(body)
		out["body"] = safeBody
	}
	return out
}

func redactLoginBody(body any) any {
	m, ok := body.(map[string]any)
	if !ok {
		return map[string]string{"password": "<REDACTED>"}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if strings.EqualFold(k, "password") {
			out[k] = "<REDACTED>"
		} else {
			out[k] = v
		}
	}
	return out
}

// SanitizeUsers keeps id, username, and name only.
func SanitizeUsers(raw json.RawMessage) (json.RawMessage, error) {
	items, err := unwrapItems(raw)
	if err != nil {
		return raw, nil
	}
	allowed := map[string]bool{"id": true, "username": true, "name": true}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		safe := make(map[string]any, 3)
		for key, val := range u {
			if allowed[key] {
				safe[key] = val
			}
		}
		out = append(out, safe)
	}
	return json.Marshal(out)
}

func unwrapItems(raw json.RawMessage) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var wrap struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Items) > 0 {
		return wrap.Items, nil
	}
	var single struct {
		Item map[string]any `json:"item"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Item != nil {
		return []map[string]any{single.Item}, nil
	}
	return nil, fmt.Errorf("unexpected users payload shape")
}

// ExtractAccessToken reads an access token string from a login response body.
func ExtractAccessToken(raw json.RawMessage) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	if item, ok := m["item"]; ok {
		switch t := item.(type) {
		case string:
			if t != "" {
				return t, nil
			}
		case map[string]any:
			for _, key := range []string{"accessToken", "token"} {
				if v, ok := t[key].(string); ok && v != "" {
					return v, nil
				}
			}
		}
	}
	for _, key := range []string{"accessToken", "token"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v, nil
		}
	}
	return "", errors.New("access token not found in response")
}

func extractIncludedArray(raw json.RawMessage, key string) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if inc, ok := m["included"].(map[string]any); ok {
		if arr, ok := inc[key].([]any); ok {
			return json.Marshal(arr)
		}
	}
	if arr, ok := m[key].([]any); ok {
		return json.Marshal(arr)
	}
	if items, ok := m["items"].([]any); ok && key == "lists" {
		return json.Marshal(items)
	}
	return json.Marshal([]any{})
}

// ExtractBoardLists returns list records embedded in a board GET response.
func ExtractBoardLists(board json.RawMessage) (json.RawMessage, error) {
	return extractIncludedArray(board, "lists")
}

// ExtractBoardCards returns card records embedded in a board GET response.
func ExtractBoardCards(board json.RawMessage) (json.RawMessage, error) {
	return extractIncludedArray(board, "cards")
}
