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
		nodeID, cachedOrgID, found := appCtx.Cache.Resolve(args[0])
		if !found && !config.IsFullUUID(nodeID) {
			return fmt.Errorf("unknown id %q — pass a full UUID or run 'yertle tree' first to populate the local short-ID cache", args[0])
		}

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

		complete := canvasToComplete(canvas, nodeID)
		if complete == nil {
			return fmt.Errorf("node %s not found in canvas response", nodeID)
		}

		termWidth := getTerminalWidth()

		headerLines := renderNodeHeader(complete)
		connLines := renderConnectionsBlock(complete)
		canvasLines := renderCanvasLines(complete, false)

		// Left column of the top band = header + (if any) connections list.
		leftLines := headerLines
		if len(connLines) > 0 {
			leftLines = append(leftLines, connLines...)
		}

		// Collect per-card parts up front so we can compute a single global
		// left-column width spanning the header and every card, giving every
		// diagram in the output a common starting column.
		parentParts := collectCardParts(complete.ParentNodes, canvas)
		childParts := collectCardParts(complete.ChildNodes, canvas)

		globalLeftW := maxLineWidth(leftLines)
		for _, p := range parentParts {
			if w := maxLineWidth(p.left); w > globalLeftW {
				globalLeftW = w
			}
		}
		for _, p := range childParts {
			if w := maxLineWidth(p.left); w > globalLeftW {
				globalLeftW = w
			}
		}

		// Right column = diagram alone (shifted down 2 lines to align under
		// the title underline).
		var rightLines []string
		if len(canvasLines) > 0 {
			rightLines = append(rightLines, "", "")
			rightLines = append(rightLines, canvasLines...)
		}

		if len(rightLines) > 0 {
			top := renderSideBySide(padToWidth(leftLines, globalLeftW), rightLines, 4, termWidth)
			for _, line := range top {
				fmt.Println(line)
			}
		} else {
			for _, line := range leftLines {
				fmt.Println(line)
			}
		}

		// Bottom band: parents, child cards, ingress/egress.
		for _, line := range renderRelationships(complete, parentParts, childParts, globalLeftW) {
			fmt.Println(line)
		}
		return nil
	},
}

// canvasToComplete extracts the parent entry from a canvas response and
// builds a CompleteNode view for the existing parent-level renderers. Child
// details (tags, grandchildren, etc.) are read directly from the canvas map
// by renderChildCards, so they are not duplicated here.
func canvasToComplete(canvas api.CanvasResponse, nodeID string) *api.CompleteNode {
	parent, ok := canvas[nodeID]
	if !ok || parent == nil {
		return nil
	}

	children := resolveNodeRefs(parent.ChildNodes, canvas)
	parents := resolveNodeRefs(parent.ParentNodes, canvas)

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
		ParentNodes:      parents,
		VisualProperties: parent.VisualProperties,
		Connections:      parent.Connections,
		IngressConns:     parent.IngressConns,
		EgressConns:      parent.EgressConns,
		Metadata:         parent.Metadata,
	}
}

// resolveNodeRefs turns a list of node references (which may be bare UUIDs or
// {id,title,description} stubs) into NodeSummary values, falling back to the
// canvas map when a ref lacks title/description fields (the include_parents=
// full path always returns bare UUIDs, expecting the caller to look up full
// state from the top-level canvas entries).
func resolveNodeRefs(refs []api.ChildRef, canvas api.CanvasResponse) []api.NodeSummary {
	out := make([]api.NodeSummary, 0, len(refs))
	for _, ref := range refs {
		title := ref.Title
		desc := ref.Description
		if entry, ok := canvas[ref.ID]; ok && entry != nil {
			if entry.Title != "" {
				title = entry.Title
			}
			if entry.Description != "" {
				desc = entry.Description
			}
		}
		out = append(out, api.NodeSummary{
			ID:          ref.ID,
			Title:       title,
			Description: desc,
		})
	}
	return out
}

// renderNodeHeader emits the top-left panel: title, IDs, description, tags,
// and directories for the node itself. Relationships (parents/children/
// connections) are emitted separately by renderRelationships so the layout
// can place the header beside the diagram and run relationships full-width
// below.
const emptyValue = "—"

func renderNodeHeader(n *api.CompleteNode) []string {
	var lines []string

	lines = append(lines, n.Node.Title)
	lines = append(lines, strings.Repeat("─", sectionRuleWidth))

	desc := n.Node.Description
	if desc == "" {
		desc = emptyValue
	}
	lines = append(lines, fmt.Sprintf("  Description:  %s", desc))

	lines = append(lines, fmt.Sprintf("  Node ID:      %s", config.ShortID(n.Node.ID)))
	orgID := config.ShortID(n.Node.OrgID)
	if orgID == "" {
		orgID = emptyValue
	}
	lines = append(lines, fmt.Sprintf("  Org ID:       %s", orgID))

	lines = append(lines, "")
	if len(n.Tags) > 0 {
		lines = append(lines, "  Tags:")
		lines = append(lines, formatParentTagLines(n.Tags, "    - ")...)
	} else {
		lines = append(lines, fmt.Sprintf("  Tags:         %s", emptyValue))
	}

	if len(n.Directories) > 0 {
		lines = append(lines, "  Directories:")
		for _, d := range n.Directories {
			lines = append(lines, fmt.Sprintf("    - %s", d))
		}
	} else {
		lines = append(lines, fmt.Sprintf("  Directories:  %s", emptyValue))
	}

	return lines
}

