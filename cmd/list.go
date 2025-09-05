package cmd

import (
	"fmt"
	"time"

	"github.com/cwarden/urd/internal/remind"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List today's events and exit",
	Long:  `List all events for today in a simple text format and exit.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Ensure config is loaded
	if cfg == nil {
		initConfig()
	}

	// Initialize reminder source(s)
	var source remind.ReminderSource

	// Always start with remind client
	remindClient := remind.NewClient()
	remindClient.RemindPath = cfg.RemindCommand

	// Use command-line specified files if provided, otherwise use config files
	if len(remindFiles) > 0 {
		remindClient.SetFiles(remindFiles)
	} else {
		remindClient.SetFiles(cfg.RemindFiles)
	}

	// Test remind connection
	if err := remindClient.TestConnection(); err != nil {
		return fmt.Errorf("remind connection failed: %w", err)
	}

	// If p2 is requested, create a composite source
	if useP2 {
		p2Client := remind.NewP2Client()
		p2Client.SetFiles([]string{p2File})
		// Create composite source with both remind and p2
		source = remind.NewCompositeSource(remindClient, p2Client)
	} else {
		// Use remind client alone
		source = remindClient
	}

	// Get today's events - normalize to midnight for date comparison
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	events, err := source.GetEvents(today, today)
	if err != nil {
		return fmt.Errorf("error getting events: %w", err)
	}

	// Display events
	fmt.Printf("Events for %s:\n", time.Now().Format(cfg.DateFormat))
	if len(events) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	for _, event := range events {
		timeStr := "All day"
		if event.Time != nil {
			timeStr = event.Time.Format(cfg.TimeFormat)
		}

		priorityStr := ""
		switch event.Priority {
		case remind.PriorityHigh:
			priorityStr = "!!!"
		case remind.PriorityMedium:
			priorityStr = "!!"
		case remind.PriorityLow:
			priorityStr = "!"
		}

		fmt.Printf("  %s - %s%s\n", timeStr, event.Description, priorityStr)
		if len(event.Tags) > 0 {
			fmt.Printf("    Tags: %v\n", event.Tags)
		}
	}

	return nil
}
