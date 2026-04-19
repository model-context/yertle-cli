package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Yertle API",
	Long: `Authenticate with the Yertle API and persist the token to config.

By default, sign in against production (https://api.yertle.com).
Pass --api-url to point at a different backend (e.g. a local dev server
or staging). The URL you log into is bound to the stored token, so all
subsequent commands use that same backend until you log in again.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		// If --api-url is set, update config and rebuild the client so the
		// SignIn call hits the chosen backend. Also wipe the ID cache —
		// cached short-ID mappings are per-backend and become invalid (or
		// ambiguous) when the backend changes.
		flagAPIURL, _ := cmd.Flags().GetString("api-url")
		if flagAPIURL != "" && flagAPIURL != appCtx.Config.APIURL {
			appCtx.Config.APIURL = flagAPIURL
			appCtx.Config.Auth = nil // stale — was issued by a different backend
			appCtx.Client = api.NewClient(flagAPIURL, "")
			if err := config.ClearCache(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not clear ID cache: %v\n", err)
			}
			appCtx.Cache = config.LoadCache() // reset to empty in-memory cache
		}

		fmt.Printf("Signing in to %s\n", appCtx.Config.APIURL)

		// Prompt for email
		fmt.Print("Email: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		email := strings.TrimSpace(scanner.Text())
		if email == "" {
			return fmt.Errorf("email is required")
		}

		// Prompt for password (hidden input)
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		password := string(passwordBytes)
		if password == "" {
			return fmt.Errorf("password is required")
		}

		resp, err := appCtx.Client.SignIn(email, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		appCtx.Config.Auth = &config.AuthConfig{
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
			Email:        resp.User.Email,
		}

		if err := appCtx.Config.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Authenticated as %s\n", resp.User.Email)
		fmt.Printf("Token stored in %s\n", config.DefaultConfigPath())
		return nil
	},
}

func init() {
	loginCmd.Flags().String("api-url", "", "Override the API URL for this login (defaults to the current config value)")
}
