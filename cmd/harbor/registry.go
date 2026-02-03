// Copyright 2021 vjranagit
//
// Registry management commands

package main

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"github.com/vjranagit/harbor/pkg/registry"
)

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Registry management operations",
		Long:  "Manage registry operations including tag protection, batch operations, and health monitoring",
	}

	cmd.AddCommand(
		newTagProtectionCmd(),
		newBatchOpsCmd(),
		newHealthCmd(),
	)

	return cmd
}

func newTagProtectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protect",
		Short: "Manage tag protection policies",
	}

	// Add policy
	addPolicy := &cobra.Command{
		Use:   "add",
		Short: "Add a tag protection policy",
		Example: `  # Protect production tags from modification
  harbor registry protect add --name prod-immutable --pattern '.*:v\d+\.\d+\.\d+$' --immutable

  # Protect tags for 7 days
  harbor registry protect add --name recent --pattern '.*:.*' --max-age 168h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			pattern, _ := cmd.Flags().GetString("pattern")
			immutable, _ := cmd.Flags().GetBool("immutable")
			maxAge, _ := cmd.Flags().GetDuration("max-age")

			tp := registry.NewTagProtection()
			policy := &registry.ProtectionPolicy{
				Name:      name,
				Pattern:   regexp.MustCompile(pattern),
				Immutable: immutable,
				MaxAge:    maxAge,
				Priority:  10,
			}

			if err := tp.AddPolicy(policy); err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}

			fmt.Printf("✓ Policy '%s' added successfully\n", name)
			return nil
		},
	}
	addPolicy.Flags().String("name", "", "Policy name (required)")
	addPolicy.Flags().String("pattern", "", "Tag pattern regex (required)")
	addPolicy.Flags().Bool("immutable", false, "Make tags immutable")
	addPolicy.Flags().Duration("max-age", 0, "Protection duration (e.g., 168h for 7 days)")
	addPolicy.MarkFlagRequired("name")
	addPolicy.MarkFlagRequired("pattern")

	cmd.AddCommand(addPolicy)
	return cmd
}

func newBatchOpsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch operations on tags",
	}

	// Delete tags
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete multiple tags in batch",
		Example: `  # Delete old tags
  harbor registry batch delete library/nginx:old-1 library/nginx:old-2 library/redis:deprecated`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("no tags specified")
			}

			bo := registry.NewBatchOperator(5)
			op, err := bo.DeleteTags(context.Background(), args)
			if err != nil {
				return fmt.Errorf("batch delete failed: %w", err)
			}

			fmt.Printf("✓ Batch delete initiated (ID: %s)\n", op.ID)
			fmt.Printf("  Tags: %d\n", len(args))
			fmt.Printf("  Status: %s\n", op.Status)
			return nil
		},
	}

	// Copy tags
	copyCmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy multiple tags in batch",
		Example: `  # Copy tags to backup repository
  harbor registry batch copy --dest backup/ library/nginx:1.20 library/nginx:1.21`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("no tags specified")
			}

			dest, _ := cmd.Flags().GetString("dest")
			if dest == "" {
				return fmt.Errorf("--dest required")
			}

			bo := registry.NewBatchOperator(5)
			op, err := bo.CopyTags(context.Background(), args, dest)
			if err != nil {
				return fmt.Errorf("batch copy failed: %w", err)
			}

			fmt.Printf("✓ Batch copy initiated (ID: %s)\n", op.ID)
			fmt.Printf("  Sources: %d\n", len(args))
			fmt.Printf("  Destination: %s\n", dest)
			return nil
		},
	}
	copyCmd.Flags().String("dest", "", "Destination prefix (required)")

	// Retag
	retagCmd := &cobra.Command{
		Use:   "retag",
		Short: "Retag multiple images in batch",
		Example: `  # Retag latest to versioned tags
  harbor registry batch retag --mapping library/app:latest=library/app:v1.0.0 --mapping library/app:nightly=library/app:v1.1.0-beta`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mappings, _ := cmd.Flags().GetStringToString("mapping")
			if len(mappings) == 0 {
				return fmt.Errorf("no mappings specified")
			}

			bo := registry.NewBatchOperator(5)
			op, err := bo.RetagBatch(context.Background(), mappings)
			if err != nil {
				return fmt.Errorf("batch retag failed: %w", err)
			}

			fmt.Printf("✓ Batch retag initiated (ID: %s)\n", op.ID)
			fmt.Printf("  Mappings: %d\n", len(mappings))
			return nil
		},
	}
	retagCmd.Flags().StringToString("mapping", nil, "Tag mappings (source=dest)")

	cmd.AddCommand(deleteCmd, copyCmd, retagCmd)
	return cmd
}

func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Health monitoring with circuit breaker",
	}

	// Monitor endpoints
	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor registry endpoint health",
		Example: `  # Monitor multiple registries
  harbor registry health monitor https://registry1.example.com https://registry2.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("no endpoints specified")
			}

			threshold, _ := cmd.Flags().GetInt("threshold")
			retryDelay, _ := cmd.Flags().GetDuration("retry-delay")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			interval, _ := cmd.Flags().GetDuration("interval")

			hm := registry.NewHealthMonitor(threshold, retryDelay, timeout, interval)

			for _, endpoint := range args {
				hm.Register(endpoint)
			}

			hm.Start()

			fmt.Printf("✓ Monitoring %d endpoints\n", len(args))
			fmt.Printf("  Threshold: %d consecutive failures\n", threshold)
			fmt.Printf("  Check interval: %s\n", interval)
			fmt.Printf("\nPress Ctrl+C to stop...\n")

			// Monitor for a bit and show status
			time.Sleep(10 * time.Second)

			statuses := hm.GetAllStatuses()
			fmt.Printf("\n=== Health Status ===\n")
			for endpoint, status := range statuses {
				fmt.Printf("%s: %s (circuit: %s, attempts: %d)\n",
					endpoint, status.Status, status.Circuit, status.Attempts)
			}

			hm.Stop()
			return nil
		},
	}
	monitorCmd.Flags().Int("threshold", 3, "Failure threshold before circuit opens")
	monitorCmd.Flags().Duration("retry-delay", 30*time.Second, "Delay before retrying failed endpoint")
	monitorCmd.Flags().Duration("timeout", 5*time.Second, "Health check timeout")
	monitorCmd.Flags().Duration("interval", 10*time.Second, "Check interval")

	cmd.AddCommand(monitorCmd)
	return cmd
}