// renderRelationships emits the bottom band: parents, one card per child,
// and ingress/egress to external nodes. Connections between children are
// rendered in the top band (next to the diagram) by renderConnectionsBlock.
// leftW is the global left-column width so every card's diagram starts at
// the same X column as the top-band diagram.
func renderRelationships(n *api.CompleteNode, parentParts, childParts []cardParts, leftW int) []string {
	var lines []string

	// Parents
	lines = append(lines, "")
	lines = append(lines, sectionHeader("PARENTS", len(n.ParentNodes), sectionRuleWidth)...)
	if len(parentParts) > 0 {
		lines = append(lines, renderCardsWithWidth(parentParts, leftW)...)
	} else {
		lines = append(lines, "  "+emptyValue)
	}

	// Children
	lines = append(lines, "")
	lines = append(lines, sectionHeader("CHILDREN", len(n.ChildNodes), sectionRuleWidth)...)
	if len(childParts) > 0 {
		lines = append(lines, renderCardsWithWidth(childParts, leftW)...)
	} else {
		lines = append(lines, "  "+emptyValue)
	}

	// Ingress
	lines = append(lines, "")
	lines = append(lines, sectionHeader("INGRESS", len(n.IngressConns), sectionRuleWidth)...)
	if len(n.IngressConns) > 0 {
		for _, conn := range n.IngressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.FromNodeID
			}
			if conn.Label != "" {
				lines = append(lines, fmt.Sprintf("  - %s → this (%s)", name, conn.Label))
			} else {
				lines = append(lines, fmt.Sprintf("  - %s → this", name))
			}
		}
	} else {
		lines = append(lines, "  "+emptyValue)
	}

	// Egress
	lines = append(lines, "")
	lines = append(lines, sectionHeader("EGRESS", len(n.EgressConns), sectionRuleWidth)...)
	if len(n.EgressConns) > 0 {
		for _, conn := range n.EgressConns {
			name := conn.ConnectedNode.Title
			if name == "" {
				name = conn.ToNodeID
			}
			if conn.Label != "" {
				lines = append(lines, fmt.Sprintf("  - this → %s (%s)", name, conn.Label))
			} else {
				lines = append(lines, fmt.Sprintf("  - this → %s", name))
			}
		}
	} else {
		lines = append(lines, "  "+emptyValue)
	}

	return lines
}

// renderConnectionsBlock returns a CONNECTIONS section (header + rule + list)
// for the left-hand data column beneath the parent header. Uses the same
// rule width as every other section so all separators are visually uniform.
func renderConnectionsBlock(n *api.CompleteNode) []string {
	if len(n.Connections) == 0 {
		return []string{fmt.Sprintf("  Connections:  %s", emptyValue)}
	}

	lines := []string{"  Connections:"}
	titles := make(map[string]string, len(n.ChildNodes))
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

// Layout constants for child cards.
const (
	cardHeaderIndent = "  - " // bullet for card header "- Title  shortID"
	cardBodyIndent   = "    " // description / section labels
	cardTagIndent    = "      " // tag rows, one level deeper than body
	cardSubIndent    = "    " // sub-diagram indent
	tagKeyColMin     = 8
	tagKeyColMax     = 20
)

// sectionRuleWidth is the width (in runes) of the horizontal rule under a
// section header in the bottom band. Capped so it doesn't stretch absurdly
// wide on large terminals.
const sectionRuleWidth = 60

// sectionHeader emits a flush-left uppercase section label followed by a
// full-width horizontal rule. Count appears in parentheses so scanners can
// pick it out at a glance.
func sectionHeader(label string, count, ruleWidth int) []string {
	return []string{
		fmt.Sprintf("%s (%d)", label, count),
		strings.Repeat("─", ruleWidth),
	}
}

// cardParts holds the pre-rendered left (text) and right (diagram) for one
// card. Keeping these split lets us compute a single global left-column
// width across every section before joining.
type cardParts struct {
	left, right []string
}

// collectCardParts builds a cardParts for each node without rendering them
// yet. Width computation happens at the command level.
func collectCardParts(nodes []api.NodeSummary, canvas api.CanvasResponse) []cardParts {
	parts := make([]cardParts, len(nodes))
	for i, n := range nodes {
		entry := canvas[n.ID]
		parts[i] = cardParts{
			left:  cardLeftLines(n, entry),
			right: cardRightLines(entry),
		}
	}
	return parts
}

// renderCardsWithWidth emits all cards with their left columns padded to
// leftW so every diagram starts at the same X. Cards are separated by a
// blank line.
func renderCardsWithWidth(parts []cardParts, leftW int) []string {
	const gutter = 4
	var out []string
	for i, p := range parts {
		if i > 0 {
			out = append(out, "")
		}
		rows := len(p.left)
		if len(p.right) > rows {
			rows = len(p.right)
		}
		for r := 0; r < rows; r++ {
			l := ""
			if r < len(p.left) {
				l = p.left[r]
			}
			right := ""
			if r < len(p.right) {
				right = p.right[r]
			}
			padded := l + strings.Repeat(" ", leftW-runeWidth(l))
			if right != "" {
				out = append(out, padded+strings.Repeat(" ", gutter)+right)
			} else {
				out = append(out, l)
			}
		}
	}
	return out
}

// padToWidth right-pads each line with spaces so it reaches w runes. Lines
// already ≥ w are left unchanged.
func padToWidth(lines []string, w int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		pad := w - runeWidth(l)
		if pad > 0 {
			out[i] = l + strings.Repeat(" ", pad)
		} else {
			out[i] = l
		}
	}
	return out
}

