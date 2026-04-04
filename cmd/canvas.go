package cmd

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
	"github.com/albertcmiller1/flow/cli/output"
	"github.com/spf13/cobra"
)

var canvasCmd = &cobra.Command{
	Use:   "canvas <node-id>",
	Short: "Render an ASCII architecture diagram of a node's children",
	Long:  "Display a node's child components and their connections as a box-and-arrow diagram in the terminal.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

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

		if len(complete.ChildNodes) == 0 {
			fmt.Printf("%s has no children to display.\n", complete.Node.Title)
			return nil
		}

		renderCanvas(complete)
		return nil
	},
}

// --- Canvas rendering ---

type connInfo struct {
	fromID, toID string
	label        string
}

const (
	boxPadding  = 2  // spaces inside box on each side of title
	boxHeight   = 3  // top border + title + bottom border
	arrowStr    = "───→"
	arrowLen    = 4
	minBoxWidth = 12
)

// canvasNode holds a child node with its grid position.
type canvasNode struct {
	id    string
	title string
	rawX  int // original position_x
	rawY  int // original position_y
	gridR int // quantized grid row
	gridC int // quantized grid column
	boxW  int // rendered box width (chars)
}

func renderCanvas(complete *api.CompleteNode) {
	fmt.Printf("%s\n", complete.Node.Title)
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	// Build position map from visual properties
	posMap := make(map[string]api.VisualProperty)
	for _, vp := range complete.VisualProperties {
		posMap[vp.ChildNodeID] = vp
	}

	// Build title map
	titleMap := make(map[string]string)
	for _, c := range complete.ChildNodes {
		titleMap[c.ID] = c.Title
	}

	// Create canvas nodes with raw positions
	var nodes []*canvasNode
	for _, child := range complete.ChildNodes {
		vp, hasPos := posMap[child.ID]
		if !hasPos {
			continue
		}
		boxW := len(child.Title) + boxPadding*2 + 2
		if boxW < minBoxWidth {
			boxW = minBoxWidth
		}
		nodes = append(nodes, &canvasNode{
			id:    child.ID,
			title: child.Title,
			rawX:  int(vp.PositionX),
			rawY:  int(vp.PositionY),
			boxW:  boxW,
		})
	}

	if len(nodes) == 0 {
		fmt.Println("  (no positioned nodes)")
		return
	}

	// Quantize positions into grid rows and columns
	gridRows, numCols := quantizeGrid(nodes)

	// Compute column widths (max box width in each column)
	colWidths := make([]int, numCols)
	for _, n := range nodes {
		if n.boxW > colWidths[n.gridC] {
			colWidths[n.gridC] = n.boxW
		}
	}

	// Build connections
	var conns []connInfo
	for _, c := range complete.Connections {
		conns = append(conns, connInfo{fromID: c.FromChild, toID: c.ToChild, label: c.Label})
	}

	// Build node grid position lookup
	nodeGridR := make(map[string]int)
	nodeGridC := make(map[string]int)
	for _, n := range nodes {
		nodeGridR[n.id] = n.gridR
		nodeGridC[n.id] = n.gridC
	}

	// Build grid: grid[row][col] = *canvasNode or nil
	grid := make([][]*canvasNode, len(gridRows))
	for r, row := range gridRows {
		grid[r] = make([]*canvasNode, numCols)
		for _, n := range row {
			grid[r][n.gridC] = n
		}
	}

	// Render each row
	for rowIdx := range grid {
		renderGridRow(grid[rowIdx], colWidths, conns, nodeGridR, nodeGridC)

		// Draw vertical arrows to next row
		if rowIdx < len(grid)-1 {
			vertConns := findVertConns(conns, grid[rowIdx], grid[rowIdx+1], nodeGridR)
			renderVertArrows(grid[rowIdx], colWidths, vertConns)
		}
	}

	// Print connections that couldn't be drawn visually
	var undrawn []connInfo
	for _, c := range conns {
		fromR, fromOk := nodeGridR[c.fromID]
		toR, toOk := nodeGridR[c.toID]
		if !fromOk || !toOk {
			continue
		}
		// Same row, directly adjacent columns (left to right) — drawn as horizontal arrow
		if fromR == toR {
			fromC := nodeGridC[c.fromID]
			toC := nodeGridC[c.toID]
			if toC-fromC == 1 {
				continue
			}
		}
		// Adjacent rows, same column — drawn as vertical arrow
		if toR-fromR == 1 {
			fromC := nodeGridC[c.fromID]
			toC := nodeGridC[c.toID]
			if fromC == toC {
				continue
			}
		}
		undrawn = append(undrawn, c)
	}
	if len(undrawn) > 0 {
		fmt.Println()
		fmt.Println("  Other connections:")
		for _, c := range undrawn {
			from := titleMap[c.fromID]
			to := titleMap[c.toID]
			if from == "" {
				from = config.ShortID(c.fromID)
			}
			if to == "" {
				to = config.ShortID(c.toID)
			}
			if c.label != "" {
				fmt.Printf("    - %s → %s (%s)\n", from, to, c.label)
			} else {
				fmt.Printf("    - %s → %s\n", from, to)
			}
		}
	}
}

