package cmd

import (
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/autoupdate"
	"github.com/spf13/cobra"
)

// autoUpdateCmd represents the auto-update parent command
var autoUpdateCmd = &cobra.Command{
	Use:   "auto-update",
	Short: "Manage automatic update settings",
	Long: `Manage automatic update settings for installed Go modules.

When enabled, glix will periodically check for updates to all installed
modules and automatically install newer versions.

Examples:
  glix auto-update status     # Show current auto-update status
  glix auto-update enable     # Enable automatic updates
  glix auto-update disable    # Disable automatic updates
  glix auto-update now        # Run an update check immediately
  glix auto-update config     # Configure auto-update settings`,
}

// autoUpdateStatusCmd shows the current auto-update status
var autoUpdateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show auto-update status",
	Long:  "Display the current auto-update configuration and statistics.",
	RunE:  runAutoUpdateStatus,
}

// autoUpdateEnableCmd enables auto-update
var autoUpdateEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable automatic updates",
	Long:  "Enable automatic checking and installation of module updates.",
	RunE:  runAutoUpdateEnable,
}

// autoUpdateDisableCmd disables auto-update
var autoUpdateDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable automatic updates",
	Long:  "Disable automatic checking and installation of module updates.",
	RunE:  runAutoUpdateDisable,
}

// autoUpdateNowCmd runs an update check immediately
var autoUpdateNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Run update check immediately",
	Long:  "Perform an immediate update check for all installed modules.",
	RunE:  runAutoUpdateNow,
}

// autoUpdateConfigCmd configures auto-update settings
var autoUpdateConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure auto-update settings",
	Long: `Configure auto-update settings such as check interval and behavior.

Examples:
  glix auto-update config --interval 12h    # Check every 12 hours
  glix auto-update config --notify-only     # Only notify, don't auto-install
  glix auto-update config --no-notify-only  # Auto-install updates`,
	RunE: runAutoUpdateConfig,
}

var (
	autoUpdateInterval   string
	autoUpdateNotifyOnly bool
	autoUpdateNoNotify   bool
)

func init() {
	rootCmd.AddCommand(autoUpdateCmd)

	autoUpdateCmd.AddCommand(autoUpdateStatusCmd)
	autoUpdateCmd.AddCommand(autoUpdateEnableCmd)
	autoUpdateCmd.AddCommand(autoUpdateDisableCmd)
	autoUpdateCmd.AddCommand(autoUpdateNowCmd)
	autoUpdateCmd.AddCommand(autoUpdateConfigCmd)

	// Config flags
	autoUpdateConfigCmd.Flags().StringVar(&autoUpdateInterval, "interval", "", "Update check interval (e.g., 24h, 12h, 1h)")
	autoUpdateConfigCmd.Flags().BoolVar(&autoUpdateNotifyOnly, "notify-only", false, "Only notify about updates, don't auto-install")
	autoUpdateConfigCmd.Flags().BoolVar(&autoUpdateNoNotify, "no-notify-only", false, "Auto-install updates (disable notify-only)")
}

func runAutoUpdateStatus(cmd *cobra.Command, _ []string) error {
	store := autoupdate.GetStore()
	cfg := store.Get()

	cmd.Println("Auto-Update Status")
	cmd.Println("==================")
	cmd.Println()

	if cfg.Enabled {
		cmd.Println("Status:        ENABLED")
	} else {
		cmd.Println("Status:        DISABLED")
	}

	cmd.Printf("Interval:      %s\n", formatDuration(cfg.Interval))

	if cfg.NotifyOnly {
		cmd.Println("Mode:          Notify only (no auto-install)")
	} else {
		cmd.Println("Mode:          Auto-install updates")
	}

	cmd.Println()
	cmd.Println("Statistics")
	cmd.Println("----------")

	if cfg.LastCheck.IsZero() {
		cmd.Println("Last check:    Never")
	} else {
		cmd.Printf("Last check:    %s (%s ago)\n",
			cfg.LastCheck.Format(time.RFC3339),
			formatDuration(time.Since(cfg.LastCheck)))
	}

	if cfg.LastUpdate.IsZero() {
		cmd.Println("Last update:   Never")
	} else {
		cmd.Printf("Last update:   %s (%s ago)\n",
			cfg.LastUpdate.Format(time.RFC3339),
			formatDuration(time.Since(cfg.LastUpdate)))
	}

	cmd.Printf("Total checks:  %d\n", cfg.CheckedCount)
	cmd.Printf("Total updates: %d\n", cfg.UpdatedCount)

	// Show next check time if enabled
	if cfg.Enabled && !cfg.LastCheck.IsZero() {
		nextCheck := cfg.LastCheck.Add(cfg.Interval)
		if nextCheck.After(time.Now()) {
			cmd.Printf("\nNext check:    %s (in %s)\n",
				nextCheck.Format(time.RFC3339),
				formatDuration(time.Until(nextCheck)))
		} else {
			cmd.Println("\nNext check:    Pending (will run soon)")
		}
	}

	return nil
}

