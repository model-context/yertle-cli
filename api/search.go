package api

import "fmt"

type RagScope struct {
	RootNodeID      string            `json:"root_node_id,omitempty"`
	TagFilters      map[string]string `json:"tag_filters,omitempty"`
	DirectoryPrefix string            `json:"directory_prefix,omitempty"`
}

type RagRequest struct {
	Query          string    `json:"query"`
	TopK           int       `json:"top_k,omitempty"`
	ExpansionDepth string    `json:"expansion_depth,omitempty"`
	Scope          *RagScope `json:"scope,omitempty"`
	IncludeRawText bool      `json:"include_raw_text,omitempty"`
}

type RagMatch struct {
	NodeID      string  `json:"node_id"`
	Title       string  `json:"title"`
	Score       float64 `json:"score"`
	MatchReason string  `json:"match_reason"`
}

type RagNode struct {
	NodeID      string            `json:"node_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Path        []string          `json:"path"`
	Tags        map[string]string `json:"tags"`
	Directories []string          `json:"directories"`
	TextContent string            `json:"text_content,omitempty"`
}

type RagConnection struct {
	ConnectionID   string `json:"connection_id"`
	FromNodeID     string `json:"from_node_id"`
	FromTitle      string `json:"from_title"`
	ToNodeID       string `json:"to_node_id"`
	ToTitle        string `json:"to_title"`
	Label          string `json:"label,omitempty"`
	ConnectionType string `json:"connection_type"`
}

type RagResponse struct {
	Query       string          `json:"query"`
	Matches     []RagMatch      `json:"matches"`
	Nodes       []RagNode       `json:"nodes"`
	Connections []RagConnection `json:"connections"`
}

func (c *Client) RagSearch(orgID string, req RagRequest) (*RagResponse, error) {
	var resp RagResponse
	path := fmt.Sprintf("/orgs/%s/search/retrieve", orgID)
	if err := c.Post(path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
