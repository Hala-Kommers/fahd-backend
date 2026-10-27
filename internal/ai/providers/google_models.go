package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"fahd-backend/internal/ai"
)

type GoogleModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// ListModels is discovery only. A successful generation is still required to
// verify that this project's key, model permissions and quota work together.
func (p *GoogleProvider) ListModels(ctx context.Context) ([]GoogleModel, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("google api key is required")
	}
	models := []GoogleModel{}
	seen := map[string]bool{}
	pageToken := ""
	for {
		query := url.Values{"pageSize": {"1000"}}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-goog-api-key", p.apiKey)
		res, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list google models: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, googleProviderError(res.StatusCode, body)
		}
		var page struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode google models: %w", err)
		}
		for _, model := range page.Models {
			id := ai.NormalizeGoogleModel(model.Name)
			if !isGoogleChatModel(id) || seen[id] {
				continue
			}
			for _, method := range model.SupportedGenerationMethods {
				if method == "generateContent" {
					models = append(models, GoogleModel{ID: id, DisplayName: model.DisplayName})
					seen[id] = true
					break
				}
			}
		}
		if page.NextPageToken == "" || page.NextPageToken == pageToken {
			break
		}
		pageToken = page.NextPageToken
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("لم يرجع Google موديلات تدعم الشات لهذا المفتاح")
	}
	sort.Slice(models, func(i, j int) bool {
		if (models[i].ID == ai.DefaultGoogleModel) != (models[j].ID == ai.DefaultGoogleModel) {
			return models[i].ID == ai.DefaultGoogleModel
		}
		return models[i].ID < models[j].ID
	})
	return models, nil
}

func isGoogleChatModel(id string) bool {
	if !strings.HasPrefix(id, "gemini-") {
		return false
	}
	for _, kind := range []string{"image", "tts", "audio", "live", "embedding", "robotics"} {
		if strings.Contains(id, kind) {
			return false
		}
	}
	return true
}