func runAutoUpdateEnable(cmd *cobra.Command, _ []string) error {
	store := autoupdate.GetStore()

	if err := store.SetEnabled(true); err != nil {
		return fmt.Errorf("failed to enable auto-update: %w", err)
	}

	cmd.Println("Auto-update enabled")
	cmd.Println()
	cmd.Println("Note: Auto-update runs when the glix server is running.")
	cmd.Println("Use 'glix service start' to start the server, or it will")
	cmd.Println("start automatically when you run glix commands.")

	return nil
}

func runAutoUpdateDisable(cmd *cobra.Command, _ []string) error {
	store := autoupdate.GetStore()

	if err := store.SetEnabled(false); err != nil {
		return fmt.Errorf("failed to disable auto-update: %w", err)
	}

	cmd.Println("Auto-update disabled")

	return nil
}

func runAutoUpdateNow(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cmd.Println("Running update check...")
	cmd.Println()

	scheduler := autoupdate.NewScheduler(nil)

	result, err := scheduler.RunOnce(ctx)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}

	// Display results
	if result.ModulesCount == 0 {
		cmd.Println("No modules installed")
		return nil
	}

	cmd.Printf("Checked %d module(s)\n", result.ModulesCount)
	cmd.Println()

	// Show updates
	var updated, available, upToDate, errors int

	for _, r := range result.Results {
		if r.Error != nil {
			errors++
			continue
		}

		if r.NewVersion == r.PreviousVersion {
			upToDate++
			continue
		}

		if r.Updated {
			updated++

			cmd.Printf("Updated: %s\n", r.Name)
			cmd.Printf("  %s -> %s\n", r.PreviousVersion, r.NewVersion)
		} else {
			available++

			cmd.Printf("Update available: %s\n", r.Name)
			cmd.Printf("  %s -> %s\n", r.PreviousVersion, r.NewVersion)
		}
	}

	cmd.Println()
	cmd.Printf("Summary: %d up to date, %d updated, %d available, %d error(s)\n",
		upToDate, updated, available, errors)

	return nil
}

func runAutoUpdateConfig(cmd *cobra.Command, _ []string) error {
	store := autoupdate.GetStore()
	changed := false

	// Handle interval
	if autoUpdateInterval != "" {
		interval, err := time.ParseDuration(autoUpdateInterval)
		if err != nil {
			return fmt.Errorf("invalid interval format: %w", err)
		}

		if err := store.SetInterval(interval); err != nil {
			return err
		}

		cmd.Printf("Interval set to: %s\n", formatDuration(interval))

		changed = true
	}

	// Handle notify-only flags
	if autoUpdateNotifyOnly {
		if err := store.SetNotifyOnly(true); err != nil {
			return err
		}

		cmd.Println("Mode set to: notify-only (no auto-install)")

		changed = true
	} else if autoUpdateNoNotify {
		if err := store.SetNotifyOnly(false); err != nil {
			return err
		}

		cmd.Println("Mode set to: auto-install updates")

		changed = true
	}

	if !changed {
		// Show current config
		cfg := store.Get()

		cmd.Println("Current configuration:")
		cmd.Printf("  Interval:     %s\n", formatDuration(cfg.Interval))

		if cfg.NotifyOnly {
			cmd.Println("  Mode:         notify-only")
		} else {
			cmd.Println("  Mode:         auto-install")
		}

		cmd.Println()
		cmd.Println("Use flags to modify:")
		cmd.Println("  --interval <duration>   Set check interval (e.g., 24h, 12h)")
		cmd.Println("  --notify-only           Only notify, don't auto-install")
		cmd.Println("  --no-notify-only        Auto-install updates")
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}

	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	if d < 24*time.Hour {
		hours := int(d.Hours())

		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}

		return fmt.Sprintf("%dh", hours)
	}

	days := int(d.Hours()) / 24

	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}

	return fmt.Sprintf("%dd", days)
}
