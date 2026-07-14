package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const responseLimit = 1 << 20

type Config struct {
	Provider       string
	Endpoint       string
	Model          string
	TimeoutSeconds int
}

type ConnectionResult struct {
	Provider        string   `json:"provider"`
	Endpoint        string   `json:"endpoint"`
	Model           string   `json:"model"`
	AvailableModels []string `json:"availableModels"`
	ModelAvailable  bool     `json:"modelAvailable"`
	Message         string   `json:"message"`
}

func TestConnection(ctx context.Context, config Config, credential string) (ConnectionResult, error) {
	client := &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}
	return testConnection(ctx, config, credential, client)
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func testConnection(ctx context.Context, config Config, credential string, client httpDoer) (ConnectionResult, error) {
	base, err := validate(config)
	if err != nil {
		return ConnectionResult{}, err
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 30
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.TimeoutSeconds)*time.Second)
	defer cancel()

	path := "/api/tags"
	if config.Provider == "openrouter" {
		if strings.TrimSpace(credential) == "" {
			return ConnectionResult{}, fmt.Errorf("für OpenRouter ist noch kein API-Schlüssel gespeichert")
		}
		path = "/models"
	}
	requestURL := strings.TrimRight(base.String(), "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return ConnectionResult{}, err
	}
	if config.Provider == "openrouter" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(credential))
	}
	response, err := client.Do(req)
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("KI-Anbieter nicht erreichbar: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return ConnectionResult{}, fmt.Errorf("Antwort lesen: %w", err)
	}
	if len(body) > responseLimit {
		return ConnectionResult{}, fmt.Errorf("Antwort des KI-Anbieters ist unerwartet groß")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ConnectionResult{}, fmt.Errorf("KI-Anbieter antwortet mit HTTP %d: %s", response.StatusCode, concise(body))
	}

	models, err := parseModels(config.Provider, body)
	if err != nil {
		return ConnectionResult{}, err
	}
	result := ConnectionResult{Provider: config.Provider, Endpoint: base.String(), Model: config.Model, AvailableModels: models}
	for _, model := range models {
		if model == config.Model {
			result.ModelAvailable = true
			break
		}
	}
	if result.ModelAvailable {
		result.Message = fmt.Sprintf("Verbindung erfolgreich; Modell %s ist verfügbar", config.Model)
	} else {
		result.Message = fmt.Sprintf("Verbindung erfolgreich; Modell %s wurde nicht in der Modellliste gefunden", config.Model)
	}
	return result, nil
}

func validate(config Config) (*url.URL, error) {
	if config.Provider != "ollama" && config.Provider != "openrouter" {
		return nil, fmt.Errorf("nicht unterstützter KI-Anbieter")
	}
	base, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("KI-Endpunkt ist keine gültige HTTP-Adresse")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("KI-Endpunkt muss HTTP oder HTTPS verwenden")
	}
	if base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("KI-Endpunkt darf keine Abfrage oder Sprungmarke enthalten")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return base, nil
}

func parseModels(provider string, body []byte) ([]string, error) {
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("Modellliste des KI-Anbieters ist ungültig: %w", err)
	}
	models := make([]string, 0)
	if provider == "ollama" {
		for _, model := range payload.Models {
			if model.Name != "" {
				models = append(models, model.Name)
			}
		}
	} else {
		for _, model := range payload.Data {
			if model.ID != "" {
				models = append(models, model.ID)
			}
		}
	}
	return models, nil
}

func concise(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if len(text) > 240 {
		return text[:240] + "…"
	}
	if text == "" {
		return "keine Fehlermeldung"
	}
	return text
}
