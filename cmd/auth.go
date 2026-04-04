package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/albertcmiller1/flow/cli/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Show current authentication status",
	RunE:  authStatusCmd.RunE,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Yertle API",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

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

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if !appCtx.Config.IsAuthenticated() {
			fmt.Println("Not logged in. Run: yertle auth login")
			return nil
		}

		auth := appCtx.Config.Auth
		fmt.Printf("User:     %s\n", auth.Email)
		fmt.Printf("API:      %s\n", appCtx.Config.APIURL)

		if appCtx.Config.IsTokenExpired() {
			fmt.Println("Token:    expired")
		} else {
			remaining := time.Until(auth.ExpiresAt).Truncate(time.Minute)
			fmt.Printf("Token:    valid (expires in %s)\n", remaining)
		}

		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
}
