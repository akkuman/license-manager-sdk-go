package models

import "time"

// LicensePayload describes the decoded content of a license file.
type LicensePayload struct {
	Product             string                 `json:"product,omitempty"`
	Version             string                 `json:"version,omitempty"`
	LicenseKey          string                 `json:"license_key"`
	AuthorizationCode   string                 `json:"authorization_code"`
	AuthorizationCodeID string                 `json:"authorization_code_id,omitempty"`
	HardwareFingerprint string                 `json:"hardware_fingerprint"`
	Status              string                 `json:"status"`
	DeploymentType      string                 `json:"deployment_type,omitempty"`
	MaxActivations      int                    `json:"max_activations,omitempty"`
	CustomParameters    map[string]interface{} `json:"custom_parameters,omitempty"`
	FeatureConfig       map[string]interface{} `json:"feature_config,omitempty"`
	UsageLimits         map[string]interface{} `json:"usage_limits,omitempty"`
	StartDate           time.Time              `json:"start_date"`
	EndDate             time.Time              `json:"end_date"`
	ActivatedAt         *time.Time             `json:"activated_at,omitempty"`
	GeneratedAt         *time.Time             `json:"generated_at,omitempty"`
	ConfigUpdatedAt     *time.Time             `json:"config_updated_at,omitempty"`
	ExpiresAt           time.Time              `json:"-"`
	Extras              map[string]interface{} `json:"extras,omitempty"`
}

// LicenseEnvelope is the serialized document persisted locally.
type LicenseEnvelope struct {
	Algorithm string `json:"algorithm"`
	Data      string `json:"data"`
	Signature string `json:"signature"` // base64 encoded signature.
}

// ActivateRequest mirrors the server request contract.
type ActivateRequest struct {
	AuthorizationCode   string                 `json:"authorization_code"`
	Product             string                 `json:"product"`
	Version             string                 `json:"version"`
	HardwareFingerprint string                 `json:"hardware_fingerprint"`
	SoftwareVersion     string                 `json:"software_version,omitempty"`
	DeviceInfo          map[string]interface{} `json:"device_info,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// ActivateResponse contains activation result data.
type ActivateResponse struct {
	LicenseKey        string          `json:"license_key"`
	LicenseFile       string          `json:"license_file"` // base64 encoded LicenseEnvelope
	HeartbeatInterval int             `json:"heartbeat_interval"`
	Payload           *LicensePayload `json:"payload,omitempty"`
}

// HeartbeatRequest describes the heartbeat payload.
type HeartbeatRequest struct {
	LicenseKey          string                 `json:"license_key"`
	HardwareFingerprint string                 `json:"hardware_fingerprint"`
	SoftwareVersion     string                 `json:"software_version,omitempty"`
	UsageData           map[string]interface{} `json:"usage_data,omitempty"`
	ConfigUpdatedAt     *time.Time             `json:"config_updated_at,omitempty"`
}

// HeartbeatResponse returns status and optionally a new license.
type HeartbeatResponse struct {
	Status            string          `json:"status"`
	LicenseFile       string          `json:"license_file,omitempty"`
	HeartbeatInterval int             `json:"heartbeat_interval,omitempty"`
	ConfigUpdated     bool            `json:"config_updated,omitempty"`
	Payload           *LicensePayload `json:"payload,omitempty"`
}

// APIError is a simplified version of the server error payload.
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}
