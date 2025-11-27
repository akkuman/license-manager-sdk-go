package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config aggregates all the SDK options.
type Config struct {
	Server                string
	BasePath              string
	Product               string
	Version               string
	AuthorizationCode     string
	AuthorizationCodePath string
	LicenseFilePath       string

	PublicKeyPEM  []byte
	PublicKeyPath string

	Offline bool

	HeartbeatInterval time.Duration
	HTTPTimeout       time.Duration

	LogLevel string

	StoragePath   string
	StorageSecret []byte

	HardwareFields []string

	DeviceInfo map[string]interface{}
	Metadata   map[string]interface{}

	HTTPHeaders map[string]string
}

// Validate performs a static sanity check on the configuration.
func (c *Config) Validate() error {
	if c.Product == "" {
		return errors.New("config: product is required")
	}
	if c.Version == "" {
		return errors.New("config: version is required")
	}
	if !c.Offline && c.Server == "" {
		return errors.New("config: server is required for online mode")
	}
	if !c.Offline && c.AuthorizationCode == "" && c.AuthorizationCodePath == "" && c.LicenseFilePath == "" {
		return errors.New("config: authorization code, authorization code path, or license file path is required")
	}
	if c.LicenseFilePath == "" {
		c.LicenseFilePath = filepath.Join("license_code", "license.lic")
	}
	if c.StoragePath == "" {
		c.StoragePath = c.LicenseFilePath
	}
	if _, err := c.ResolvePublicKey(); err != nil {
		return err
	}
	if !c.Offline {
		if _, err := c.ResolveAuthorizationCode(); err != nil {
			return err
		}
	}
	return nil
}

// ResolvePublicKey loads the configured RSA public key bytes.
func (c *Config) ResolvePublicKey() ([]byte, error) {
	if len(c.PublicKeyPEM) > 0 {
		return c.PublicKeyPEM, nil
	}
	if c.PublicKeyPath == "" {
		return nil, errors.New("config: public key pem or path must be provided")
	}
	data, err := os.ReadFile(c.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("config: read public key: %w", err)
	}
	c.PublicKeyPEM = data
	return data, nil
}

// ResolveAuthorizationCode loads the authorization code from memory or file.
func (c *Config) ResolveAuthorizationCode() (string, error) {
	if c.AuthorizationCode != "" {
		return strings.TrimSpace(c.AuthorizationCode), nil
	}
	if c.AuthorizationCodePath == "" {
		return "", errors.New("config: authorization code or path must be provided")
	}
	data, err := os.ReadFile(c.AuthorizationCodePath)
	if err != nil {
		return "", fmt.Errorf("config: read authorization code: %w", err)
	}
	code := strings.TrimSpace(string(data))
	if code == "" {
		return "", errors.New("config: authorization code file is empty")
	}
	c.AuthorizationCode = code
	return code, nil
}

// HeartbeatIntervalOrDefault returns configured interval or default 5 minutes.
func (c *Config) HeartbeatIntervalOrDefault() time.Duration {
	if c.HeartbeatInterval > 0 {
		return c.HeartbeatInterval
	}
	return 5 * time.Minute
}

// HTTPTimeoutOrDefault returns http timeout or 15s default.
func (c *Config) HTTPTimeoutOrDefault() time.Duration {
	if c.HTTPTimeout > 0 {
		return c.HTTPTimeout
	}
	return 15 * time.Second
}
