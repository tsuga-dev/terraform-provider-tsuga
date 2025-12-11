package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TsugaClient struct {
	BaseURL string
	Token   string
	Version string
	Commit  string
	Date    string
	client  *http.Client
}

func (c *TsugaClient) httpClient() *http.Client {
	if c.client == nil {
		c.client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return c.client
}

func (c *TsugaClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-tsuga-source", "terraform-provider")
	req.Header.Set("x-tsuga-source-version", c.Version)
	req.Header.Set("x-tsuga-source-commit", c.Commit)
	req.Header.Set("x-tsuga-source-date", c.Date)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

func (c *TsugaClient) checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API request failed with status %d (unable to read error body)", resp.StatusCode)
	}

	var errorResp struct {
		RequestID string `json:"requestId"`
		Error     struct {
			Code       string `json:"code"`
			Message    string `json:"message"`
			StatusCode int    `json:"statusCode"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errorResp); err != nil {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("Tsuga API error [%s]: %s (request ID: %s)", errorResp.Error.Code, errorResp.Error.Message, errorResp.RequestID)
}
