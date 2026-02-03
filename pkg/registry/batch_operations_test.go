// Copyright 2021 vjranagit
//
// Batch operations tests

package registry

import (
	"context"
	"testing"
	"time"
)

func TestBatchOperator_DeleteTags(t *testing.T) {
	bo := NewBatchOperator(5)
	tags := []string{
		"library/nginx:old-1",
		"library/nginx:old-2",
		"library/redis:deprecated",
	}

	op, err := bo.DeleteTags(context.Background(), tags)
	if err != nil {
		t.Fatalf("DeleteTags failed: %v", err)
	}

	if op.Type != BatchOpDelete {
		t.Errorf("expected type %s, got %s", BatchOpDelete, op.Type)
	}

	if len(op.Targets) != len(tags) {
		t.Errorf("expected %d targets, got %d", len(tags), len(op.Targets))
	}

	// Wait for completion
	time.Sleep(1 * time.Second)

	retrieved, ok := bo.GetOperation(op.ID)
	if !ok {
		t.Fatal("operation not found")
	}

	if retrieved.Status != BatchOpCompleted {
		t.Errorf("expected status %s, got %s", BatchOpCompleted, retrieved.Status)
	}

	if len(retrieved.Results) != len(tags) {
		t.Errorf("expected %d results, got %d", len(tags), len(retrieved.Results))
	}
}

func TestBatchOperator_CopyTags(t *testing.T) {
	bo := NewBatchOperator(3)
	sources := []string{
		"library/nginx:1.20",
		"library/nginx:1.21",
	}

	op, err := bo.CopyTags(context.Background(), sources, "backup/")
	if err != nil {
		t.Fatalf("CopyTags failed: %v", err)
	}

	if op.Type != BatchOpCopy {
		t.Errorf("expected type %s, got %s", BatchOpCopy, op.Type)
	}

	// Wait for completion
	time.Sleep(1 * time.Second)

	retrieved, ok := bo.GetOperation(op.ID)
	if !ok {
		t.Fatal("operation not found")
	}

	if retrieved.Status != BatchOpCompleted {
		t.Errorf("expected status %s, got %s", BatchOpCompleted, retrieved.Status)
	}
}

func TestBatchOperator_RetagBatch(t *testing.T) {
	bo := NewBatchOperator(4)
	mappings := map[string]string{
		"library/app:latest":   "library/app:v1.0.0",
		"library/app:nightly":  "library/app:v1.1.0-beta",
		"library/app:unstable": "library/app:v2.0.0-alpha",
	}

	op, err := bo.RetagBatch(context.Background(), mappings)
	if err != nil {
		t.Fatalf("RetagBatch failed: %v", err)
	}

	if op.Type != BatchOpTag {
		t.Errorf("expected type %s, got %s", BatchOpTag, op.Type)
	}

	// Wait for completion
	time.Sleep(1 * time.Second)

	retrieved, ok := bo.GetOperation(op.ID)
	if !ok {
		t.Fatal("operation not found")
	}

	if retrieved.Status != BatchOpCompleted {
		t.Errorf("expected status %s, got %s", BatchOpCompleted, retrieved.Status)
	}

	if len(retrieved.Results) != len(mappings) {
		t.Errorf("expected %d results, got %d", len(mappings), len(retrieved.Results))
	}

	// Verify all succeeded
	for _, result := range retrieved.Results {
		if !result.Success {
			t.Errorf("operation failed for %s: %s", result.Target, result.Error)
		}
	}
}

func TestBatchOperator_ListOperations(t *testing.T) {
	bo := NewBatchOperator(2)

	// Create multiple operations
	bo.DeleteTags(context.Background(), []string{"test:1"})
	bo.CopyTags(context.Background(), []string{"test:2"}, "backup/")

	time.Sleep(500 * time.Millisecond)

	ops := bo.ListOperations()
	if len(ops) != 2 {
		t.Errorf("expected 2 operations, got %d", len(ops))
	}
}
