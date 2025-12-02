package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTsugaClientDoRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var received struct {
		Method  string
		Path    string
		Headers http.Header
		Body    map[string]string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		received.Method = r.Method
		received.Path = r.URL.Path
		received.Headers = r.Header.Clone()

		if err := json.NewDecoder(r.Body).Decode(&received.Body); err != nil {
			t.Fatalf("expected JSON body, got error: %v", err)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &TsugaClient{
		BaseURL: server.URL,
		Token:   "token-123",
		client:  server.Client(),
	}

	_, err := client.doRequest(ctx, http.MethodPost, "/teams", map[string]string{"name": "cedar"})
	if err != nil {
		t.Fatalf("doRequest returned error: %v", err)
	}

	if received.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", received.Method)
	}
	if received.Path != "/teams" {
		t.Errorf("expected path /teams, got %s", received.Path)
	}
	if got := received.Headers.Get("Authorization"); got != "Bearer token-123" {
		t.Errorf("expected Authorization header, got %q", got)
	}
	if got := received.Headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", got)
	}
	if received.Body["name"] != "cedar" {
		t.Errorf("expected JSON body to include name=cedar, got %v", received.Body)
	}
}

func TestTsugaClientCheckResponse(t *testing.T) {
	t.Parallel()

	okResp := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	if err := (&TsugaClient{}).checkResponse(okResp); err != nil {
		t.Fatalf("expected success response to pass, got %v", err)
	}

	errResp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("fail")),
	}
	if err := (&TsugaClient{}).checkResponse(errResp); err == nil {
		t.Fatalf("expected error response to return error")
	}
}
