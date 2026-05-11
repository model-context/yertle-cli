package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
	"github.com/albertcmiller1/flow/cli/output"
	"github.com/spf13/cobra"
)

var (
	ragTopK         int
	ragExpand       string
	ragScopeRoot    string
	ragTagFilters   []string
	ragDirPrefix    string
	ragIncludeText  bool
)

var ragCmd = &cobra.Command{
	Use:   "rag <query>",
	Short: "Find nodes most likely to match a natural-language query",
	Long: `Run a graph-aware retrieval query over your organization's nodes.

By default returns the ranked top-K matches only — pass --expand to also
include surrounding parents, children, and connections.

Examples:
  yertle rag "api gateway in the webapp"
  yertle rag "checkout payment" --top-k 10
  yertle rag "auth service" --expand standard
  yertle rag "production db" --tag environment=production --tag tier=critical`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)
		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		if appCtx.OrgID == "" || appCtx.OrgID == "all" {
			return fmt.Errorf("rag requires a specific organization — set --org <id> or YERTLE_ORG")
		}

		expandChanged := cmd.Flags().Changed("expand")
		if expandChanged {
			switch ragExpand {
			case "none", "shallow", "standard", "deep":
			default:
				return fmt.Errorf("--expand must be one of: none, shallow, standard, deep")
			}
		}

		var scope *api.RagScope
		if ragScopeRoot != "" || len(ragTagFilters) > 0 || ragDirPrefix != "" {
			scope = &api.RagScope{DirectoryPrefix: ragDirPrefix}

			if ragScopeRoot != "" {
				resolved := ragScopeRoot
				if fullID, _, found := appCtx.Cache.Resolve(ragScopeRoot); found {
					resolved = fullID
				}
				scope.RootNodeID = resolved
			}

			if len(ragTagFilters) > 0 {
				scope.TagFilters = make(map[string]string, len(ragTagFilters))
				for _, t := range ragTagFilters {
					k, v, ok := strings.Cut(t, "=")
					if !ok || k == "" {
						return fmt.Errorf("--tag must be key=value, got %q", t)
					}
					scope.TagFilters[k] = v
				}
			}
		}

		req := api.RagRequest{
			Query:          args[0],
			TopK:           ragTopK,
			Scope:          scope,
			IncludeRawText: ragIncludeText,
		}
		// Only forward expansion_depth when the user explicitly set
		// --expand. Otherwise let the server apply its own default; this
		// keeps the CLI compatible with backends that haven't shipped
		// the "none" value yet.
		if expandChanged {
			req.ExpansionDepth = ragExpand
		}

		resp, err := appCtx.Client.RagSearch(appCtx.OrgID, req)
		if err != nil {
			return fmt.Errorf("rag search: %w", err)
		}

		// Cache returned IDs for short-ID resolution in follow-up commands.
		for _, n := range resp.Nodes {
			appCtx.Cache.Put(n.NodeID, appCtx.OrgID, n.Title, "node")
		}
		_ = appCtx.Cache.Save()

		if appCtx.Format == "json" {
			return output.RenderJSON(os.Stdout, resp)
		}

		if len(resp.Matches) == 0 {
			fmt.Fprintf(os.Stderr, "No matches for %q\n", req.Query)
			return nil
		}

		// Build a node lookup so we can pull tags/path onto match rows.
		nodeByID := make(map[string]api.RagNode, len(resp.Nodes))
		for _, n := range resp.Nodes {
			nodeByID[n.NodeID] = n
		}

		matchColumns := []output.Column{
			{Header: "SCORE", Value: func(r any) string { return fmt.Sprintf("%.3f", r.(api.RagMatch).Score) }},
			{Header: "TITLE", Value: func(r any) string { return r.(api.RagMatch).Title }},
			{Header: "REASON", Value: func(r any) string { return r.(api.RagMatch).MatchReason }},
			{Header: "TAGS", Value: func(r any) string {
				n, ok := nodeByID[r.(api.RagMatch).NodeID]
				if !ok {
					return ""
				}
				return formatTagsCompact(n.Tags)
			}},
			{Header: "PATH", Value: func(r any) string {
				n, ok := nodeByID[r.(api.RagMatch).NodeID]
				if !ok || len(n.Path) == 0 {
					return ""
				}
				return strings.Join(n.Path, " / ")
			}},
			{Header: "ID", Value: func(r any) string { return config.ShortID(r.(api.RagMatch).NodeID) }},
		}
		matchRows := make([]any, len(resp.Matches))
		for i := range resp.Matches {
			matchRows[i] = resp.Matches[i]
		}

		if appCtx.Format == "csv" {
			output.RenderCSV(os.Stdout, matchColumns, matchRows)
			return nil
		}

		fmt.Println("MATCHES")
		output.RenderTable(os.Stdout, matchColumns, matchRows, appCtx.NoColor)

		if len(resp.Connections) > 0 {
			fmt.Println()
			fmt.Println("CONNECTIONS")
			connColumns := []output.Column{
				{Header: "FROM", Value: func(r any) string { return r.(api.RagConnection).FromTitle }},
				{Header: "LABEL", Value: func(r any) string {
					c := r.(api.RagConnection)
					if c.Label != "" {
						return c.Label
					}
					return c.ConnectionType
				}},
				{Header: "TO", Value: func(r any) string { return r.(api.RagConnection).ToTitle }},
			}
			connRows := make([]any, len(resp.Connections))
			for i := range resp.Connections {
				connRows[i] = resp.Connections[i]
			}
			output.RenderTable(os.Stdout, connColumns, connRows, appCtx.NoColor)
		}

		if ragIncludeText {
			fmt.Println()
			fmt.Println("TEXT CONTENT")
			for _, m := range resp.Matches {
				n, ok := nodeByID[m.NodeID]
				if !ok || n.TextContent == "" {
					continue
				}
				fmt.Printf("\n--- %s (%s) ---\n%s\n", n.Title, config.ShortID(n.NodeID), n.TextContent)
			}
		}

		return nil
	},
}

func formatTagsCompact(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + tags[k]
	}
	return strings.Join(parts, ",")
}

func init() {
	ragCmd.Flags().IntVarP(&ragTopK, "top-k", "k", 5, "Max matches to return")
	ragCmd.Flags().StringVar(&ragExpand, "expand", "none", "Graph expansion: none, shallow, standard, deep")
	ragCmd.Flags().StringVar(&ragScopeRoot, "scope-root", "", "Restrict search to this node's subtree (full or short ID)")
	ragCmd.Flags().StringArrayVar(&ragTagFilters, "tag", nil, "Pre-filter by tag (key=value, repeatable)")
	ragCmd.Flags().StringVar(&ragDirPrefix, "dir-prefix", "", "Restrict to nodes whose directory starts with this prefix")
	ragCmd.Flags().BoolVar(&ragIncludeText, "include-text", false, "Include each node's prose text_content (debug/eval)")
}
