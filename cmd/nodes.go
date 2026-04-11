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

var nodesCmd = &cobra.Command{
	Use:   "nodes [id]",
	Short: "List nodes, or show details for a specific node",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nodesShowCmd.RunE(cmd, args)
		}
		return nodesListCmd.RunE(cmd, args)
	},
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List nodes in an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		var nodes []api.Node
		var err error
		if appCtx.OrgID == "" || appCtx.OrgID == "all" {
			nodes, err = fetchAllNodes(appCtx.Client, "")
		} else {
			nodes, err = fetchAllNodes(appCtx.Client, appCtx.OrgID)
		}
		if err != nil {
			return fmt.Errorf("fetching nodes: %w", err)
		}

		// Populate cache with node IDs
		for _, n := range nodes {
			appCtx.Cache.Put(n.ID, n.OrgID, n.Title, "node")
		}
		_ = appCtx.Cache.Save() // non-critical

		columns := []output.Column{
			{Header: "TITLE", Value: func(r any) string { return r.(api.Node).Title }},
			{Header: "CHILDREN", Value: func(r any) string { return formatOptionalInt(r.(api.Node).NumChildren) }},
			{Header: "PARENTS", Value: func(r any) string { return formatOptionalInt(r.(api.Node).NumParents) }},
			{Header: "DESCENDANTS", Value: func(r any) string { return formatOptionalInt(r.(api.Node).NumDescendants) }},
			{Header: "ANCESTORS", Value: func(r any) string { return formatOptionalInt(r.(api.Node).NumAncestors) }},
			{Header: "ORG", Value: func(r any) string { return config.ShortID(r.(api.Node).OrgID) }},
			{Header: "ID", Value: func(r any) string { return config.ShortID(r.(api.Node).ID) }},
		}

		rows := make([]any, len(nodes))
		for i := range nodes {
			rows[i] = nodes[i]
		}

		switch appCtx.Format {
		case "json":
			return output.RenderJSON(os.Stdout, nodes)
		case "csv":
			output.RenderCSV(os.Stdout, columns, rows)
		default:
			output.RenderTable(os.Stdout, columns, rows, appCtx.NoColor)
		}

		return nil
	},
}

var nodesShowCmd = &cobra.Command{
	Use:   "show <node-id>",
	Short: "Show detailed information about a node",
	Long:  "Display a node's full details including children, parents, connections, tags, and directories.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		// Resolve node ID (and possibly org) through cache
		nodeID, cachedOrgID, _ := appCtx.Cache.Resolve(args[0])

		orgID := appCtx.OrgID
		if (orgID == "" || orgID == "all") && cachedOrgID != "" {
			orgID = cachedOrgID
		}
		if orgID == "" || orgID == "all" {
			return fmt.Errorf("organization required — use --org flag or run 'yertle tree' first to populate the cache")
		}

		complete, err := appCtx.Client.GetCompleteNode(orgID, nodeID, "main")
		if err != nil {
			return fmt.Errorf("fetching node: %w", err)
		}

		if appCtx.Format == "json" {
			return output.RenderJSON(os.Stdout, complete)
		}

		printNodeDetails(complete)
		return nil
	},
}

