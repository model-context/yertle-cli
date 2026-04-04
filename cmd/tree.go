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

var treeCmd = &cobra.Command{
	Use:   "tree [org-id]",
	Short: "Display the containment hierarchy",
	Long:  "Show the parent/child ownership structure of all nodes as a tree. Defaults to all orgs if no org is specified.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx := GetAppContext(cmd)

		if err := ensureAuth(appCtx); err != nil {
			return err
		}

		// Determine org: positional arg > --org flag > all orgs
		orgID := appCtx.OrgID
		if len(args) == 1 {
			resolved, _, _ := appCtx.Cache.Resolve(args[0])
			orgID = resolved
		}

		var entries []api.HierarchyEntry
		var err error

		if orgID != "" && orgID != "all" {
			entries, err = appCtx.Client.GetHierarchy(orgID)
		} else {
			entries, err = appCtx.Client.GetAllHierarchy()
		}
		if err != nil {
			return fmt.Errorf("fetching hierarchy: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No nodes found.")
			return nil
		}

		// Populate cache with all node and org IDs
		for _, e := range entries {
			appCtx.Cache.Put(e.NodeID, e.OrgID, e.Title, "node")
			if e.OrgID != "" {
				appCtx.Cache.Put(e.OrgID, "", e.OrgName, "org")
			}
		}
		_ = appCtx.Cache.Save() // non-critical

		if appCtx.Format == "json" {
			return output.RenderJSON(os.Stdout, entries)
		}

		// Group entries by org
		orgEntries := groupByOrg(entries)

		multiOrg := len(orgEntries) > 1
		for i, og := range orgEntries {
			if multiOrg {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("%s (%s)\n", og.name, config.ShortID(og.orgID))
				printTree(og.entries, "  ")
			} else {
				printTree(og.entries, "")
			}
		}

		return nil
	},
}

type orgGroup struct {
	name    string
	orgID   string
	entries []api.HierarchyEntry
}

func groupByOrg(entries []api.HierarchyEntry) []orgGroup {
	orderMap := make(map[string]int)
	grouped := make(map[string]*orgGroup)

	for _, e := range entries {
		key := e.OrgID
		if key == "" {
			key = "unknown"
		}
		if _, exists := grouped[key]; !exists {
			name := e.OrgName
			if name == "" {
				name = key
			}
			grouped[key] = &orgGroup{name: name, orgID: key}
			orderMap[key] = len(orderMap)
		}
		grouped[key].entries = append(grouped[key].entries, e)
	}

	result := make([]orgGroup, 0, len(grouped))
	for _, g := range grouped {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

type nodeInfo struct {
	title       string
	nodeID      string
	fullPath    string
	isDirectory bool
}

type treeLine struct {
	prefix string // tree drawing chars + indent
	title  string
	id     string // short ID
}

// printTree builds a tree from hierarchy entries and prints with aligned short IDs.
func printTree(entries []api.HierarchyEntry, indent string) {
	childrenOf := make(map[string][]*nodeInfo)

	for _, e := range entries {
		parentPath := e.Path
		if parentPath == "" {
			parentPath = "/"
		}

		var fullPath string
		if parentPath == "/" {
			fullPath = "/" + sanitizeTitle(e.Title)
		} else {
			fullPath = parentPath + "/" + sanitizeTitle(e.Title)
		}

		info := &nodeInfo{
			title:       e.Title,
			nodeID:      e.NodeID,
			fullPath:    fullPath,
			isDirectory: e.IsDirectory,
		}

		childrenOf[parentPath] = append(childrenOf[parentPath], info)
	}

	for _, children := range childrenOf {
		sort.Slice(children, func(i, j int) bool {
			return children[i].title < children[j].title
		})
	}

	// Pass 1: collect all lines
	var lines []treeLine
	roots := childrenOf["/"]
	for _, root := range roots {
		lines = append(lines, treeLine{prefix: indent, title: root.title, id: config.ShortID(root.nodeID)})
		if root.isDirectory {
			collectChildren(childrenOf, root.fullPath, indent, &lines)
		}
	}

	// Pass 2: find max visual width (prefix + title)
	maxWidth := 0
	for _, l := range lines {
		w := runeWidth(l.prefix) + runeWidth(l.title)
		if w > maxWidth {
			maxWidth = w
		}
	}

	// Pass 3: print with aligned IDs
	for _, l := range lines {
		w := runeWidth(l.prefix) + runeWidth(l.title)
		padding := maxWidth - w + 2
		if padding < 2 {
			padding = 2
		}
		fmt.Printf("%s%s%s%s\n", l.prefix, l.title, strings.Repeat(" ", padding), l.id)
	}
}

func collectChildren(childrenOf map[string][]*nodeInfo, parentPath string, baseIndent string, lines *[]treeLine) {
	children := childrenOf[parentPath]
	for i, child := range children {
		isLast := i == len(children)-1

		connector := "├── "
		if isLast {
			connector = "└── "
		}

		*lines = append(*lines, treeLine{
			prefix: baseIndent + connector,
			title:  child.title,
			id:     config.ShortID(child.nodeID),
		})

		if child.isDirectory {
			childIndent := baseIndent + "│   "
			if isLast {
				childIndent = baseIndent + "    "
			}
			collectChildren(childrenOf, child.fullPath, childIndent, lines)
		}
	}
}

// runeWidth returns the display width of a string, counting each rune as 1.
func runeWidth(s string) int {
	return len([]rune(s))
}

func sanitizeTitle(title string) string {
	return strings.ReplaceAll(title, "/", "-")
}
