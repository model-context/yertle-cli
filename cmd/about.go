package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Learn what Yertle is and how to use this CLI",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`Yertle — A hierarchical context layer for software systems

Yertle gives engineers and AI agents a structured, navigable view of their
software systems. Think of it like a filesystem for your infrastructure:
organizations contain nodes, nodes contain child nodes, and connections
describe how they interact — forming a living map of your architecture.

The primary purpose is context. When debugging an incident or understanding
a system, Yertle provides the hierarchical view — what depends on what, how
services connect, and where things live — so you can navigate a complex
system the way you navigate a directory tree.

Data model:
  Organizations    Top-level containers (like a workspace or team)
  Nodes            Any component: a service, database, queue, deployment, etc.
  Containment      Nodes contain child nodes (parent/child ownership hierarchy)
  Connections      Labeled edges between nodes ("calls", "reads from", etc.)
  Tags             Key-value metadata on nodes (env:prod, team:backend, etc.)
  Directories      Organizational paths for browsing (/infra, /services, etc.)
  Branches         Git-like versioning — all changes are tracked with commits

Common workflows:

  Orient — see what exists:
    yertle orgs                      List your organizations
    yertle tree                      Browse the full node hierarchy
    yertle nodes                     List all nodes with metadata

  Inspect — understand a specific component:
    yertle nodes <id>                Full details: children, parents, connections, tags
    yertle canvas <id>               ASCII architecture diagram of a node's children

  Configure:
    yertle auth login                Authenticate with the Yertle API
    yertle config                    View current configuration

Short IDs:
  All commands display 8-character short IDs (e.g. d29d9d78) that you can
  use in place of full UUIDs. Run any list or tree command first to populate
  the local ID cache, then use short IDs everywhere.
`)
		return nil
	},
}