// quantizeGrid assigns gridR and gridC to each node based on clustering raw positions.
// Returns rows (slices of nodes per row) and the total number of columns.
func quantizeGrid(nodes []*canvasNode) ([][]*canvasNode, int) {
	// Collect unique Y and X values, cluster nearby ones
	rowSlots := clusterValues(nodes, func(n *canvasNode) int { return n.rawY }, 80)
	colSlots := clusterValues(nodes, func(n *canvasNode) int { return n.rawX }, 150)

	// Assign grid positions
	for _, n := range nodes {
		n.gridR = rowSlots[n.rawY]
		n.gridC = colSlots[n.rawX]
	}

	// Group into rows
	maxRow := 0
	maxCol := 0
	for _, n := range nodes {
		if n.gridR > maxRow {
			maxRow = n.gridR
		}
		if n.gridC > maxCol {
			maxCol = n.gridC
		}
	}

	rows := make([][]*canvasNode, maxRow+1)
	for _, n := range nodes {
		rows[n.gridR] = append(rows[n.gridR], n)
	}

	return rows, maxCol + 1
}

// clusterValues maps raw coordinate values to grid indices by clustering nearby values.
func clusterValues(nodes []*canvasNode, getVal func(*canvasNode) int, threshold int) map[int]int {
	// Collect unique values
	valSet := make(map[int]bool)
	for _, n := range nodes {
		valSet[getVal(n)] = true
	}

	vals := make([]int, 0, len(valSet))
	for v := range valSet {
		vals = append(vals, v)
	}
	sort.Ints(vals)

	// Cluster: values within threshold of each other get the same slot
	slotMap := make(map[int]int) // raw value → grid index
	slot := 0
	for i, v := range vals {
		if i > 0 && int(math.Abs(float64(v-vals[i-1]))) > threshold {
			slot++
		}
		slotMap[v] = slot
	}

	return slotMap
}

