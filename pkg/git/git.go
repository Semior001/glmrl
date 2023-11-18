package git

import "time"

// State is a pull request state.
type State string

const (
	// StateDraft is a draft pull request state.
	StateDraft State = "draft"
	// StateMerged is a merged pull request state.
	StateMerged State = "merged"
	// StateClosed is a closed pull request state.
	StateClosed State = "closed"
	// StateOpen is an open pull request state.
	StateOpen State = "open"
)

// PullRequest describes a pull request.
type PullRequest struct {
	URL          string   `json:"url"`
	Number       int      `json:"number"`
	Project      Project  `json:"project"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Author       User     `json:"author"`
	Labels       []string `json:"labels"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Assignees    []User   `json:"assignees"`
	Approvals    struct {
		RequestedFrom  []User `json:"requested_from"`
		By             []User `json:"by"`
		SatisfiesRules bool   `json:"satisfies_rules"`
		Required       int    `json:"required"`
	}
	History []Event   `json:"history"`
	Threads []Comment `json:"threads"`
	State   State     `json:"state"`

	ClosedAt  time.Time `json:"closed_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Project holds project data.
type Project struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

// User holds user data.
type User struct {
	Username string `json:"username"`
}

// SystemUser is a system user.
var SystemUser = User{Username: "system"}

// Comment describes a comment.
type Comment struct {
	Author    User      `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved"`
	Child     *Comment  `json:"child"`
}

// Last returns the last comment in the thread.
func (c *Comment) Last() Comment {
	if c.Child == nil {
		return *c
	}
	return c.Child.Last()
}

// Event describes a pull request event.
type Event struct {
	ID string `json:"id"`

	Actor     User      `json:"actor"`
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`

	ObjectID   string     `json:"object_id"`
	ObjectType ObjectType `json:"object_type"`
}

// EventType describes a pull request event type.
type EventType string

// Observable event types.
// If not explicitly specified, object id and type will be empty.
const (
	// EventTypeThreadResolved is a pull request event type for a resolution of a thread.
	// Object ID will be a position (file:line) thread (root comment) ID and type
	// will be "comment".
	EventTypeThreadResolved EventType = "resolved"
	// EventTypeCommented is a pull request event type for a comment.
	// If a comment is a reply to another comment, object ID will be a position of
	// parent comment (file:line) and type will be "comment".
	EventTypeCommented EventType = "commented"
	// EventTypeReplied is a pull request event type for a reply to a comment.
	// Object ID will be a position of parent comment (file:line) and type will be
	// "comment".
	EventTypeReplied EventType = "replied"

	// EventTypeApproved is a pull request event type for an approval.
	EventTypeApproved EventType = "approved"
	// EventTypeUnapproved is a pull request event type for an unapproval.
	EventTypeUnapproved EventType = "unapproved"
)

// ObjectType defines an object over which an event was performed.
type ObjectType string

const (
	// ObjectTypeComment is a pull request event object type for a comment.
	ObjectTypeComment ObjectType = "comment"
	// ObjectTypeCommit is a pull request event object type for a commit.
	ObjectTypeCommit ObjectType = "commit"
)