// cardLeftLines builds the text portion of one card: header, identity,
// description, directories, contains summary, and tags (last because tags
// are usually the longest section). `entry` may be nil if the canvas didn't
// return a standalone entry for this node.
func cardLeftLines(child api.NodeSummary, entry *api.CanvasEntry) []string {
	var lines []string

	lines = append(lines, fmt.Sprintf("%s%s", cardHeaderIndent, child.Title))
	lines = append(lines, fmt.Sprintf("%sNode ID:      %s", cardBodyIndent, config.ShortID(child.ID)))

	if entry == nil {
		return lines
	}

	desc := entry.Description
	if desc == "" {
		desc = emptyValue
	}
	lines = append(lines, fmt.Sprintf("%sDescription:  %s", cardBodyIndent, desc))

	if len(entry.Directories) > 0 {
		lines = append(lines, fmt.Sprintf("%sDirectories:", cardBodyIndent))
		for _, d := range entry.Directories {
			lines = append(lines, fmt.Sprintf("%s- %s", cardTagIndent, d))
		}
	} else {
		lines = append(lines, fmt.Sprintf("%sDirectories:  %s", cardBodyIndent, emptyValue))
	}

	gcCount := len(entry.ChildNodes)
	connCount := len(entry.Connections)
	lines = append(lines, fmt.Sprintf("%sContains:", cardBodyIndent))
	lines = append(lines, fmt.Sprintf("%s- %d %s", cardTagIndent, gcCount, pluralize("child", gcCount)))
	lines = append(lines, fmt.Sprintf("%s- %d %s", cardTagIndent, connCount, pluralize("connection", connCount)))

	if len(entry.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("%sTags:", cardBodyIndent))
		lines = append(lines, formatChildTagTable(entry.Tags, cardTagIndent)...)
	} else {
		lines = append(lines, fmt.Sprintf("%sTags:         %s", cardBodyIndent, emptyValue))
	}

	return lines
}

// cardRightLines returns the sub-diagram lines that sit to the right of
// the card text. Returns nil when the node has no drawable children.
func cardRightLines(entry *api.CanvasEntry) []string {
	if entry == nil {
		return nil
	}
	return buildSubDiagram(entry)
}

// buildSubDiagram synthesizes a minimal CompleteNode view of a child's
// internal structure and reuses the existing canvas renderer.
func buildSubDiagram(entry *api.CanvasEntry) []string {
	gcs := make([]api.NodeSummary, 0, len(entry.ChildNodes))
	for _, ref := range entry.ChildNodes {
		gcs = append(gcs, api.NodeSummary{
			ID:          ref.ID,
			Title:       ref.Title,
			Description: ref.Description,
		})
	}
	synthetic := &api.CompleteNode{
		ChildNodes:       gcs,
		VisualProperties: entry.VisualProperties,
		Connections:      entry.Connections,
	}
	return renderCanvasLines(synthetic, false)
}

// formatChildTagTable builds a 2-column "key value" table for one child's
// tags, indented by `indent`. Values are emitted in full so agents get the
// real ARNs. Tag `link` fields are intentionally NOT rendered here — they
// double every row visually and are available losslessly via --format json.
func formatChildTagTable(tags map[string]any, indent string) []string {
	if len(tags) == 0 {
		return nil
	}

	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	keyW := tagKeyColMin
	for _, k := range keys {
		if len(k) > keyW {
			keyW = len(k)
		}
	}
	if keyW > tagKeyColMax {
		keyW = tagKeyColMax
	}

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := tags[k]
		val := ""
		if t, ok := v.(map[string]any); ok {
			if value, ok := t["value"]; ok {
				val = fmt.Sprintf("%v", value)
			}
		} else {
			val = fmt.Sprintf("%v", v)
		}

		keyCell := k
		if len(keyCell) > keyW {
			keyCell = keyCell[:keyW]
		}
		keyCell = keyCell + strings.Repeat(" ", keyW-len(keyCell))

		out = append(out, fmt.Sprintf("%s- %s  %s", indent, keyCell, val))
	}
	return out
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	switch word {
	case "Child":
		return "Children"
	case "child":
		return "children"
	}
	return word + "s"
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
