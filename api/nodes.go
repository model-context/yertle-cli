package api

import (
	"encoding/json"
	"fmt"
)

type Node struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	OrgID       string         `json:"org_id"`
	PublicID    string         `json:"public_id"`
	Tags        map[string]any `json:"tags"`
	NumParents     *int           `json:"num_parents"`
	NumChildren    *int           `json:"num_children"`
	NumDescendants *int           `json:"num_descendants"`
	NumAncestors   *int           `json:"num_ancestors"`
	CreatedAt      string         `json:"created_at"`
}

type NodeListResponse struct {
	Nodes  []Node `json:"nodes"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (c *Client) GetNodes(orgID string, limit, offset int) ([]Node, int, error) {
	path := fmt.Sprintf("/orgs/%s/nodes?limit=%d&offset=%d", orgID, limit, offset)
	var resp NodeListResponse
	if err := c.Get(path, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Nodes, resp.Total, nil
}

func (c *Client) GetAllNodes(limit, offset int) ([]Node, int, error) {
	path := fmt.Sprintf("/orgs/all/nodes?limit=%d&offset=%d", limit, offset)
	var resp NodeListResponse
	if err := c.Get(path, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Nodes, resp.Total, nil
}

type HierarchyEntry struct {
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Depth       int    `json:"depth"`
	IsDirectory bool   `json:"is_directory"`
	OrgID       string `json:"org_id,omitempty"`
	OrgName     string `json:"org_name,omitempty"`
}

type HierarchyResponse struct {
	Entries []HierarchyEntry `json:"entries"`
	Total   int              `json:"total"`
}

func (c *Client) GetHierarchy(orgID string) ([]HierarchyEntry, error) {
	path := fmt.Sprintf("/orgs/%s/hierarchy", orgID)
	var resp HierarchyResponse
	if err := c.Get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (c *Client) GetAllHierarchy() ([]HierarchyEntry, error) {
	var resp HierarchyResponse
	if err := c.Get("/orgs/all/hierarchy", &resp); err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

// Complete node types — response from /tree/{branch}/complete

type NodeDetail struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	OrgID       string `json:"org_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type NodeSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Connection struct {
	ID        string `json:"id"`
	FromChild string `json:"from_child_id"`
	ToChild   string `json:"to_child_id"`
	Label     string `json:"label"`
	Type      string `json:"type"`
}

type ExternalConn struct {
	ConnectionID  string      `json:"connection_id"`
	FromNodeID    string      `json:"from_node_id"`
	ToNodeID      string      `json:"to_node_id"`
	Label         string      `json:"label"`
	ConnectedNode NodeSummary `json:"connected_node"`
}

type VisualProperty struct {
	ChildNodeID string  `json:"child_node_id"`
	PositionX   float64 `json:"position_x"`
	PositionY   float64 `json:"position_y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
}

type CompleteNode struct {
	Node             NodeDetail       `json:"node"`
	Tags             map[string]any   `json:"tags"`
	Directories      []string         `json:"directories"`
	Documentation    []any            `json:"documentation"`
	ChildNodes       []NodeSummary    `json:"child_nodes"`
	ParentNodes      []NodeSummary    `json:"parent_nodes"`
	VisualProperties []VisualProperty `json:"visual_properties"`
	Connections      []Connection     `json:"connections"`
	IngressConns     []ExternalConn   `json:"ingress_connections"`
	EgressConns      []ExternalConn   `json:"egress_connections"`
	Metadata         map[string]any   `json:"metadata"`
}

func (c *Client) GetCompleteNode(orgID, nodeID, branch string) (*CompleteNode, error) {
	path := fmt.Sprintf("/orgs/%s/nodes/%s/tree/%s/complete", orgID, nodeID, branch)
	var resp CompleteNode
	if err := c.Get(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Canvas response — flat dict keyed by node ID. Each entry is a "push-compatible"
// flattened state for one node (parent + one level of children). The parent's
// `child_nodes` field is a list of ID strings; child entries carry the deeper
// `child_nodes` objects, but we don't use those here.

type CanvasEntry struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	OrgID            string           `json:"org_id"`
	Tags             map[string]any   `json:"tags"`
	Directories      []string         `json:"directories"`
	ChildNodes       []ChildRef       `json:"child_nodes"`
	VisualProperties []VisualProperty `json:"visual_properties"`
	Connections      []Connection     `json:"connections"`
	// ParentNodes accepts either a list of {id,title,description} stubs (slim
	// mode, the API default) or a list of UUID strings (include_parents=full,
	// where each parent also appears as a top-level entry in the response map).
	ParentNodes []ChildRef `json:"parent_nodes"`
	IngressConns     []ExternalConn   `json:"ingress_connections"`
	EgressConns      []ExternalConn   `json:"egress_connections"`
	Metadata         map[string]any   `json:"metadata"`
}

// ChildRef accepts either a string ID (parent entry) or a {id,title,...}
// object (child entries).
type ChildRef struct {
	ID          string
	Title       string
	Description string
}

func (c *ChildRef) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.ID = s
		return nil
	}
	var obj struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	c.ID = obj.ID
	c.Title = obj.Title
	c.Description = obj.Description
	return nil
}

// CanvasResponse is keyed by node ID; it also contains a `_canvas_metadata`
// key inside the parent entry, which we ignore.
type CanvasResponse map[string]*CanvasEntry

func (c *Client) GetCanvasState(orgID, nodeID, branch string) (CanvasResponse, error) {
	path := fmt.Sprintf("/orgs/%s/nodes/%s/tree/%s/canvas?include_parents=full", orgID, nodeID, branch)
	resp := CanvasResponse{}
	if err := c.Get(path, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
