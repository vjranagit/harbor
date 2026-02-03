// Copyright 2021 vjranagit
//
// Health monitor tests

package registry

import (
	"testing"
	"time"
)

func TestHealthMonitor_CircuitBreaker(t *testing.T) {
	hm := NewHealthMonitor(
		3,                   // threshold
		5*time.Second,       // retry delay
		2*time.Second,       // timeout
		100*time.Millisecond, // check interval
	)

	endpoint := "https://registry.example.com"
	hm.Register(endpoint)

	// Get initial status
	status, ok := hm.GetStatus(endpoint)
	if !ok {
		t.Fatal("endpoint not registered")
	}

	if status.Status != HealthStatusUnknown {
		t.Errorf("expected status %s, got %s", HealthStatusUnknown, status.Status)
	}

	if status.Circuit != CircuitClosed {
		t.Errorf("expected circuit %s, got %s", CircuitClosed, status.Circuit)
	}
}

func TestHealthMonitor_MultipleEndpoints(t *testing.T) {
	hm := NewHealthMonitor(2, 3*time.Second, 1*time.Second, 200*time.Millisecond)

	endpoints := []string{
		"https://registry1.example.com",
		"https://registry2.example.com",
		"https://registry3.example.com",
	}

	for _, endpoint := range endpoints {
		hm.Register(endpoint)
	}

	statuses := hm.GetAllStatuses()
	if len(statuses) != len(endpoints) {
		t.Errorf("expected %d statuses, got %d", len(endpoints), len(statuses))
	}

	for _, endpoint := range endpoints {
		if _, ok := statuses[endpoint]; !ok {
			t.Errorf("endpoint %s not found in statuses", endpoint)
		}
	}
}

func TestHealthMonitor_StatusTransitions(t *testing.T) {
	hm := NewHealthMonitor(3, 2*time.Second, 1*time.Second, 50*time.Millisecond)

	endpoint := "https://test.example.com"
	hm.Register(endpoint)
	hm.Start()

	// Let monitor run for a bit
	time.Sleep(1 * time.Second)

	status, ok := hm.GetStatus(endpoint)
	if !ok {
		t.Fatal("endpoint not found")
	}

	// Should have attempted at least a few checks
	if status.Attempts == 0 {
		t.Error("expected some health check attempts")
	}

	// Should have updated last check time
	if status.LastCheck.IsZero() {
		t.Error("expected last check time to be set")
	}

	hm.Stop()
}

func TestHealthMonitor_GracefulShutdown(t *testing.T) {
	hm := NewHealthMonitor(2, 1*time.Second, 500*time.Millisecond, 100*time.Millisecond)

	hm.Register("https://test1.example.com")
	hm.Register("https://test2.example.com")

	hm.Start()
	time.Sleep(300 * time.Millisecond)

	// Should complete without hanging
	done := make(chan struct{})
	go func() {
		hm.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}
