// Copyright 2021 vjranagit
//
// Batch operations for registry management

package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// BatchOperation represents a batch operation request
type BatchOperation struct {
	ID        string
	Type      BatchOpType
	Targets   []string
	Status    BatchOpStatus
	Results   []BatchOpResult
	CreatedAt time.Time
	StartedAt time.Time
	EndedAt   time.Time
}

// BatchOpType defines the type of batch operation
type BatchOpType string

const (
	BatchOpDelete  BatchOpType = "delete"
	BatchOpTag     BatchOpType = "tag"
	BatchOpConvert BatchOpType = "convert"
	BatchOpCopy    BatchOpType = "copy"
)

// BatchOpStatus represents operation status
type BatchOpStatus string

const (
	BatchOpPending   BatchOpStatus = "pending"
	BatchOpRunning   BatchOpStatus = "running"
	BatchOpCompleted BatchOpStatus = "completed"
	BatchOpFailed    BatchOpStatus = "failed"
)

// BatchOpResult represents the result of a single operation
type BatchOpResult struct {
	Target  string
	Success bool
	Error   string
	Elapsed time.Duration
}

// BatchOperator manages batch operations
type BatchOperator struct {
	operations map[string]*BatchOperation
	mu         sync.RWMutex
	workers    int
	logger     *slog.Logger
}

// NewBatchOperator creates a new batch operator
func NewBatchOperator(workers int) *BatchOperator {
	return &BatchOperator{
		operations: make(map[string]*BatchOperation),
		workers:    workers,
		logger:     slog.Default().With("component", "batch_operator"),
	}
}

// DeleteTags performs batch deletion of tags
func (bo *BatchOperator) DeleteTags(ctx context.Context, tags []string) (*BatchOperation, error) {
	op := &BatchOperation{
		ID:        generateID(),
		Type:      BatchOpDelete,
		Targets:   tags,
		Status:    BatchOpPending,
		CreatedAt: time.Now(),
	}

	bo.mu.Lock()
	bo.operations[op.ID] = op
	bo.mu.Unlock()

	bo.logger.InfoContext(ctx, "batch delete initiated",
		"id", op.ID,
		"count", len(tags),
	)

	// Execute batch operation
	go bo.executeBatch(ctx, op, func(ctx context.Context, target string) error {
		// Simulate tag deletion (would call actual registry API)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	return op, nil
}

// CopyTags performs batch copying of tags
func (bo *BatchOperator) CopyTags(ctx context.Context, sources []string, destPrefix string) (*BatchOperation, error) {
	op := &BatchOperation{
		ID:        generateID(),
		Type:      BatchOpCopy,
		Targets:   sources,
		Status:    BatchOpPending,
		CreatedAt: time.Now(),
	}

	bo.mu.Lock()
	bo.operations[op.ID] = op
	bo.mu.Unlock()

	bo.logger.InfoContext(ctx, "batch copy initiated",
		"id", op.ID,
		"count", len(sources),
		"dest_prefix", destPrefix,
	)

	go bo.executeBatch(ctx, op, func(ctx context.Context, source string) error {
		// Simulate tag copy (would call actual registry API)
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	return op, nil
}

// RetagBatch performs batch retagging operations
func (bo *BatchOperator) RetagBatch(ctx context.Context, mappings map[string]string) (*BatchOperation, error) {
	targets := make([]string, 0, len(mappings))
	for source := range mappings {
		targets = append(targets, source)
	}

	op := &BatchOperation{
		ID:        generateID(),
		Type:      BatchOpTag,
		Targets:   targets,
		Status:    BatchOpPending,
		CreatedAt: time.Now(),
	}

	bo.mu.Lock()
	bo.operations[op.ID] = op
	bo.mu.Unlock()

	bo.logger.InfoContext(ctx, "batch retag initiated",
		"id", op.ID,
		"count", len(mappings),
	)

	go bo.executeBatch(ctx, op, func(ctx context.Context, source string) error {
		dest := mappings[source]
		// Simulate retagging (would call actual registry API)
		_ = dest
		time.Sleep(150 * time.Millisecond)
		return nil
	})

	return op, nil
}

// GetOperation retrieves a batch operation by ID
func (bo *BatchOperator) GetOperation(id string) (*BatchOperation, bool) {
	bo.mu.RLock()
	defer bo.mu.RUnlock()

	op, ok := bo.operations[id]
	return op, ok
}

// ListOperations returns all batch operations
func (bo *BatchOperator) ListOperations() []*BatchOperation {
	bo.mu.RLock()
	defer bo.mu.RUnlock()

	ops := make([]*BatchOperation, 0, len(bo.operations))
	for _, op := range bo.operations {
		ops = append(ops, op)
	}
	return ops
}

// executeBatch runs a batch operation with worker pool
func (bo *BatchOperator) executeBatch(ctx context.Context, op *BatchOperation, handler func(context.Context, string) error) {
	// Update status to running
	bo.mu.Lock()
	op.Status = BatchOpRunning
	op.StartedAt = time.Now()
	bo.mu.Unlock()

	results := make([]BatchOpResult, len(op.Targets))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, bo.workers)

	for i, target := range op.Targets {
		wg.Add(1)
		go func(idx int, tgt string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			start := time.Now()
			err := handler(ctx, tgt)
			elapsed := time.Since(start)

			results[idx] = BatchOpResult{
				Target:  tgt,
				Success: err == nil,
				Elapsed: elapsed,
			}
			if err != nil {
				results[idx].Error = err.Error()
			}
		}(i, target)
	}

	wg.Wait()

	// Update final status
	bo.mu.Lock()
	op.Results = results
	op.EndedAt = time.Now()
	op.Status = BatchOpCompleted

	// Check if any failed
	for _, result := range results {
		if !result.Success {
			op.Status = BatchOpFailed
			break
		}
	}
	bo.mu.Unlock()

	bo.logger.InfoContext(ctx, "batch operation completed",
		"id", op.ID,
		"status", op.Status,
		"duration", op.EndedAt.Sub(op.StartedAt),
	)
}

// generateID generates a unique operation ID
func generateID() string {
	return fmt.Sprintf("batch-%d", time.Now().UnixNano())
}
