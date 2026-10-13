package harborclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type HarborRole struct {
	ID          int                    `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Permissions []HarborRolePermission `json:"permissions,omitempty"`
	IsBuiltin   bool                   `json:"is_builtin,omitempty"`
}

type HarborRolePermission struct {
	Kind      string             `json:"kind,omitempty"`
	Namespace string             `json:"namespace,omitempty"`
	Access    []HarborRoleAccess `json:"access,omitempty"`
}

type HarborRoleAccess struct {
	Resource string `json:"resource,omitempty"`
	Action   string `json:"action,omitempty"`
	Effect   string `json:"effect,omitempty"`
}

func (c *Client) ListHarborRoles(ctx context.Context) ([]HarborRole, error) {
	return getPaged[HarborRole](ctx, c, "/api/v2.0/roles", nil)
}

func (c *Client) GetHarborRole(ctx context.Context, id int) (*HarborRole, error) {
	var role HarborRole
	err := c.get(ctx, fmt.Sprintf("/api/v2.0/roles/%d", id), &role)
	return &role, err
}

func (c *Client) CreateHarborRole(ctx context.Context, in HarborRole) (int, error) {
	var created struct {
		ID int `json:"id"`
	}
	_, err := c.do(ctx, http.MethodPost, "/api/v2.0/roles", &in, &created)
	return created.ID, err
}

func (c *Client) UpdateHarborRole(ctx context.Context, id int, in HarborRole) error {
	return c.put(ctx, fmt.Sprintf("/api/v2.0/roles/%d", id), &in)
}

func (c *Client) DeleteHarborRole(ctx context.Context, id int) error {
	return c.deleteIgnoringNotFound(ctx, fmt.Sprintf("/api/v2.0/roles/%d", id))
}

type SystemCVEAllowlist struct {
	ID           int                      `json:"id,omitempty"`
	ProjectID    int                      `json:"project_id,omitempty"`
	ExpiresAt    *int64                   `json:"expires_at,omitempty"`
	Items        []SystemCVEAllowlistItem `json:"items"`
	CreationTime string                   `json:"creation_time,omitempty"`
	UpdateTime   string                   `json:"update_time,omitempty"`
}

type SystemCVEAllowlistItem struct {
	CVEID string `json:"cve_id"`
}

func (c *Client) GetSystemCVEAllowlist(ctx context.Context) (*SystemCVEAllowlist, error) {
	var allowlist SystemCVEAllowlist
	err := c.get(ctx, "/api/v2.0/system/CVEAllowlist", &allowlist)
	return &allowlist, err
}

func (c *Client) UpdateSystemCVEAllowlist(ctx context.Context, in SystemCVEAllowlist) error {
	return c.put(ctx, "/api/v2.0/system/CVEAllowlist", &in)
}

type PreheatInstance struct {
	ID             int               `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Description    string            `json:"description,omitempty"`
	Vendor         string            `json:"vendor,omitempty"`
	Endpoint       string            `json:"endpoint,omitempty"`
	AuthMode       string            `json:"auth_mode,omitempty"`
	AuthInfo       map[string]string `json:"auth_info,omitempty"`
	Status         string            `json:"status,omitempty"`
	Enabled        bool              `json:"enabled"`
	Default        bool              `json:"default"`
	Insecure       bool              `json:"insecure"`
	SetupTimestamp int64             `json:"setup_timestamp,omitempty"`
}

func (c *Client) ListPreheatInstances(ctx context.Context) ([]PreheatInstance, error) {
	return getPaged[PreheatInstance](ctx, c, "/api/v2.0/p2p/preheat/instances", nil)
}

func (c *Client) GetPreheatInstance(ctx context.Context, name string) (*PreheatInstance, error) {
	var instance PreheatInstance
	err := c.get(ctx, "/api/v2.0/p2p/preheat/instances/"+url.PathEscape(name), &instance)
	return &instance, err
}

func (c *Client) CreatePreheatInstance(ctx context.Context, in PreheatInstance) (int, error) {
	_, err := c.do(ctx, http.MethodPost, "/api/v2.0/p2p/preheat/instances", &in, nil)
	return 0, err
}

func (c *Client) UpdatePreheatInstance(ctx context.Context, name string, in PreheatInstance) error {
	return c.put(ctx, "/api/v2.0/p2p/preheat/instances/"+url.PathEscape(name), &in)
}

func (c *Client) DeletePreheatInstance(ctx context.Context, name string) error {
	return c.deleteIgnoringNotFound(ctx, "/api/v2.0/p2p/preheat/instances/"+url.PathEscape(name))
}

type PreheatPolicy struct {
	ID           int    `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	ProjectID    int    `json:"project_id,omitempty"`
	ProviderID   int    `json:"provider_id,omitempty"`
	ProviderName string `json:"provider_name,omitempty"`
	Filters      string `json:"filters,omitempty"`
	Trigger      string `json:"trigger,omitempty"`
	Enabled      bool   `json:"enabled"`
	ExtraAttrs   string `json:"extra_attrs,omitempty"`
	CreationTime string `json:"creation_time,omitempty"`
	UpdateTime   string `json:"update_time,omitempty"`
}

func (c *Client) ListPreheatPolicies(ctx context.Context, project string) ([]PreheatPolicy, error) {
	return getPaged[PreheatPolicy](ctx, c, "/api/v2.0/projects/"+url.PathEscape(project)+"/preheat/policies", nil)
}

func (c *Client) GetPreheatPolicy(ctx context.Context, project, name string) (*PreheatPolicy, error) {
	var policy PreheatPolicy
	err := c.get(ctx, "/api/v2.0/projects/"+url.PathEscape(project)+"/preheat/policies/"+url.PathEscape(name), &policy)
	return &policy, err
}

func (c *Client) CreatePreheatPolicy(ctx context.Context, project string, in PreheatPolicy) (int, error) {
	_, err := c.do(ctx, http.MethodPost, "/api/v2.0/projects/"+url.PathEscape(project)+"/preheat/policies", &in, nil)
	return 0, err
}

func (c *Client) UpdatePreheatPolicy(ctx context.Context, project, name string, in PreheatPolicy) error {
	return c.put(ctx, "/api/v2.0/projects/"+url.PathEscape(project)+"/preheat/policies/"+url.PathEscape(name), &in)
}

func (c *Client) DeletePreheatPolicy(ctx context.Context, project, name string) error {
	return c.deleteIgnoringNotFound(ctx, "/api/v2.0/projects/"+url.PathEscape(project)+"/preheat/policies/"+url.PathEscape(name))
}
