package cmd

import (
	"fmt"
	"os"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"
	"github.com/cwarden/urd/internal/ui"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea/v2"
)

var (
	cfgFile     string
	remindFiles []string
	useP2       bool
	p2File      string
	cfg         *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "urd",
	Short: "A terminal calendar application for the remind calendar system",
	Long: `Urd is a terminal calendar application providing a TUI frontend for
the remind calendar system (and the forthcoming p2 project management tool).`,
	RunE: runTUI,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringSliceVarP(&remindFiles, "file", "f", []string{}, "Remind file(s) to use (can be specified multiple times)")
	rootCmd.PersistentFlags().BoolVar(&useP2, "p2", false, "Include p2 tasks as calendar events")
	rootCmd.PersistentFlags().StringVar(&p2File, "p2-file", "tasks.rec", "Path to p2 tasks file")
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Initialize reminder source(s)
	var source remind.ReminderSource

	// Always start with remind client
	remindClient := remind.NewClient()
	remindClient.RemindPath = cfg.RemindCommand

	// Use command-line specified files if provided, otherwise use config files
	if len(remindFiles) > 0 {
		remindClient.SetFiles(remindFiles)
		// Also update the config so the UI has the correct files for editing
		cfg.RemindFiles = remindFiles
	} else {
		remindClient.SetFiles(cfg.RemindFiles)
	}

	// Test remind connection (only for remind client, not the interface)
	if err := remindClient.TestConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure 'remind' is installed and in your PATH\n")
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

	// Start TUI
	model := ui.NewModelWithRemind(cfg, source, remindClient)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
