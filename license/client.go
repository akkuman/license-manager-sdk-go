package license

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cedar-v/license-manage-sdk-go/activation"
	"github.com/cedar-v/license-manage-sdk-go/config"
	"github.com/cedar-v/license-manage-sdk-go/hardware"
	"github.com/cedar-v/license-manage-sdk-go/heartbeat"
	"github.com/cedar-v/license-manage-sdk-go/internal/httpclient"
	"github.com/cedar-v/license-manage-sdk-go/logger"
	"github.com/cedar-v/license-manage-sdk-go/models"
	"github.com/cedar-v/license-manage-sdk-go/storage"
	"github.com/cedar-v/license-manage-sdk-go/validator"
)

// Callbacks expose SDK lifecycle hooks.
type Callbacks struct {
	OnLicenseUpdated     func(*models.LicensePayload)
	OnHeartbeatError     func(error)
	OnActivationRequired func(string)
}

// Option configures the client.
type Option func(*clientOptions)

type clientOptions struct {
	logger           logger.Logger
	hardwareProvider hardware.Provider
	store            storage.Store
	httpClient       *http.Client
	callbacks        Callbacks
}

// WithLogger overrides the default logger.
func WithLogger(l logger.Logger) Option {
	return func(o *clientOptions) {
		o.logger = l
	}
}

// WithHardwareProvider injects a custom hardware provider.
func WithHardwareProvider(p hardware.Provider) Option {
	return func(o *clientOptions) {
		o.hardwareProvider = p
	}
}

// WithStore overrides the storage backend.
func WithStore(s storage.Store) Option {
	return func(o *clientOptions) {
		o.store = s
	}
}

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(o *clientOptions) {
		o.httpClient = client
	}
}

// WithCallbacks registers lifecycle hooks.
func WithCallbacks(cb Callbacks) Option {
	return func(o *clientOptions) {
		o.callbacks = cb
	}
}

// Client orchestrates activation, validation and heartbeat.
type Client struct {
	cfg        *config.Config
	log        logger.Logger
	store      storage.Store
	validator  *validator.Validator
	activate   *activation.Service
	heartbeatS *heartbeat.Service
	heartbeatM *heartbeat.Manager
	httpClient *http.Client
	hardware   hardware.Provider
	callbacks  Callbacks

	fingerprint string
	fpDetails   map[string]string
	licenseKey  string

	mu      sync.RWMutex
	current *models.LicensePayload
}

