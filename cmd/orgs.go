package cmd

import (
	"fmt"
	"os"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
	"github.com/albertcmiller1/flow/cli/output"
	"github.com/spf13/cobra"
)

var orgsCmd = &cobra.Command{
	Use:   "orgs [org-id]",
	Short: "List organizations, or show details for a specific org",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return orgsShowCmd.RunE(cmd, args)
		}
		return orgsListCmd.RunE(cmd, args)
	},
}

var orgsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations you belong to",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		orgs, err := appCtx.Client.GetOrganizations()
		if err != nil {
			return fmt.Errorf("fetching organizations: %w", err)
		}

		// Populate cache with org IDs
		for _, org := range orgs {
			appCtx.Cache.Put(org.ID, "", org.Name, "org")
		}
		_ = appCtx.Cache.Save() // non-critical

		columns := []output.Column{
			{Header: "NAME", Value: func(r any) string { return r.(api.Organization).Name }},
			{Header: "ROLE", Value: func(r any) string { return r.(api.Organization).Role }},
			{Header: "MEMBERS", Value: func(r any) string { return formatOptionalInt(r.(api.Organization).MemberCount) }},
			{Header: "NODES", Value: func(r any) string { return formatOptionalInt(r.(api.Organization).NodeCount) }},
			{Header: "ID", Value: func(r any) string { return config.ShortID(r.(api.Organization).ID) }},
		}

		rows := make([]any, len(orgs))
		for i := range orgs {
			rows[i] = orgs[i]
		}

		switch appCtx.Format {
		case "json":
			return output.RenderJSON(os.Stdout, orgs)
		case "csv":
			output.RenderCSV(os.Stdout, columns, rows)
		default:
			output.RenderTable(os.Stdout, columns, rows, appCtx.NoColor)
		}

		return nil
	},
}

var orgsShowCmd = &cobra.Command{
	Use:   "show <org-id>",
	Short: "Show detailed information about an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		orgID, _, found := appCtx.Cache.Resolve(args[0])
		if !found && !config.IsFullUUID(orgID) {
			return fmt.Errorf("unknown id %q — pass a full UUID or run 'yertle orgs' first to populate the local short-ID cache", args[0])
		}

		org, err := appCtx.Client.GetOrganization(orgID)
		if err != nil {
			return fmt.Errorf("fetching organization: %w", err)
		}

		if appCtx.Format == "json" {
			return output.RenderJSON(os.Stdout, org)
		}

		fmt.Printf("%s\n", org.Name)
		fmt.Println("────────────────────────────────────────")

		if org.Description != "" {
			fmt.Printf("  Description:  %s\n", org.Description)
		}

		fmt.Printf("  Org ID:       %s\n", config.ShortID(org.ID))
		fmt.Printf("  Role:         %s\n", org.Role)
		fmt.Printf("  Members:      %s\n", formatOptionalInt(org.MemberCount))
		fmt.Printf("  Nodes:        %s\n", formatOptionalInt(org.NodeCount))
		fmt.Printf("  Invite mode:  %s\n", org.InviteMode)

		if org.RootNodeID != "" {
			fmt.Printf("  Root node:    %s\n", config.ShortID(org.RootNodeID))
		}

		return nil
	},
}

func init() {
	orgsCmd.AddCommand(orgsListCmd)
	orgsCmd.AddCommand(orgsShowCmd)
}
