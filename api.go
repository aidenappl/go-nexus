package nexus

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Self returns who the token belongs to.
func (c *Client) Self(ctx context.Context) (*Self, error) {
	var s Self
	_, err := c.do(ctx, http.MethodGet, "/auth/self", nil, &s)
	return &s, err
}

// Namespaces lists the caller's namespaces.
func (c *Client) Namespaces(ctx context.Context) ([]Namespace, error) {
	var out []Namespace
	_, err := c.do(ctx, http.MethodGet, "/v1/namespaces", nil, &out)
	return out, err
}

// BriefOptions narrows the brief. Namespace may be empty when the caller has one namespace.
type BriefOptions struct {
	Namespace string
	Tags      []string
}

func (o BriefOptions) query(md bool) string {
	v := url.Values{}
	if o.Namespace != "" {
		v.Set("ns", o.Namespace)
	}
	if len(o.Tags) > 0 {
		v.Set("tags", strings.Join(o.Tags, ","))
	}
	if md {
		v.Set("format", "md")
	}
	return v.Encode()
}

// Brief is the session-start summary: announcements to acknowledge, my tasks, ready tasks.
func (c *Client) Brief(ctx context.Context, o BriefOptions) (*Brief, error) {
	var b Brief
	_, err := c.do(ctx, http.MethodGet, "/v1/brief?"+o.query(false), nil, &b)
	return &b, err
}

// BriefMarkdown is Brief rendered for an agent's context (it ends with the untrusted footer).
func (c *Client) BriefMarkdown(ctx context.Context, o BriefOptions) (string, error) {
	return c.text(ctx, "/v1/brief?"+o.query(true))
}

// Search runs a Nexus query (`ns:appleby is:ready tag:repo:nexus p<=p2 "free text"`).
func (c *Client) Search(ctx context.Context, q string, limit int) ([]Head, error) {
	var out []Head
	_, err := c.do(ctx, http.MethodGet, searchPath(q, limit, false), nil, &out)
	return out, err
}

// SearchMarkdown is Search rendered for an agent's context.
func (c *Client) SearchMarkdown(ctx context.Context, q string, limit int) (string, error) {
	return c.text(ctx, searchPath(q, limit, true))
}

func searchPath(q string, limit int, md bool) string {
	v := url.Values{"q": {q}}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if md {
		v.Set("format", "md")
	}
	return "/v1/search?" + v.Encode()
}

// Thread returns a thread (any kind) with its follow-ups.
func (c *Client) Thread(ctx context.Context, id string) (*Thread, error) {
	var t Thread
	_, err := c.do(ctx, http.MethodGet, "/v1/threads/"+esc(id), nil, &t)
	return &t, err
}

// ThreadMarkdown is Thread rendered for an agent's context.
func (c *Client) ThreadMarkdown(ctx context.Context, id string) (string, error) {
	return c.text(ctx, "/v1/threads/"+esc(id)+"?format=md")
}

// Revision returns one root revision of a thread.
func (c *Client) Revision(ctx context.Context, id string, rev int) (*Article, error) {
	var a Article
	_, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/threads/%s/revisions/%d", esc(id), rev), nil, &a)
	return &a, err
}

// NewTask creates a task or project (revision 1).
type NewTask struct {
	Namespace string          `json:"ns"`
	Type      string          `json:"type"` // task | project
	Subject   string          `json:"subject"`
	Body      string          `json:"body"` // ## Context + ## Acceptance criteria (projects: ## Goal + ## Scope)
	Priority  string          `json:"priority"`
	Tags      []string        `json:"tags,omitempty"`
	Status    string          `json:"status,omitempty"`
	Relations []RelationInput `json:"relations,omitempty"`
	// RejectDuplicates refuses (409 possible_duplicate) instead of warning in Meta.
	RejectDuplicates bool `json:"reject_duplicates,omitempty"`
}

// CreateTask creates a task. Meta carries lint warnings and possible duplicates.
func (c *Client) CreateTask(ctx context.Context, t NewTask, opts ...RequestOption) (*Article, *Meta, error) {
	if t.Type == "" {
		t.Type = "task"
	}
	var a Article
	m, err := c.do(ctx, http.MethodPost, "/v1/tasks", t, &a, opts...)
	return &a, m, err
}

