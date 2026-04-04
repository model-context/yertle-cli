package cmd

import (
	"fmt"

	"github.com/albertcmiller1/flow/cli/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE:  configShowCmd.RunE,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)
		cfg := appCtx.Config

		fmt.Printf("Config file:  %s\n", config.DefaultConfigPath())
		fmt.Printf("API URL:      %s\n", cfg.APIURL)

		if cfg.IsAuthenticated() {
			fmt.Printf("Logged in as: %s\n", cfg.Auth.Email)
		} else {
			fmt.Println("Auth:         not logged in")
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
