package cmd

import (
	"fmt"
	"time"

	"github.com/albertcmiller1/flow/cli/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration and auth status",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)
		cfg := appCtx.Config

		fmt.Printf("Config file:  %s\n", config.DefaultConfigPath())
		fmt.Printf("API URL:      %s\n", cfg.APIURL)

		if !cfg.IsAuthenticated() {
			fmt.Println("User:         not logged in — run: yertle login")
			return nil
		}

		fmt.Printf("User:         %s\n", cfg.Auth.Email)
		if cfg.IsTokenExpired() {
			fmt.Println("Token:        expired — run: yertle login")
		} else {
			remaining := time.Until(cfg.Auth.ExpiresAt).Truncate(time.Minute)
			fmt.Printf("Token:        valid (expires in %s)\n", remaining)
		}
		return nil
	},
}
