package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"
	"github.com/cwarden/urd/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Parse command line flags
	var (
		configFile = flag.String("config", "", "Path to config file")
		listEvents = flag.Bool("list", false, "List today's events and exit")
		version    = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Println("Urd 0.1.0")
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override config file if specified
	if *configFile != "" {
		// TODO: Load specific config file
	}

	// Initialize remind client
	client := remind.NewClient()
	client.RemindPath = cfg.RemindCommand
	client.SetFiles(cfg.RemindFiles)

	// Test remind connection
	if err := client.TestConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure 'remind' is installed and in your PATH\n")
	}

	// List mode
	if *listEvents {
		listTodayEvents(cfg, client)
		return
	}

	// Start TUI
	model := ui.NewModel(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func listTodayEvents(cfg *config.Config, client *remind.Client) {
	events, err := client.GetEventsForDate(time.Now())
	if err != nil {
		log.Printf("Error getting events: %v", err)
		return
	}

	fmt.Printf("Events for %s:\n", time.Now().Format(cfg.DateFormat))
	if len(events) == 0 {
		fmt.Println("No events found.")
		return
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
}
