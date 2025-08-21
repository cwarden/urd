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
		useP2      = flag.Bool("p2", false, "Include p2 tasks as calendar events")
		p2File     = flag.String("p2-file", "tasks.rec", "Path to p2 tasks file")
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

	// Initialize reminder source(s)
	var source remind.ReminderSource

	// Always start with remind client
	remindClient := remind.NewClient()
	remindClient.RemindPath = cfg.RemindCommand
	remindClient.SetFiles(cfg.RemindFiles)

	// Test remind connection (only for remind client, not the interface)
	if err := remindClient.TestConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure 'remind' is installed and in your PATH\n")
	}

	// If p2 is requested, create a composite source
	if *useP2 {
		p2Client := remind.NewP2Client()
		p2Client.SetFiles([]string{*p2File})
		// Create composite source with both remind and p2
		source = remind.NewCompositeSource(remindClient, p2Client)
	} else {
		// Use remind client alone
		source = remindClient
	}

	// List mode
	if *listEvents {
		listTodayEvents(cfg, source)
		return
	}

	// Start TUI
	// We need to create a version of NewModel that also takes the remind client
	model := ui.NewModelWithRemind(cfg, source, remindClient)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func listTodayEvents(cfg *config.Config, source remind.ReminderSource) {
	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)
	events, err := source.GetEvents(today, tomorrow)
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
