package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
	"github.com/spf13/cobra"
)

type contextKey string

const appCtxKey contextKey = "appCtx"

// AppContext holds resolved configuration and clients for subcommands.
type AppContext struct {
	Config  *config.Config
	Cache   *config.IDCache
	Client  *api.Client
	Format  string
	NoColor bool
	OrgID   string
}

// GetAppContext extracts the AppContext from a cobra command's context.
func GetAppContext(cmd *cobra.Command) *AppContext {
	val := cmd.Context().Value(appCtxKey)
	if val == nil {
		panic("yertle: app context not initialized — this is a bug")
	}
	return val.(*AppContext)
}

// ensureAuth returns an error if the user is not authenticated.
func ensureAuth(appCtx *AppContext) error {
	if !appCtx.Config.IsAuthenticated() {
		return fmt.Errorf("not logged in — run: yertle login")
	}
	return nil
}

// formatOptionalInt formats an *int as a string, returning "-" if nil.
func formatOptionalInt(v *int) string {
	if v != nil {
		return strconv.Itoa(*v)
	}
	return "-"
}

var rootCmd = &cobra.Command{
	Use:   "yertle",
	Short: "Navigate and inspect your software systems",
	Long:  "Yertle CLI — a hierarchical context layer for software systems. Browse organizations, explore node hierarchies, and inspect components. Run 'yertle about' for a full overview.",
	// Usage is only noise when a command's RunE returns a runtime error
	// (wrong ID, API 4xx, etc.). Cobra still prints Usage for its own
	// argument/flag validation errors, which is where it helps.
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		flagOrg, _ := cmd.Flags().GetString("org")
		format, _ := cmd.Flags().GetString("format")
		noColor, _ := cmd.Flags().GetBool("no-color")

		resolvedAPI, resolvedOrg := cfg.Resolve(flagOrg)

		token := ""
		refreshToken := ""
		if cfg.Auth != nil {
			token = cfg.Auth.AccessToken
			refreshToken = cfg.Auth.RefreshToken
		}

		client := api.NewClient(resolvedAPI, token)

		// Set up auto-refresh: on 401, try refresh token and persist new tokens
		if refreshToken != "" {
			client.WithRefresh(refreshToken, func(newToken string, expiresIn int) {
				cfg.Auth.AccessToken = newToken
				cfg.Auth.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
				cfg.Save()
			})
		}

		cache := config.LoadCache()

		// Resolve org through cache if it's a short ID
		if resolvedOrg != "" && resolvedOrg != "all" {
			if fullID, _, found := cache.Resolve(resolvedOrg); found {
				resolvedOrg = fullID
			}
		}

		appCtx := &AppContext{
			Config:  cfg,
			Cache:   cache,
			Client:  client,
			Format:  format,
			NoColor: noColor,
			OrgID:   resolvedOrg,
		}

		cmd.SetContext(context.WithValue(cmd.Context(), appCtxKey, appCtx))
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringP("org", "o", "", "Scope to a specific organization (or set YERTLE_ORG)")
	rootCmd.PersistentFlags().StringP("format", "f", "table", "Output format: table, json, csv")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable terminal colors")

	// TODO: un-hide this when we're ready to advertise shell completion
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.AddCommand(aboutCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(orgsCmd)
	rootCmd.AddCommand(nodesCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(monitorCmd)
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}