func printNodeDetails(n *api.CompleteNode) {
	fmt.Printf("%s\n", n.Node.Title)
	fmt.Println(strings.Repeat("─", 40))

	if n.Node.Description != "" {
		fmt.Printf("  Description:  %s\n", n.Node.Description)
	}

	fmt.Printf("  Node ID:      %s\n", config.ShortID(n.Node.ID))
	if n.Node.OrgID != "" {
		fmt.Printf("  Org ID:       %s\n", config.ShortID(n.Node.OrgID))
	}

	// Tags
	if len(n.Tags) > 0 {
		fmt.Println()
		fmt.Println("  Tags:")
		type tagPair struct{ key, value, link string }
		var tags []tagPair
		for k, v := range n.Tags {
			val := ""
			link := ""
			switch t := v.(type) {
			case map[string]any:
				if value, ok := t["value"]; ok {
					val = fmt.Sprintf("%v", value)
				}
				if l, ok := t["link"]; ok && l != nil {
					link = fmt.Sprintf("%v", l)
				}
			default:
				val = fmt.Sprintf("%v", v)
			}
			tags = append(tags, tagPair{k, val, link})
		}
		sort.Slice(tags, func(i, j int) bool { return tags[i].key < tags[j].key })
		for _, t := range tags {
			if t.value != "" && t.link != "" {
				fmt.Printf("    - %s: %s - %s\n", t.key, t.value, t.link)
			} else if t.value != "" {
				fmt.Printf("    - %s: %s\n", t.key, t.value)
			} else {
				fmt.Printf("    - %s\n", t.key)
			}
		}
	}

	// Directories
	if len(n.Directories) > 0 {
		fmt.Println()
		fmt.Println("  Directories:")
		for _, d := range n.Directories {
			fmt.Printf("    - %s\n", d)
		}
	}

	// Parents
	fmt.Println()
	if len(n.ParentNodes) > 0 {
		fmt.Printf("  %d %s:\n", len(n.ParentNodes), pluralize("Parent", len(n.ParentNodes)))
		printAlignedIDs(n.ParentNodes)
	} else {
		fmt.Println("  0 Parents")
	}

	// Children
	fmt.Println()
	if len(n.ChildNodes) > 0 {
		fmt.Printf("  %d %s:\n", len(n.ChildNodes), pluralize("Child", len(n.ChildNodes)))
		printAlignedIDs(n.ChildNodes)
	} else {
		fmt.Println("  0 Children")
	}

	// Internal connections (between children)
	fmt.Println()
	if len(n.Connections) > 0 {
		fmt.Printf("  %d %s:\n", len(n.Connections), pluralize("Connection", len(n.Connections)))
		// Build a title lookup from children
		titles := make(map[string]string)
		for _, c := range n.ChildNodes {
			titles[c.ID] = c.Title
		}
		for _, conn := range n.Connections {
			from := titles[conn.FromChild]
			to := titles[conn.ToChild]
			if from == "" {
				from = conn.FromChild
			}
			if to == "" {
				to = conn.ToChild
			}
			if conn.Label != "" {
				fmt.Printf("    - %s → %s (%s)\n", from, to, conn.Label)
			} else {
				fmt.Printf("    - %s → %s\n", from, to)
			}
		}
	} else {
		fmt.Println("  0 Connections")
	}

	// Ingress connections
	if len(n.IngressConns) > 0 {
		fmt.Println()
		fmt.Printf("  %d Ingress:\n", len(n.IngressConns))
		for _, conn := range n.IngressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.FromNodeID
			}
			if conn.Label != "" {
				fmt.Printf("    - %s → this (%s)\n", name, conn.Label)
			} else {
				fmt.Printf("    - %s → this\n", name)
			}
		}
	}

	// Egress connections
	if len(n.EgressConns) > 0 {
		fmt.Println()
		fmt.Printf("  %d Egress:\n", len(n.EgressConns))
		for _, conn := range n.EgressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.ToNodeID
			}
			if conn.Label != "" {
				fmt.Printf("    - this → %s (%s)\n", name, conn.Label)
			} else {
				fmt.Printf("    - this → %s\n", name)
			}
		}
	}
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	if word == "Child" {
		return "Children"
	}
	return word + "s"
}

func printAlignedIDs(nodes []api.NodeSummary) {
	maxW := 0
	for _, n := range nodes {
		if len(n.Title) > maxW {
			maxW = len(n.Title)
		}
	}
	for _, n := range nodes {
		padding := maxW - len(n.Title) + 2
		fmt.Printf("    - %s%s%s\n", n.Title, strings.Repeat(" ", padding), config.ShortID(n.ID))
	}
}

// fetchAllNodes fetches all nodes with pagination. If orgID is empty, fetches across all orgs.
func fetchAllNodes(client *api.Client, orgID string) ([]api.Node, error) {
	var all []api.Node
	limit, offset := 50, 0
	for {
		var nodes []api.Node
		var total int
		var err error
		if orgID == "" {
			nodes, total, err = client.GetAllNodes(limit, offset)
		} else {
			nodes, total, err = client.GetNodes(orgID, limit, offset)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, nodes...)
		if offset+limit >= total {
			break
		}
		offset += limit
	}
	return all, nil
}

func init() {
	nodesCmd.AddCommand(nodesListCmd)
	nodesCmd.AddCommand(nodesShowCmd)
}
