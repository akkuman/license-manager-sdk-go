package activation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cedar-v/license-manage-sdk-go/config"
	"github.com/cedar-v/license-manage-sdk-go/logger"
	"github.com/cedar-v/license-manage-sdk-go/models"
)

// Service performs activation requests against the License Manager server.
type Service struct {
	baseURL string
	client  *http.Client
	headers map[string]string
	log     logger.Logger
}

// New creates the activation service.
func New(cfg *config.Config, httpClient *http.Client, log logger.Logger) *Service {
	base := strings.TrimRight(cfg.Server, "/")
	if cfg.BasePath != "" {
		base = base + "/" + strings.Trim(cfg.BasePath, "/")
	}
	return &Service{
		baseURL: base,
		client:  httpClient,
		headers: cfg.HTTPHeaders,
		log:     log,
	}
}

// Activate exchanges an authorization code for a license file.
func (s *Service) Activate(ctx context.Context, req *models.ActivateRequest) (*models.ActivateResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("activation: marshal request: %w", err)
	}
	url := s.baseURL + "/api/v1/activate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("activation: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("activation: http error: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("activation: read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		var apiErr models.APIError
		if err := json.Unmarshal(payload, &apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("activation: %s (%s)", apiErr.Message, apiErr.Code)
		}
		return nil, fmt.Errorf("activation: status %d", resp.StatusCode)
	}
	var envelope struct {
		Code    string                   `json:"code"`
		Message string                   `json:"message"`
		Data    *models.ActivateResponse `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("activation: decode response: %w", err)
	}
	if envelope.Code != "" && envelope.Code != "000000" {
		return nil, fmt.Errorf("activation: %s (%s)", envelope.Message, envelope.Code)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("activation: response missing data")
	}
	return envelope.Data, nil
}
