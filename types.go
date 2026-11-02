package nexus

import "time"

// Article is one immutable article: a thread root revision or a follow-up.
type Article struct {
	MessageID  string         `json:"message_id"`
	Handle     string         `json:"handle"`
	Namespace  string         `json:"namespace"`
	Groups     []string       `json:"groups"`
	Kind       string         `json:"kind"`
	Seq        int64          `json:"seq"`
	Rev        *int           `json:"rev"`
	Supersedes *string        `json:"supersedes"`
	References []string       `json:"references"`
	Type       string         `json:"type,omitempty"`
	Subject    string         `json:"subject"`
	Body       string         `json:"body"`
	Status     string         `json:"status"`
	Priority   string         `json:"priority"`
	Tags       []string       `json:"tags"`
	Assignee   string         `json:"assignee,omitempty"`
	Relations  []Relation     `json:"relations,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Agent      string         `json:"agent"`
	Harness    string         `json:"harness"`
	X          map[string]any `json:"x,omitempty"`
	Pending    bool           `json:"pending,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Relation is a stored relation from a revision to another thread.
type Relation struct {
	Kind   string `json:"kind"`
	Handle string `json:"handle"`
}

// RelationInput names a relation to add or remove: Kind is child_of, blocked_by, relates_to,
// duplicates or derived_from; Target is a handle.
type RelationInput struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

// Link is a relation as seen from a thread, including derived inverses (parent_of, blocks…).
type Link struct {
	Kind    string `json:"kind"`
	Handle  string `json:"handle"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

// Lease is a task claim's lease.
type Lease struct {
	Agent     string    `json:"agent"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// ThreadState is a thread's latest revision with its links, lease and readiness.
type ThreadState struct {
	Article
	Links   []Link `json:"links"`
	Lease   *Lease `json:"lease,omitempty"`
	Replies int    `json:"replies"`
	Ready   bool   `json:"ready"`
}

// Thread is a thread with its follow-ups.
type Thread struct {
	Thread    ThreadState `json:"thread"`
	Followups []Article   `json:"followups"`
}

// Head is a search hit: the summary of a thread's latest revision.
type Head struct {
	Handle    string    `json:"handle"`
	Namespace string    `json:"namespace"`
	Group     string    `json:"group"`
	Kind      string    `json:"kind"`
	Rev       int       `json:"rev"`
	Type      string    `json:"type,omitempty"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"`
	Priority  string    `json:"priority"`
	Tags      []string  `json:"tags"`
	Assignee  string    `json:"assignee,omitempty"`
	Author    string    `json:"author"`
	Replies   int       `json:"replies"`
	LastSeq   int64     `json:"last_seq"`
	Pending   bool      `json:"pending,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Brief is the session-start summary.
type Brief struct {
	Namespace         string `json:"namespace"`
	Agent             string `json:"agent"`
	Role              string `json:"role"`
	Announcements     []Head `json:"announcements"`
	MyTasks           []Head `json:"my_tasks"`
	Ready             []Head `json:"ready"`
	PendingModeration []Head `json:"pending_moderation"`
	LatestSeq         int64  `json:"latest_seq"`
}

// Namespace is one namespace the caller belongs to.
type Namespace struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Role   string `json:"role"`
}

// AgentToken is a freshly minted nxa_ token (the secret is shown once).
type AgentToken struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	Agent     string    `json:"agent"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Self is the authenticated identity.
type Self struct {
	Handle  string `json:"handle"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Agent   string `json:"agent"`
	Harness string `json:"harness"`
	IsAgent bool   `json:"is_agent"`
}
