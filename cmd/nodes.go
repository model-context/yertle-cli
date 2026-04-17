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

		canvas, err := appCtx.Client.GetCanvasState(orgID, nodeID, "main")
		if err != nil {
			return fmt.Errorf("fetching node: %w", err)
		}

		if appCtx.Format == "json" {
			return output.RenderJSON(os.Stdout, canvas)
		}

		complete, childTags := canvasToComplete(canvas, nodeID)
		if complete == nil {
			return fmt.Errorf("node %s not found in canvas response", nodeID)
		}

		termWidth := getTerminalWidth()

		headerLines := renderNodeHeader(complete)
		canvasLines := renderCanvasLines(complete, false)
		connLines := renderConnectionsBlock(complete)

		// Right panel = diagram + connections list beneath it.
		var rightLines []string
		if len(canvasLines) > 0 {
			// Shift diagram down 2 lines so it sits below the title underline.
			rightLines = append(rightLines, "", "")
			rightLines = append(rightLines, canvasLines...)
			if len(connLines) > 0 {
				rightLines = append(rightLines, "")
				rightLines = append(rightLines, connLines...)
			}
		}

		if len(rightLines) > 0 {
			top := renderSideBySide(headerLines, rightLines, 4, termWidth)
			for _, line := range top {
				fmt.Println(line)
			}
		} else {
			for _, line := range headerLines {
				fmt.Println(line)
			}
		}

		// Bottom band: parents, children-with-tags, ingress/egress.
		for _, line := range renderRelationships(complete, childTags) {
			fmt.Println(line)
		}
		return nil
	},
}

// canvasToComplete extracts the parent entry from a canvas response and builds
// a CompleteNode view for the existing renderers. It also returns a map of
// child node ID → tags for rendering child tags inline.
func canvasToComplete(canvas api.CanvasResponse, nodeID string) (*api.CompleteNode, map[string]map[string]any) {
	parent, ok := canvas[nodeID]
	if !ok || parent == nil {
		return nil, nil
	}

	children := make([]api.NodeSummary, 0, len(parent.ChildNodes))
	childTags := make(map[string]map[string]any, len(parent.ChildNodes))
	for _, ref := range parent.ChildNodes {
		title := ref.Title
		desc := ref.Description
		if entry, ok := canvas[ref.ID]; ok && entry != nil {
			if entry.Title != "" {
				title = entry.Title
			}
			if entry.Description != "" {
				desc = entry.Description
			}
			if len(entry.Tags) > 0 {
				childTags[ref.ID] = entry.Tags
			}
		}
		children = append(children, api.NodeSummary{
			ID:          ref.ID,
			Title:       title,
			Description: desc,
		})
	}

	return &api.CompleteNode{
		Node: api.NodeDetail{
			ID:          parent.ID,
			Title:       parent.Title,
			Description: parent.Description,
			OrgID:       parent.OrgID,
		},
		Tags:             parent.Tags,
		Directories:      parent.Directories,
		ChildNodes:       children,
		ParentNodes:      parent.ParentNodes,
		VisualProperties: parent.VisualProperties,
		Connections:      parent.Connections,
		IngressConns:     parent.IngressConns,
		EgressConns:      parent.EgressConns,
		Metadata:         parent.Metadata,
	}, childTags
}

// renderNodeHeader emits the top-left panel: title, IDs, description, tags,
// and directories for the node itself. Relationships (parents/children/
// connections) are emitted separately by renderRelationships so the layout
// can place the header beside the diagram and run relationships full-width
// below.
func renderNodeHeader(n *api.CompleteNode) []string {
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

	if len(n.Tags) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Tags:")
		lines = append(lines, formatParentTagLines(n.Tags, "    - ")...)
	}

	if len(n.Directories) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Directories:")
		for _, d := range n.Directories {
			lines = append(lines, fmt.Sprintf("    - %s", d))
		}
	}

	return lines
}

