package cmd

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/albertcmiller1/flow/cli/api"
	"github.com/albertcmiller1/flow/cli/config"
)

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

// renderCanvasLines builds an ASCII box-and-arrow diagram of a node's children.
// If includeFooter is true, connections that couldn't be drawn visually are listed
// at the bottom. Returns nil if there are no children or no positioned nodes.
func renderCanvasLines(complete *api.CompleteNode, includeFooter bool) []string {
	if len(complete.ChildNodes) == 0 {
		return nil
	}

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
		return nil
	}

	var lines []string

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
		lines = append(lines, renderGridRowLines(grid[rowIdx], colWidths, conns, nodeGridR, nodeGridC)...)

		// Draw vertical arrows to next row
		if rowIdx < len(grid)-1 {
			vertConns := findVertConns(conns, grid[rowIdx], grid[rowIdx+1], nodeGridR)
			lines = append(lines, renderVertArrowLines(grid[rowIdx], colWidths, vertConns)...)
		}
	}

	if !includeFooter {
		return lines
	}

	// Append connections that couldn't be drawn visually
	var undrawn []connInfo
	for _, c := range conns {
		fromR, fromOk := nodeGridR[c.fromID]
		toR, toOk := nodeGridR[c.toID]
		if !fromOk || !toOk {
			continue
		}
		if fromR == toR {
			fromC := nodeGridC[c.fromID]
			toC := nodeGridC[c.toID]
			if toC-fromC == 1 {
				continue
			}
		}
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
		lines = append(lines, "")
		lines = append(lines, "  Other connections:")
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
				lines = append(lines, fmt.Sprintf("    - %s → %s (%s)", from, to, c.label))
			} else {
				lines = append(lines, fmt.Sprintf("    - %s → %s", from, to))
			}
		}
	}

	return lines
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

// renderGridRowLines builds the three lines (top border, title, bottom border) for a grid row.
func renderGridRowLines(row []*canvasNode, colWidths []int, conns []connInfo, nodeGridR, nodeGridC map[string]int) []string {
	numCols := len(colWidths)

	// Determine which gaps have horizontal arrows.
	hasArrow := make(map[int]bool)

	for _, c := range conns {
		fromR, fromOk := nodeGridR[c.fromID]
		toR, toOk := nodeGridR[c.toID]
		if !fromOk || !toOk || fromR != toR {
			continue
		}
		fromC := nodeGridC[c.fromID]
		toC := nodeGridC[c.toID]
		if toC-fromC != 1 {
			continue
		}
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
				line2.WriteString(strings.Repeat("─", colWidths[c]))
			} else {
				line2.WriteString(strings.Repeat(" ", colWidths[c]))
			}
		}
	}

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

	return []string{line1.String(), line2.String(), line3.String()}
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

// renderVertArrowLines builds the vertical arrow lines between two rows.
func renderVertArrowLines(topRow []*canvasNode, colWidths []int, conns []connInfo) []string {
	if len(conns) == 0 {
		return []string{""}
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
	var lines []string

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
		lines = append(lines, line.String())
	}

	return lines
}