// renderGridRow draws a row of the grid, with empty slots where no node exists.
func renderGridRow(row []*canvasNode, colWidths []int, conns []connInfo, nodeGridR, nodeGridC map[string]int) {
	numCols := len(colWidths)

	// Determine which gaps have horizontal arrows.
	// Only draw when from is directly left of to (fromC + 1 == toC) — simple, straight-line connections.
	hasArrow := make(map[int]bool) // gap after column i

	for _, c := range conns {
		fromR, fromOk := nodeGridR[c.fromID]
		toR, toOk := nodeGridR[c.toID]
		if !fromOk || !toOk || fromR != toR {
			continue
		}
		fromC := nodeGridC[c.fromID]
		toC := nodeGridC[c.toID]
		// Only draw left-to-right, directly adjacent columns
		if toC-fromC != 1 {
			continue
		}
		// Check from node is in this row
		if row[fromC] == nil || row[fromC].id != c.fromID {
			continue
		}
		hasArrow[fromC] = true
	}

	// Line 1: top borders
	var line1 strings.Builder
	for c := 0; c < numCols; c++ {
		if c > 0 {
			line1.WriteString(strings.Repeat(" ", arrowLen))
		}
		n := row[c]
		if n != nil {
			line1.WriteString("┌")
			line1.WriteString(strings.Repeat("─", colWidths[c]-2))
			line1.WriteString("┐")
		} else {
			line1.WriteString(strings.Repeat(" ", colWidths[c]))
		}
	}
	fmt.Println(line1.String())

	// Line 2: titles with arrows
	var line2 strings.Builder
	for c := 0; c < numCols; c++ {
		if c > 0 {
			if hasArrow[c-1] {
				line2.WriteString(arrowStr)
			} else {
				line2.WriteString(strings.Repeat(" ", arrowLen))
			}
		}
		n := row[c]
		if n != nil {
			w := colWidths[c]
			padding := w - 2 - len(n.title)
			if padding < 0 {
				padding = 0
			}
			leftPad := padding / 2
			rightPad := padding - leftPad
			line2.WriteString("│")
			line2.WriteString(strings.Repeat(" ", leftPad))
			title := n.title
			if len(title) > w-2 {
				title = title[:w-5] + "..."
			}
			line2.WriteString(title)
			line2.WriteString(strings.Repeat(" ", rightPad))
			line2.WriteString("│")
		} else {
			if hasArrow[c-1] && c < numCols-1 && hasArrow[c] {
				// Empty cell in the middle of an arrow path — draw continuation
				line2.WriteString(strings.Repeat("─", colWidths[c]))
			} else {
				line2.WriteString(strings.Repeat(" ", colWidths[c]))
			}
		}
	}
	fmt.Println(line2.String())

	// Line 3: bottom borders
	var line3 strings.Builder
	for c := 0; c < numCols; c++ {
		if c > 0 {
			line3.WriteString(strings.Repeat(" ", arrowLen))
		}
		n := row[c]
		if n != nil {
			line3.WriteString("└")
			line3.WriteString(strings.Repeat("─", colWidths[c]-2))
			line3.WriteString("┘")
		} else {
			line3.WriteString(strings.Repeat(" ", colWidths[c]))
		}
	}
	fmt.Println(line3.String())
}

// findVertConns returns connections from a node in topRow to a node in bottomRow
// that are in the same grid column (straight down).
func findVertConns(conns []connInfo, topRow, bottomRow []*canvasNode, nodeGridR map[string]int) []connInfo {
	// Build column lookup for each row
	topByCol := make(map[int]string) // col → nodeID
	for _, n := range topRow {
		if n != nil {
			topByCol[n.gridC] = n.id
		}
	}
	bottomByCol := make(map[int]string)
	for _, n := range bottomRow {
		if n != nil {
			bottomByCol[n.gridC] = n.id
		}
	}

	var result []connInfo
	for _, c := range conns {
		// Check from is in top row and to is in bottom row
		fromCol := -1
		for col, id := range topByCol {
			if id == c.fromID {
				fromCol = col
				break
			}
		}
		if fromCol == -1 {
			continue
		}
		// To must be in the same column in the bottom row
		if bottomByCol[fromCol] == c.toID {
			result = append(result, c)
		}
	}
	return result
}

// renderVertArrows draws vertical arrows between two rows.
func renderVertArrows(topRow []*canvasNode, colWidths []int, conns []connInfo) {
	if len(conns) == 0 {
		fmt.Println()
		return
	}

	// Find which columns have downward arrows
	fromCols := make(map[int]bool)
	for _, c := range conns {
		for _, n := range topRow {
			if n != nil && n.id == c.fromID {
				fromCols[n.gridC] = true
			}
		}
	}

	numCols := len(colWidths)

	// Draw two lines: │ then ↓
	for _, ch := range []string{"│", "↓"} {
		var line strings.Builder
		for c := 0; c < numCols; c++ {
			if c > 0 {
				line.WriteString(strings.Repeat(" ", arrowLen))
			}
			w := colWidths[c]
			if fromCols[c] {
				leftPad := w / 2
				rightPad := w - leftPad - 1
				line.WriteString(strings.Repeat(" ", leftPad))
				line.WriteString(ch)
				line.WriteString(strings.Repeat(" ", rightPad))
			} else {
				line.WriteString(strings.Repeat(" ", w))
			}
		}
		fmt.Println(line.String())
	}
}
