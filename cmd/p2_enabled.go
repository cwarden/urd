//go:build p2

package cmd

import (
	"github.com/cwarden/urd/internal/remind"
	"github.com/spf13/cobra"
)

var (
	useP2  bool
	p2File string
)

func registerP2Flags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&useP2, "p2", false, "Include p2 tasks as calendar events")
	cmd.PersistentFlags().StringVar(&p2File, "p2-file", "tasks.rec", "Path to p2 tasks file")
}

func buildReminderSource(remindClient *remind.Client) (remind.ReminderSource, error) {
	if !useP2 {
		return remindClient, nil
	}

	p2Client := remind.NewP2Client()
	p2Client.SetFiles([]string{p2File})

	return remind.NewCompositeSource(remindClient, p2Client), nil
}
