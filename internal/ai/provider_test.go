package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestOllamaVisionSendsRawBase64Image(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Messages []struct {
				Images []string `json:"images"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 || len(payload.Messages[1].Images) != 1 || payload.Messages[1].Images[0] != "aW1hZ2U=" {
			t.Errorf("unexpected payload: %+v, %v", payload, err)
		}
		return response(`{"message":{"content":"{\"summary\":\"Ein Bild.\",\"tags\":[\"Foto\"]}"}}`), nil
	})}
	result, err := analyzeImageWithClient(context.Background(), Config{Provider: "ollama", Endpoint: "http://127.0.0.1:11434", Model: "vision", TimeoutSeconds: 5}, "", AnalysisRequest{Filename: "foto.jpg"}, "data:image/jpeg;base64,aW1hZ2U=", client)
	if err != nil || result.Summary != "Ein Bild." {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}

func TestOpenRouterVisionSendsImageURL(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || !strings.Contains(fmt.Sprint(payload), "data:image/png;base64,aW1hZ2U=") {
			t.Errorf("unexpected payload: %+v, %v", payload, err)
		}
		return response(`{"choices":[{"message":{"content":"{\"summary\":\"Eine Grafik.\",\"tags\":[\"Grafik\"]}"}}]}`), nil
	})}
	result, err := analyzeImageWithClient(context.Background(), Config{Provider: "openrouter", Endpoint: "https://openrouter.example/api/v1", Model: "auto", TimeoutSeconds: 5}, "token", AnalysisRequest{Filename: "grafik.png"}, "data:image/png;base64,aW1hZ2U=", client)
	if err != nil || result.Summary != "Eine Grafik." {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
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

func TestOllamaAnalysisParsesJSONResult(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/chat" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		return response(`{"message":{"content":"{\"summary\":\"Ein Projektplan.\",\"tags\":[\"Planung\",\"Projekt\",\"Projekt\"]}"}}`), nil
	})}
	result, err := analyzeWithClient(context.Background(), Config{Provider: "ollama", Endpoint: "http://127.0.0.1:11434", Model: "test", TimeoutSeconds: 5}, "", AnalysisRequest{Filename: "plan.txt", Content: "Inhalt"}, client)
	if err != nil || result.Summary != "Ein Projektplan." || len(result.Tags) != 2 || result.Provider != "ollama" {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}

func TestOpenRouterAnalysisSendsCredential(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("unexpected request %s, %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		return response("{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"summary\\\":\\\"Ein Dokument.\\\",\\\"tags\\\":[\\\"Dokument\\\"]}\\n```\"}}]}"), nil
	})}
	result, err := analyzeWithClient(context.Background(), Config{Provider: "openrouter", Endpoint: "https://openrouter.example/api/v1", Model: "test", TimeoutSeconds: 5}, "token", AnalysisRequest{Filename: "doc.md"}, client)
	if err != nil || result.Summary != "Ein Dokument." || len(result.Tags) != 1 {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}
