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
	Short: "Show node details and architecture diagram",
	Long:  "Display a node's full details including children, parents, connections, tags, and directories, with an ASCII architecture diagram shown alongside when children exist.",
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

		detailLines := renderNodeDetails(complete)

		// If node has children, render canvas alongside details
		canvasLines := renderCanvasLines(complete, false)
		if len(canvasLines) > 0 {
			// Shift canvas down so it starts below the title/separator
			padded := make([]string, 0, 2+len(canvasLines))
			padded = append(padded, "", "")
			padded = append(padded, canvasLines...)
			canvasLines = padded

			termWidth := getTerminalWidth()
			combined := renderSideBySide(detailLines, canvasLines, 4, termWidth)
			for _, line := range combined {
				fmt.Println(line)
			}
		} else {
			for _, line := range detailLines {
				fmt.Println(line)
			}
		}
		return nil
	},
}

func renderNodeDetails(n *api.CompleteNode) []string {
	var lines []string

	lines = append(lines, n.Node.Title)
	lines = append(lines, strings.Repeat("─", 40))

	if n.Node.Description != "" {
		lines = append(lines, fmt.Sprintf("  Description:  %s", n.Node.Description))
	}

	lines = append(lines, fmt.Sprintf("  Node ID:      %s", config.ShortID(n.Node.ID)))
	if n.Node.OrgID != "" {
		lines = append(lines, fmt.Sprintf("  Org ID:       %s", config.ShortID(n.Node.OrgID)))
	}

	// Tags
	if len(n.Tags) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Tags:")
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
				lines = append(lines, fmt.Sprintf("    - %s: %s - %s", t.key, t.value, t.link))
			} else if t.value != "" {
				lines = append(lines, fmt.Sprintf("    - %s: %s", t.key, t.value))
			} else {
				lines = append(lines, fmt.Sprintf("    - %s", t.key))
			}
		}
	}

	// Directories
	if len(n.Directories) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Directories:")
		for _, d := range n.Directories {
			lines = append(lines, fmt.Sprintf("    - %s", d))
		}
	}

	// Parents
	lines = append(lines, "")
	if len(n.ParentNodes) > 0 {
		lines = append(lines, fmt.Sprintf("  %d %s:", len(n.ParentNodes), pluralize("Parent", len(n.ParentNodes))))
		lines = append(lines, renderAlignedIDs(n.ParentNodes)...)
	} else {
		lines = append(lines, "  0 Parents")
	}

	// Children
	lines = append(lines, "")
	if len(n.ChildNodes) > 0 {
		lines = append(lines, fmt.Sprintf("  %d %s:", len(n.ChildNodes), pluralize("Child", len(n.ChildNodes))))
		lines = append(lines, renderAlignedIDs(n.ChildNodes)...)
	} else {
		lines = append(lines, "  0 Children")
	}

	// Internal connections (between children)
	lines = append(lines, "")
	if len(n.Connections) > 0 {
		lines = append(lines, fmt.Sprintf("  %d %s:", len(n.Connections), pluralize("Connection", len(n.Connections))))
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
				lines = append(lines, fmt.Sprintf("    - %s → %s (%s)", from, to, conn.Label))
			} else {
				lines = append(lines, fmt.Sprintf("    - %s → %s", from, to))
			}
		}
	} else {
		lines = append(lines, "  0 Connections")
	}

	// Ingress connections
	if len(n.IngressConns) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %d Ingress:", len(n.IngressConns)))
		for _, conn := range n.IngressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.FromNodeID
			}
			if conn.Label != "" {
				lines = append(lines, fmt.Sprintf("    - %s → this (%s)", name, conn.Label))
			} else {
				lines = append(lines, fmt.Sprintf("    - %s → this", name))
			}
		}
	}

	// Egress connections
	if len(n.EgressConns) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %d Egress:", len(n.EgressConns)))
		for _, conn := range n.EgressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.ToNodeID
			}
			if conn.Label != "" {
				lines = append(lines, fmt.Sprintf("    - this → %s (%s)", name, conn.Label))
			} else {
				lines = append(lines, fmt.Sprintf("    - this → %s", name))
			}
		}
	}

	return lines
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

func renderAlignedIDs(nodes []api.NodeSummary) []string {
	var lines []string
	maxW := 0
	for _, n := range nodes {
		if len(n.Title) > maxW {
			maxW = len(n.Title)
		}
	}
	for _, n := range nodes {
		padding := maxW - len(n.Title) + 2
		lines = append(lines, fmt.Sprintf("    - %s%s%s", n.Title, strings.Repeat(" ", padding), config.ShortID(n.ID)))
	}
	return lines
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
