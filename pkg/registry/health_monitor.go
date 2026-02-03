// Copyright 2021 vjranagit
//
// Enhanced health monitoring with circuit breaker

package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// HealthStatus represents endpoint health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// CircuitState represents circuit breaker state
type CircuitState string

const (
	CircuitClosed    CircuitState = "closed"
	CircuitHalfOpen  CircuitState = "half_open"
	CircuitOpen      CircuitState = "open"
)

// HealthCheck represents a health check result
type HealthCheck struct {
	Endpoint    string
	Status      HealthStatus
	Circuit     CircuitState
	Latency     time.Duration
	Error       string
	LastCheck   time.Time
	Consecutive int
	Attempts    int
}

// HealthMonitor monitors registry endpoint health with circuit breaker
type HealthMonitor struct {
	checks          map[string]*HealthCheck
	mu              sync.RWMutex
	threshold       int
	retryDelay      time.Duration
	timeout         time.Duration
	checkInterval   time.Duration
	logger          *slog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(threshold int, retryDelay, timeout, checkInterval time.Duration) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		checks:        make(map[string]*HealthCheck),
		threshold:     threshold,
		retryDelay:    retryDelay,
		timeout:       timeout,
		checkInterval: checkInterval,
		logger:        slog.Default().With("component", "health_monitor"),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Register adds an endpoint for monitoring
func (hm *HealthMonitor) Register(endpoint string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.checks[endpoint]; exists {
		return
	}

	hm.checks[endpoint] = &HealthCheck{
		Endpoint: endpoint,
		Status:   HealthStatusUnknown,
		Circuit:  CircuitClosed,
	}

	hm.logger.Info("endpoint registered", "endpoint", endpoint)
}

// Start begins health monitoring
func (hm *HealthMonitor) Start() {
	hm.logger.Info("starting health monitor", "interval", hm.checkInterval)

	for endpoint := range hm.checks {
		hm.wg.Add(1)
		go hm.monitorEndpoint(endpoint)
	}
}

// Stop gracefully stops health monitoring
func (hm *HealthMonitor) Stop() {
	hm.logger.Info("stopping health monitor")
	hm.cancel()
	hm.wg.Wait()
	hm.logger.Info("health monitor stopped")
}

// GetStatus returns the health status of an endpoint
func (hm *HealthMonitor) GetStatus(endpoint string) (*HealthCheck, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	check, ok := hm.checks[endpoint]
	return check, ok
}

// GetAllStatuses returns all health checks
func (hm *HealthMonitor) GetAllStatuses() map[string]*HealthCheck {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	statuses := make(map[string]*HealthCheck, len(hm.checks))
	for endpoint, check := range hm.checks {
		statuses[endpoint] = check
	}
	return statuses
}

// monitorEndpoint continuously monitors an endpoint
func (hm *HealthMonitor) monitorEndpoint(endpoint string) {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.performCheck(endpoint)

		case <-hm.ctx.Done():
			return
		}
	}
}

// performCheck executes a health check with circuit breaker logic
func (hm *HealthMonitor) performCheck(endpoint string) {
	hm.mu.RLock()
	check := hm.checks[endpoint]
	hm.mu.RUnlock()

	// Circuit breaker: skip check if open and not ready for retry
	if check.Circuit == CircuitOpen {
		if time.Since(check.LastCheck) < hm.retryDelay {
			return
		}
		// Move to half-open for retry
		hm.updateCircuit(endpoint, CircuitHalfOpen)
	}

	// Perform health check with timeout
	ctx, cancel := context.WithTimeout(hm.ctx, hm.timeout)
	defer cancel()

	start := time.Now()
	err := hm.checkEndpoint(ctx, endpoint)
	latency := time.Since(start)

	hm.updateHealth(endpoint, err, latency)
}

// checkEndpoint performs the actual health check
func (hm *HealthMonitor) checkEndpoint(ctx context.Context, endpoint string) error {
	// Simulate health check (would call actual registry API)
	select {
	case <-ctx.Done():
		return fmt.Errorf("health check timeout")
	case <-time.After(50 * time.Millisecond):
		// Simulate 10% failure rate for testing
		if time.Now().UnixNano()%10 == 0 {
			return fmt.Errorf("simulated failure")
		}
		return nil
	}
}

// updateHealth updates health status based on check result
func (hm *HealthMonitor) updateHealth(endpoint string, err error, latency time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	check := hm.checks[endpoint]
	check.LastCheck = time.Now()
	check.Latency = latency
	check.Attempts++

	if err != nil {
		check.Error = err.Error()
		check.Consecutive++

		// Update status based on consecutive failures
		if check.Consecutive >= hm.threshold {
			check.Status = HealthStatusUnhealthy
			check.Circuit = CircuitOpen
			hm.logger.Error("endpoint unhealthy, circuit opened",
				"endpoint", endpoint,
				"consecutive_failures", check.Consecutive,
			)
		} else if check.Consecutive > 0 {
			check.Status = HealthStatusDegraded
		}
	} else {
		// Successful check
		check.Error = ""
		check.Consecutive = 0
		check.Status = HealthStatusHealthy

		// Close circuit if it was open/half-open
		if check.Circuit != CircuitClosed {
			check.Circuit = CircuitClosed
			hm.logger.Info("endpoint recovered, circuit closed",
				"endpoint", endpoint,
				"latency_ms", latency.Milliseconds(),
			)
		}
	}
}

// updateCircuit updates circuit breaker state
func (hm *HealthMonitor) updateCircuit(endpoint string, state CircuitState) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	check := hm.checks[endpoint]
	check.Circuit = state
	hm.logger.Info("circuit state changed",
		"endpoint", endpoint,
		"state", state,
	)
}
