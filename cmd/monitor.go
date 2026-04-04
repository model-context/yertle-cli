package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Live health dashboard for organization nodes",
	Long:  "Launch an interactive htop-like TUI showing real-time health status of nodes. (Coming in Phase 3)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Monitor view coming soon.")
		return nil
	},
}