// NewClient constructs the SDK client and performs bootstrap validation.
func NewClient(cfg *config.Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("license: config is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	pubKey, err := cfg.ResolvePublicKey()
	if err != nil {
		return nil, err
	}

	opt := &clientOptions{}
	for _, o := range opts {
		o(opt)
	}

	logLevel := logger.ParseLevel(cfg.LogLevel)
	log := opt.logger
	if log == nil {
		log = logger.NewStdLogger(logLevel)
	}
	httpClient := opt.httpClient
	if httpClient == nil {
		httpClient = httpclient.New(cfg.HTTPTimeoutOrDefault())
	}
	store := opt.store
	if store == nil {
		store = storage.NewFileStore(cfg.StoragePath, cfg.StorageSecret)
	}
	hw := opt.hardwareProvider
	if hw == nil {
		hw = hardware.NewDefaultProvider(cfg.HardwareFields)
	}

	fp, details, err := hw.Fingerprint(context.Background())
	if err != nil {
		return nil, fmt.Errorf("license: collect fingerprint: %w", err)
	}

	val, err := validator.New(pubKey)
	if err != nil {
		return nil, err
	}

	client := &Client{
		cfg:         cfg,
		log:         log,
		store:       store,
		validator:   val,
		activate:    activation.New(cfg, httpClient, log),
		heartbeatS:  heartbeat.NewService(cfg, httpClient, log),
		httpClient:  httpClient,
		hardware:    hw,
		callbacks:   opt.callbacks,
		fingerprint: fp,
		fpDetails:   details,
	}

	if err := client.bootstrap(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}

// Validate re-validates the current license.
func (c *Client) Validate(ctx context.Context) error {
	c.mu.RLock()
	license := c.current
	c.mu.RUnlock()
	if license == nil {
		return errors.New("license: no loaded license")
	}
	if time.Now().After(license.ExpiresAt) {
		return errors.New("license: expired")
	}
	return nil
}

// CurrentLicense returns a copy of the payload.
func (c *Client) CurrentLicense() *models.LicensePayload {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.current == nil {
		return nil
	}
	copyPayload := *c.current
	return &copyPayload
}

// Close shuts down background goroutines.
func (c *Client) Close() error {
	if c.heartbeatM != nil {
		c.heartbeatM.Stop()
	}
	return nil
}

// PauseHeartbeat temporarily halts heartbeat loop.
func (c *Client) PauseHeartbeat() {
	if c.heartbeatM != nil {
		c.heartbeatM.Pause()
	}
}

// ResumeHeartbeat resumes heartbeat loop.
func (c *Client) ResumeHeartbeat() {
	if c.heartbeatM != nil {
		c.heartbeatM.Resume()
	}
}

func (c *Client) bootstrap(ctx context.Context) error {
	if err := c.loadExisting(ctx); err != nil {
		return err
	}
	if c.current != nil {
		c.log.Infof("license: loaded cached license expiring %s", c.current.ExpiresAt.Format(time.RFC3339))
		if !c.cfg.Offline {
			c.startHeartbeat()
		}
		return nil
	}
	if c.cfg.Offline {
		return errors.New("license: offline mode requires preloaded license")
	}
	if err := c.performActivation(ctx); err != nil {
		return err
	}
	c.startHeartbeat()
	return nil
}

func (c *Client) loadExisting(ctx context.Context) error {
	data, err := c.store.Load(ctx)
	if err != nil || len(data) == 0 {
		return err
	}
	payload, err := c.validateAndStore(data)
	if err != nil {
		c.log.Warnf("license: cached license invalid: %v", err)
		return nil
	}
	c.mu.Lock()
	c.current = payload
	c.licenseKey = payload.LicenseKey
	c.mu.Unlock()
	return nil
}

func (c *Client) performActivation(ctx context.Context) error {
	c.log.Infof("license: activating with server %s", c.cfg.Server)
	deviceInfo := map[string]interface{}{}
	for k, v := range c.cfg.DeviceInfo {
		deviceInfo[k] = v
	}
	if len(c.fpDetails) > 0 {
		deviceInfo["hardware"] = c.fpDetails
	}
	req := &models.ActivateRequest{
		AuthorizationCode:   c.cfg.AuthorizationCode,
		Product:             c.cfg.Product,
		Version:             c.cfg.Version,
		HardwareFingerprint: c.fingerprint,
		DeviceInfo:          deviceInfo,
		Metadata:            c.cfg.Metadata,
	}
	resp, err := c.activate.Activate(ctx, req)
	if err != nil {
		if c.callbacks.OnActivationRequired != nil {
			c.callbacks.OnActivationRequired(err.Error())
		}
		return err
	}
	if resp.LicenseFile == "" {
		return errors.New("license: activation response missing license file")
	}
	payload, err := c.applyLicenseFile(resp.LicenseFile)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.licenseKey = resp.LicenseKey
	c.mu.Unlock()
	c.log.Infof("license: activation successful, expires %s", payload.ExpiresAt.Format(time.RFC3339))
	return nil
}

func (c *Client) applyLicenseFile(base64File string) (*models.LicensePayload, error) {
	raw, err := base64.StdEncoding.DecodeString(base64File)
	if err != nil {
		return nil, fmt.Errorf("license: decode license file: %w", err)
	}
	payload, err := c.validateAndStore(raw)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.current = payload
	if c.callbacks.OnLicenseUpdated != nil {
		go c.callbacks.OnLicenseUpdated(payload)
	}
	c.mu.Unlock()
	return payload, nil
}

func (c *Client) validateAndStore(raw []byte) (*models.LicensePayload, error) {
	normalized, err := normalizeLicenseBytes(raw)
	if err != nil {
		return nil, err
	}
	payload, err := c.validator.Verify(normalized, c.fingerprint)
	if err != nil {
		return nil, err
	}
	if err := c.store.Save(context.Background(), normalized); err != nil {
		return nil, fmt.Errorf("license: persist license: %w", err)
	}
	return payload, nil
}

func (c *Client) startHeartbeat() {
	c.mu.RLock()
	licenseKey := c.licenseKey
	c.mu.RUnlock()
	if licenseKey == "" {
		c.log.Warnf("license: heartbeat skipped due to empty license key")
		return
	}
	req := &models.HeartbeatRequest{
		LicenseKey:          licenseKey,
		HardwareFingerprint: c.fingerprint,
	}
	callbacks := heartbeat.Callbacks{
		OnLicenseUpdated: c.handleHeartbeatUpdate,
		OnError:          c.handleHeartbeatError,
	}
	mgr := heartbeat.NewManager(c.heartbeatS, req, c.cfg.HeartbeatIntervalOrDefault(), callbacks, c.log)
	mgr.Start()
	c.heartbeatM = mgr
}

func (c *Client) handleHeartbeatUpdate(resp *models.HeartbeatResponse) {
	if resp.LicenseFile != "" {
		if _, err := c.applyLicenseFile(resp.LicenseFile); err != nil {
			c.log.Warnf("license: apply heartbeat license failed: %v", err)
		}
	}
	if resp.Status == "activation_required" && c.callbacks.OnActivationRequired != nil {
		c.callbacks.OnActivationRequired("heartbeat requested reactivation")
	}
}

func (c *Client) handleHeartbeatError(err error) {
	if c.callbacks.OnHeartbeatError != nil {
		c.callbacks.OnHeartbeatError(err)
	}
}

func normalizeLicenseBytes(raw []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("license: empty license file")
	}
	if trimmed[0] == '{' {
		return trimmed, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err != nil {
		return nil, fmt.Errorf("license: decode base64 license: %w", err)
	}
	decoded = bytes.TrimSpace(decoded)
	if len(decoded) == 0 || decoded[0] != '{' {
		return nil, errors.New("license: unsupported license file format")
	}
	return decoded, nil
}
