// Copyright 2021 vjranagit
//
// Tag protection tests

package registry

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestTagProtection_Immutability(t *testing.T) {
	tp := NewTagProtection()

	// Add immutable policy for production tags
	err := tp.AddPolicy(&ProtectionPolicy{
		Name:      "prod-immutable",
		Pattern:   regexp.MustCompile(`.*:v\d+\.\d+\.\d+$`),
		Immutable: true,
		Priority:  10,
	})
	if err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	tests := []struct {
		name       string
		repository string
		tag        string
		age        time.Duration
		wantModify bool
	}{
		{
			name:       "immutable production tag",
			repository: "library/nginx",
			tag:        "v1.2.3",
			age:        24 * time.Hour,
			wantModify: false,
		},
		{
			name:       "mutable development tag",
			repository: "library/nginx",
			tag:        "latest",
			age:        24 * time.Hour,
			wantModify: true,
		},
		{
			name:       "mutable feature tag",
			repository: "library/app",
			tag:        "feature-123",
			age:        1 * time.Hour,
			wantModify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canModify, reason := tp.CanModify(context.Background(), tt.repository, tt.tag, tt.age)
			if canModify != tt.wantModify {
				t.Errorf("CanModify() = %v (reason: %s), want %v", canModify, reason, tt.wantModify)
			}
		})
	}
}

func TestTagProtection_AgeBased(t *testing.T) {
	tp := NewTagProtection()

	// Protect tags for 7 days
	err := tp.AddPolicy(&ProtectionPolicy{
		Name:     "recent-protection",
		Pattern:  regexp.MustCompile(`.*:.*`),
		MaxAge:   7 * 24 * time.Hour,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	tests := []struct {
		name       string
		repository string
		tag        string
		age        time.Duration
		wantModify bool
	}{
		{
			name:       "recent tag protected",
			repository: "library/redis",
			tag:        "6.2",
			age:        3 * 24 * time.Hour,
			wantModify: false,
		},
		{
			name:       "old tag modifiable",
			repository: "library/redis",
			tag:        "5.0",
			age:        30 * 24 * time.Hour,
			wantModify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canModify, _ := tp.CanModify(context.Background(), tt.repository, tt.tag, tt.age)
			if canModify != tt.wantModify {
				t.Errorf("CanModify() = %v, want %v", canModify, tt.wantModify)
			}
		})
	}
}

func TestTagProtection_Priority(t *testing.T) {
	tp := NewTagProtection()

	// Lower priority: allow modification
	tp.AddPolicy(&ProtectionPolicy{
		Name:      "allow-all",
		Pattern:   regexp.MustCompile(`.*:.*`),
		Immutable: false,
		Priority:  1,
	})

	// Higher priority: block production
	tp.AddPolicy(&ProtectionPolicy{
		Name:      "block-prod",
		Pattern:   regexp.MustCompile(`production/.*:.*`),
		Immutable: true,
		Priority:  10,
	})

	// Should use higher priority policy (block)
	canModify, _ := tp.CanModify(context.Background(), "production/api", "v2.0.0", 1*time.Hour)
	if canModify {
		t.Error("expected production tag to be blocked by higher priority policy")
	}

	// Should use lower priority policy (allow)
	canModify, _ = tp.CanModify(context.Background(), "staging/api", "v2.0.0", 1*time.Hour)
	if !canModify {
		t.Error("expected staging tag to be allowed by lower priority policy")
	}
}
