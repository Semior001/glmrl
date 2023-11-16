package tui

import (
	"context"
	"fmt"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/Semior001/glmrl/pkg/tui/teax"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/pkg/browser"
	"github.com/samber/lo"
	"log"
)

// ListPR is a TUI to list merge requests.
type ListPR struct {
	ctx         context.Context
	svc         *service.Service
	req         service.ListPRsRequest
	openOnEnter bool
}

// NewListPR returns a new ListPR TUI.
func NewListPR(
	ctx context.Context,
	svc *service.Service,
	req service.ListPRsRequest,
	openOnEnter bool,
) (*teax.Table[git.PullRequest], error) {
	a := &ListPR{ctx: ctx, svc: svc, req: req, openOnEnter: openOnEnter}
	tbl, err := teax.NewTable(ListPRColumns, a)
	if err != nil {
		return nil, fmt.Errorf("new table: %w", err)
	}
	tbl.Focus()
	return tbl, nil
}

// Load loads the merge requests.
func (l *ListPR) Load() ([]git.PullRequest, error) {
	prs, err := l.svc.ListPullRequests(l.ctx, l.req)
	if err != nil {
		return nil, fmt.Errorf("list merge requests: %w", err)
	}

	return prs, nil
}

// OnEnter either opens the merge request in the browser or copies the URL to
// the clipboard.
func (l *ListPR) OnEnter(pr git.PullRequest) error {
	if l.openOnEnter {
		if err := browser.OpenURL(pr.URL); err != nil {
			return fmt.Errorf("open URL %q: %w", pr.URL, err)
		}
		return nil
	}

	if err := clipboard.WriteAll(pr.URL); err != nil {
		return fmt.Errorf("copy URL to clipboard: %w", err)
	}

	return nil
}

type loggingWriter string

func (w loggingWriter) Write(p []byte) (n int, err error) {
	log.Printf("[DEBUG] %s: %s", string(w), string(p))
	return len(p), nil
}

// ListPRColumns are the columns to show in the table.
var ListPRColumns = []teax.Column[git.PullRequest]{
	{
		Column:  table.Column{Title: `last upd: {{.LastReload.Format "15:04" }}, total: {{.Total}}`, Width: 2},
		Extract: func(pr git.PullRequest) string { return pr.Project.FullPath },
	},
	{
		Column:  table.Column{Title: "Title", Width: 5},
		Extract: func(pr git.PullRequest) string { return pr.Title },
	},
	{
		Column:  table.Column{Title: "Author", Width: 1},
		Extract: func(pr git.PullRequest) string { return pr.Author.Username },
	},
	{
		Column:  table.Column{Title: "Created At", Width: 1},
		Extract: func(pr git.PullRequest) string { return pr.CreatedAt.Format("2006-01-02") },
	},
	{
		Column: table.Column{Title: "Threads open", Width: 1},
		Extract: func(pr git.PullRequest) string {
			resolved := lo.CountBy(pr.Threads, func(t git.Comment) bool { return t.Resolved })
			return fmt.Sprintf("%d/%d (%s)",
				resolved, len(pr.Threads),
				checkmark(resolved == len(pr.Threads)),
			)
		},
	},
	{
		Column: table.Column{Title: "Approvals", Width: 1},
		Extract: func(pr git.PullRequest) string {
			return fmt.Sprintf("%d/%d (%s)",
				len(pr.Approvals.By), pr.Approvals.Required,
				checkmark(pr.Approvals.SatisfiesRules),
			)
		},
	},
}

func checkmark(b bool) string {
	if b {
		return "✔"
	}
	return "✘"
}
