//go:build !p2

package cmd

import (
	"github.com/cwarden/urd/internal/remind"
	"github.com/spf13/cobra"
)

func registerP2Flags(cmd *cobra.Command) {}

func buildReminderSource(remindClient *remind.Client) (remind.ReminderSource, error) {
	return remindClient, nil
}
