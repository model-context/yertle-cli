package api

import "fmt"

type Organization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PublicID    string `json:"public_id"`
	InviteMode  string `json:"invite_mode"`
	Role        string `json:"role"`
	MemberCount *int   `json:"member_count"`
	NodeCount   *int   `json:"node_count"`
	RootNodeID  string `json:"root_node_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type OrganizationListResponse struct {
	Organizations []Organization `json:"organizations"`
	Total         int            `json:"total"`
}

func (c *Client) GetOrganizations() ([]Organization, error) {
	var resp OrganizationListResponse
	if err := c.Get("/orgs", &resp); err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

func (c *Client) GetOrganization(orgID string) (*Organization, error) {
	path := fmt.Sprintf("/orgs/%s", orgID)
	var resp Organization
	if err := c.Get(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
