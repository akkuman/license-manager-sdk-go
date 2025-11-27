package heartbeat

import (
	"context"
	"sync"
	"time"

	"github.com/cedar-v/license-manage-sdk-go/logger"
	"github.com/cedar-v/license-manage-sdk-go/models"
)

// Callbacks capture heartbeat lifecycle events.
type Callbacks struct {
	OnLicenseUpdated func(resp *models.HeartbeatResponse)
	OnError          func(error)
}

// Manager orchestrates the heartbeat loop.
type Manager struct {
	service   *Service
	req       *models.HeartbeatRequest
	interval  time.Duration
	callbacks Callbacks
	log       logger.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	running bool
	paused  bool
}

// NewManager configures a heartbeat manager.
func NewManager(service *Service, req *models.HeartbeatRequest, interval time.Duration, callbacks Callbacks, log logger.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		service:   service,
		req:       req,
		interval:  interval,
		callbacks: callbacks,
		log:       log,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the background loop.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.running = true
	go m.loop()
}

// Stop terminates the loop.
func (m *Manager) Stop() {
	m.cancel()
}

// Pause temporarily halts heartbeats.
func (m *Manager) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = true
}

// Resume restarts after pause.
func (m *Manager) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = false
}

func (m *Manager) loop() {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()
	interval := m.interval
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(interval):
			m.mu.Lock()
			paused := m.paused
			m.mu.Unlock()
			if paused {
				continue
			}
			resp, err := m.service.Send(m.ctx, m.req)
			if err != nil {
				interval = backoff(interval)
				m.log.Warnf("heartbeat failed: %v", err)
				if m.callbacks.OnError != nil {
					go m.callbacks.OnError(err)
				}
				continue
			}
			if resp.HeartbeatInterval > 0 {
				interval = time.Duration(resp.HeartbeatInterval) * time.Second
			} else {
				interval = m.interval
			}
			if resp.LicenseFile != "" && m.callbacks.OnLicenseUpdated != nil {
				go m.callbacks.OnLicenseUpdated(resp)
			}
		}
	}
}

func backoff(current time.Duration) time.Duration {
	next := current * 2
	if next > 30*time.Minute {
		next = 30 * time.Minute
	}
	if next < 30*time.Second {
		next = 30 * time.Second
	}
	return next
}