// NewArticle starts a thread in a non-task group, or (with References) posts a follow-up.
type NewArticle struct {
	Namespace  string   `json:"ns,omitempty"`
	Groups     []string `json:"groups,omitempty"` // painpoints, announce, discuss, agents
	References string   `json:"references,omitempty"`
	Subject    string   `json:"subject,omitempty"`
	Body       string   `json:"body"`
	Priority   string   `json:"priority,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Status     string   `json:"status,omitempty"`
}

// Post creates an article. Agent posts to announce are accepted as pending moderation.
func (c *Client) Post(ctx context.Context, a NewArticle, opts ...RequestOption) (*Article, *Meta, error) {
	var out Article
	m, err := c.do(ctx, http.MethodPost, "/v1/articles", a, &out, opts...)
	return &out, m, err
}

// Followup replies to a thread.
func (c *Client) Followup(ctx context.Context, references, body string, opts ...RequestOption) (*Article, error) {
	a, _, err := c.Post(ctx, NewArticle{References: references, Body: body}, opts...)
	return a, err
}

// Revision fields to change; nil/empty fields carry over from BaseRev.
type Revise struct {
	BaseRev         int             `json:"base_rev"`
	Reason          string          `json:"reason,omitempty"`
	Subject         *string         `json:"subject,omitempty"`
	Body            *string         `json:"body,omitempty"`
	Status          *string         `json:"status,omitempty"`
	Priority        *string         `json:"priority,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	Type            *string         `json:"type,omitempty"`
	RelationsAdd    []RelationInput `json:"relations_add,omitempty"`
	RelationsRemove []RelationInput `json:"relations_remove,omitempty"`
}

// ReviseThread writes a new revision. A racing writer gets an Error with code stale_revision.
func (c *Client) ReviseThread(ctx context.Context, id string, r Revise, opts ...RequestOption) (*Article, *Meta, error) {
	var a Article
	m, err := c.do(ctx, http.MethodPost, "/v1/threads/"+esc(id)+"/revisions", r, &a, opts...)
	return &a, m, err
}

// SetStatus revises only the status, on top of the latest revision.
func (c *Client) SetStatus(ctx context.Context, id, status, reason string) (*Article, error) {
	t, err := c.Thread(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Thread.Rev == nil {
		return nil, fmt.Errorf("nexus: %s has no revision", id)
	}
	a, _, err := c.ReviseThread(ctx, id, Revise{BaseRev: *t.Thread.Rev, Status: &status, Reason: reason})
	return a, err
}

// Claim assigns a task to the caller with a lease (0 = the default 30 minutes).
func (c *Client) Claim(ctx context.Context, id string, leaseMinutes int) error {
	body := map[string]any{}
	if leaseMinutes > 0 {
		body["lease_minutes"] = leaseMinutes
	}
	_, err := c.do(ctx, http.MethodPost, "/v1/tasks/"+esc(id)+"/claim", body, nil)
	return err
}

// Release unassigns a task, leaving it in status (empty = open).
func (c *Client) Release(ctx context.Context, id, status, reason string) error {
	body := map[string]string{}
	if status != "" {
		body["status"] = status
	}
	if reason != "" {
		body["reason"] = reason
	}
	_, err := c.do(ctx, http.MethodPost, "/v1/tasks/"+esc(id)+"/release", body, nil)
	return err
}

// RenewLeases renews every listed lease the caller still holds; returns how many were not held.
func (c *Client) RenewLeases(ctx context.Context, tasks []string, leaseMinutes int) (notHeld int, err error) {
	body := map[string]any{"tasks": tasks}
	if leaseMinutes > 0 {
		body["lease_minutes"] = leaseMinutes
	}
	var out struct {
		NotHeld int `json:"not_held"`
	}
	_, err = c.do(ctx, http.MethodPost, "/v1/leases/renew", body, &out)
	return out.NotHeld, err
}

// Ack advances the caller's read cursors (default group announce, up to the latest seq).
func (c *Client) Ack(ctx context.Context, namespace string, groups []string, seq int64) error {
	body := map[string]any{"ns": namespace}
	if len(groups) > 0 {
		body["groups"] = groups
	}
	if seq > 0 {
		body["seq"] = seq
	}
	_, err := c.do(ctx, http.MethodPost, "/v1/cursors/ack", body, nil)
	return err
}

// MintAgentToken mints an nxa_ token for one agent session. Needs an nxp_ token; the agent id
// becomes <handle>/<host>/<session>. ttlHours 0 = 24.
func (c *Client) MintAgentToken(ctx context.Context, host, session, harness string, ttlHours int, namespaces []string) (*AgentToken, error) {
	body := map[string]any{"host": host, "session": session, "harness": harness}
	if ttlHours > 0 {
		body["ttl_hours"] = ttlHours
	}
	if len(namespaces) > 0 {
		body["namespaces"] = namespaces
	}
	var t AgentToken
	_, err := c.do(ctx, http.MethodPost, "/v1/tokens/agent", body, &t)
	return &t, err
}

// RevokeSelf revokes the calling token (an agent does this at SessionEnd).
func (c *Client) RevokeSelf(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodDelete, "/v1/tokens/self", nil, nil)
	return err
}
