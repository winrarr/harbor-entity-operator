package harborclient

import (
	"context"
	"fmt"
)

// UserGroup represents a Harbor user group.
type UserGroup struct {
	ID          int    `json:"id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	GroupType   int    `json:"group_type,omitempty"`
	LDAPGroupDN string `json:"ldap_group_dn,omitempty"`
}

// ListUserGroups lists all user groups.
func (c *Client) ListUserGroups(ctx context.Context) ([]UserGroup, error) {
	return getPaged[UserGroup](ctx, c, "/api/v2.0/usergroups", nil)
}

// CreateUserGroup creates a user group.
func (c *Client) CreateUserGroup(ctx context.Context, in UserGroup) (int, error) {
	return c.createWithNumericLocationID(ctx, "/api/v2.0/usergroups", &in)
}

// GetUserGroup retrieves a group by Harbor ID.
func (c *Client) GetUserGroup(ctx context.Context, id int) (*UserGroup, error) {
	var group UserGroup
	err := c.get(ctx, fmt.Sprintf("/api/v2.0/usergroups/%d", id), &group)
	return &group, err
}

// UpdateUserGroup updates a Harbor user group.
func (c *Client) UpdateUserGroup(ctx context.Context, id int, in UserGroup) error {
	return c.put(ctx, fmt.Sprintf("/api/v2.0/usergroups/%d", id), &in)
}

// DeleteUserGroup removes a Harbor user group.
func (c *Client) DeleteUserGroup(ctx context.Context, id int) error {
	return c.deleteIgnoringNotFound(ctx, fmt.Sprintf("/api/v2.0/usergroups/%d", id))
}
