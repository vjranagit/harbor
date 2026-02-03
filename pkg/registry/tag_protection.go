// Copyright 2021 vjranagit
//
// Tag protection and immutability policies

package registry

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"
)

// ProtectionPolicy defines tag protection rules
type ProtectionPolicy struct {
	Name        string
	Pattern     *regexp.Regexp
	Immutable   bool
	MaxAge      time.Duration
	AllowDelete bool
	Priority    int
}

// TagProtection manages tag protection policies
type TagProtection struct {
	policies []*ProtectionPolicy
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewTagProtection creates a new tag protection manager
func NewTagProtection() *TagProtection {
	return &TagProtection{
		policies: make([]*ProtectionPolicy, 0),
		logger:   slog.Default().With("component", "tag_protection"),
	}
}

// AddPolicy adds a new protection policy
func (tp *TagProtection) AddPolicy(policy *ProtectionPolicy) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Validate pattern
	if policy.Pattern == nil {
		return fmt.Errorf("policy pattern cannot be nil")
	}

	tp.policies = append(tp.policies, policy)
	tp.logger.Info("policy added", "name", policy.Name, "pattern", policy.Pattern.String())
	return nil
}

// CanModify checks if a tag can be modified based on policies
func (tp *TagProtection) CanModify(ctx context.Context, repository, tag string, age time.Duration) (bool, string) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	tagRef := fmt.Sprintf("%s:%s", repository, tag)

	// Find matching policies (highest priority first)
	var matchedPolicy *ProtectionPolicy
	for _, policy := range tp.policies {
		if policy.Pattern.MatchString(tagRef) {
			if matchedPolicy == nil || policy.Priority > matchedPolicy.Priority {
				matchedPolicy = policy
			}
		}
	}

	if matchedPolicy == nil {
		return true, ""
	}

	// Check immutability
	if matchedPolicy.Immutable {
		tp.logger.WarnContext(ctx, "tag modification blocked by immutability policy",
			"tag", tagRef,
			"policy", matchedPolicy.Name,
		)
		return false, fmt.Sprintf("tag is immutable (policy: %s)", matchedPolicy.Name)
	}

	// Check age-based protection
	if matchedPolicy.MaxAge > 0 && age < matchedPolicy.MaxAge {
		tp.logger.WarnContext(ctx, "tag modification blocked by age policy",
			"tag", tagRef,
			"age", age,
			"max_age", matchedPolicy.MaxAge,
			"policy", matchedPolicy.Name,
		)
		return false, fmt.Sprintf("tag protected for %s (policy: %s)", matchedPolicy.MaxAge, matchedPolicy.Name)
	}

	return true, ""
}

// CanDelete checks if a tag can be deleted based on policies
func (tp *TagProtection) CanDelete(ctx context.Context, repository, tag string) (bool, string) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	tagRef := fmt.Sprintf("%s:%s", repository, tag)

	for _, policy := range tp.policies {
		if policy.Pattern.MatchString(tagRef) && !policy.AllowDelete {
			tp.logger.WarnContext(ctx, "tag deletion blocked",
				"tag", tagRef,
				"policy", policy.Name,
			)
			return false, fmt.Sprintf("tag deletion not allowed (policy: %s)", policy.Name)
		}
	}

	return true, ""
}

// ListPolicies returns all configured policies
func (tp *TagProtection) ListPolicies() []*ProtectionPolicy {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	policies := make([]*ProtectionPolicy, len(tp.policies))
	copy(policies, tp.policies)
	return policies
}

// RemovePolicy removes a policy by name
func (tp *TagProtection) RemovePolicy(name string) bool {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	for i, policy := range tp.policies {
		if policy.Name == name {
			tp.policies = append(tp.policies[:i], tp.policies[i+1:]...)
			tp.logger.Info("policy removed", "name", name)
			return true
		}
	}
	return false
}
