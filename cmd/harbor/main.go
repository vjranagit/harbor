// Copyright 2021 vjranagit
//
// Harbor Toolkit - Unified CLI for deployment and image acceleration

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"

	cfgFile string
	verbose bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "harbor",
		Short: "Harbor Toolkit - Deployment and image acceleration for Harbor",
		Long: `Harbor Toolkit provides unified CLI for:
  - Harbor deployment orchestration (docker, k8s, compose)
  - Image acceleration (nydus, estargz)
  - Registry management (tag protection, batch ops, health monitoring)
  - Configuration management

Reimplemented from harbor-operator and acceleration-service with modern Go patterns.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, gitCommit, buildDate),
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Setup logging
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}))
		slog.SetDefault(logger)
	}

	// Add subcommands
	rootCmd.AddCommand(
		newDeployCmd(),
		newAccelerateCmd(),
		newRegistryCmd(),
		newServerCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("harbor-toolkit %s\n", version)
			fmt.Printf("  commit: %s\n", gitCommit)
			fmt.Printf("  built:  %s\n", buildDate)
		},
	}
}
