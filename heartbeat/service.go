package heartbeat

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

// Service sends heartbeat requests.
type Service struct {
	baseURL string
	client  *http.Client
	headers map[string]string
	log     logger.Logger
}

// NewService creates a heartbeat service.
func NewService(cfg *config.Config, httpClient *http.Client, log logger.Logger) *Service {
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

// Send performs a heartbeat call.
func (s *Service) Send(ctx context.Context, req *models.HeartbeatRequest) (*models.HeartbeatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: marshal request: %w", err)
	}
	url := s.baseURL + "/api/v1/heartbeat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("heartbeat: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: http error: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		var apiErr models.APIError
		if err := json.Unmarshal(payload, &apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("heartbeat: %s (%s)", apiErr.Message, apiErr.Code)
		}
		return nil, fmt.Errorf("heartbeat: status %d", resp.StatusCode)
	}
	var envelope struct {
		Code    string                    `json:"code"`
		Message string                    `json:"message"`
		Data    *models.HeartbeatResponse `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("heartbeat: decode response: %w", err)
	}
	if envelope.Code != "" && envelope.Code != "000000" {
		return nil, fmt.Errorf("heartbeat: %s (%s)", envelope.Message, envelope.Code)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("heartbeat: response missing data")
	}
	return envelope.Data, nil
}