// renderRelationships emits the bottom band: parents, children with tag
// tables, and ingress/egress to external nodes. Connections between children
// are rendered in the top band (next to the diagram) by renderConnectionsBlock.
func renderRelationships(n *api.CompleteNode, childTags map[string]map[string]any) []string {
	var lines []string

	// Parents
	lines = append(lines, "")
	if len(n.ParentNodes) > 0 {
		lines = append(lines, fmt.Sprintf("  %d %s:", len(n.ParentNodes), pluralize("Parent", len(n.ParentNodes))))
		lines = append(lines, renderAlignedIDs(n.ParentNodes)...)
	} else {
		lines = append(lines, "  0 Parents")
	}

	// Children (each child row followed by its tag table)
	lines = append(lines, "")
	if len(n.ChildNodes) > 0 {
		lines = append(lines, fmt.Sprintf("  %d %s:", len(n.ChildNodes), pluralize("Child", len(n.ChildNodes))))
		lines = append(lines, renderChildrenWithTags(n.ChildNodes, childTags)...)
	} else {
		lines = append(lines, "  0 Children")
	}

	// Ingress
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

	// Egress
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

// renderConnectionsBlock returns a compact list of child-to-child connections
// to display beneath the architecture diagram in the top band.
func renderConnectionsBlock(n *api.CompleteNode) []string {
	if len(n.Connections) == 0 {
		return nil
	}
	titles := make(map[string]string, len(n.ChildNodes))
	for _, c := range n.ChildNodes {
		titles[c.ID] = c.Title
	}
	lines := []string{fmt.Sprintf("%d %s:", len(n.Connections), pluralize("Connection", len(n.Connections)))}
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
			lines = append(lines, fmt.Sprintf("  - %s → %s (%s)", from, to, conn.Label))
		} else {
			lines = append(lines, fmt.Sprintf("  - %s → %s", from, to))
		}
	}
	return lines
}

// formatParentTagLines formats the parent node's tags for the header panel.
// Each tag takes one line: "<indent>key: value" or "<indent>key: value - link".
func formatParentTagLines(tags map[string]any, indent string) []string {
	type tagPair struct{ key, value, link string }
	var pairs []tagPair
	for k, v := range tags {
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
		pairs = append(pairs, tagPair{k, val, link})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].key < pairs[j].key })

	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		switch {
		case p.value != "" && p.link != "":
			out = append(out, fmt.Sprintf("%s%s: %s - %s", indent, p.key, p.value, p.link))
		case p.value != "":
			out = append(out, fmt.Sprintf("%s%s: %s", indent, p.key, p.value))
		default:
			out = append(out, fmt.Sprintf("%s%s", indent, p.key))
		}
	}
	return out
}

// Constants for the child tag table layout.
const (
	childRowIndent = "    - " // bullet for "- Title  shortID"
	tagRowIndent   = "      " // same indent as text after bullet
	tagKeyColMin   = 8
	tagKeyColMax   = 20
)

// renderChildrenWithTags renders each child as an aligned title/shortID row
// followed by a 2-column tag table (key, value). If a tag has a separate
// link, the link is emitted on the next indented line prefixed with "↳".
// All value and link cells are truncated to fit within termWidth so long
// ARNs and URLs don't spill the terminal.
func renderChildrenWithTags(nodes []api.NodeSummary, tagsByID map[string]map[string]any) []string {
	var lines []string

	// Align title → shortID column across all children.
	titleW := 0
	for _, n := range nodes {
		if len(n.Title) > titleW {
			titleW = len(n.Title)
		}
	}

	for i, n := range nodes {
		if i > 0 {
			lines = append(lines, "")
		}
		padding := titleW - len(n.Title) + 2
		lines = append(lines, fmt.Sprintf("%s%s%s%s", childRowIndent, n.Title, strings.Repeat(" ", padding), config.ShortID(n.ID)))
		lines = append(lines, formatChildTagTable(tagsByID[n.ID])...)
	}
	return lines
}

// formatChildTagTable builds a 2-column "key value" table for one child.
// Values are emitted in full — agents (the primary consumer) need the real
// ARNs and URLs for downstream tool calls. If a tag has a separate link, it
// appears on its own line beneath the value, aligned under the value column.
func formatChildTagTable(tags map[string]any) []string {
	if len(tags) == 0 {
		return nil
	}

	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Key column width = longest key among this child's tags, clamped.
	keyW := tagKeyColMin
	for _, k := range keys {
		if len(k) > keyW {
			keyW = len(k)
		}
	}
	if keyW > tagKeyColMax {
		keyW = tagKeyColMax
	}

	// Link sits one tab (4 spaces) further right than the value column, so
	// it reads as subordinate to the value above it.
	linkIndent := strings.Repeat(" ", len(tagRowIndent)+keyW+2+4)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := tags[k]
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

		keyCell := k
		if len(keyCell) > keyW {
			keyCell = keyCell[:keyW]
		}
		keyCell = keyCell + strings.Repeat(" ", keyW-len(keyCell))

		out = append(out, fmt.Sprintf("%s%s  %s", tagRowIndent, keyCell, val))
		if link != "" && link != val {
			out = append(out, fmt.Sprintf("%s↳ %s", linkIndent, link))
		}
	}
	return out
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
