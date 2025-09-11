package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Urd",
	Long:  `All software has versions. This is Urd's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Urd %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
