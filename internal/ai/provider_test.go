package ai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestOllamaConnectionListsConfiguredModel(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/tags" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		return response(`{"models":[{"name":"qwen2.5:1.5b"}]}`), nil
	})}
	result, err := testConnection(context.Background(), Config{Provider: "ollama", Endpoint: "http://127.0.0.1:11434", Model: "qwen2.5:1.5b", TimeoutSeconds: 5}, "", client)
	if err != nil || !result.ModelAvailable || len(result.AvailableModels) != 1 {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}

func TestOpenRouterRequiresCredential(t *testing.T) {
	_, err := TestConnection(context.Background(), Config{Provider: "openrouter", Endpoint: "https://openrouter.ai/api/v1", Model: "test", TimeoutSeconds: 5}, "")
	if err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestOpenRouterUsesBearerCredential(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/models" || request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("unexpected request %s, %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		return response(`{"data":[{"id":"qwen/test"}]}`), nil
	})}
	result, err := testConnection(context.Background(), Config{Provider: "openrouter", Endpoint: "https://openrouter.example/api/v1", Model: "qwen/test", TimeoutSeconds: 5}, "token", client)
	if err != nil || !result.ModelAvailable {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}

func TestProviderRejectsNonHTTPURL(t *testing.T) {
	_, err := TestConnection(context.Background(), Config{Provider: "ollama", Endpoint: "file:///tmp/provider", Model: "test", TimeoutSeconds: 5}, "")
	if err == nil {
		t.Fatal("expected invalid endpoint error")
	}
}
