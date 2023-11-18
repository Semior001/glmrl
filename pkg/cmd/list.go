package cmd

import (
	"context"
	"fmt"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/git/engine"
	"github.com/Semior001/glmrl/pkg/misc"
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/Semior001/glmrl/pkg/tui"
	"github.com/Semior001/glmrl/pkg/tui/teax"
	"github.com/samber/lo"
	"time"
)

// List lists all merge requests that satisfy the given criteria.
type List struct {
	CommonOpts
	State                      git.State    `long:"state" description:"list only merge requests with the given state"`
	Labels                     FilterGroup  `group:"labels" namespace:"labels" env-namespace:"LABELS"`
	Authors                    FilterGroup  `group:"authors" namespace:"authors" env-namespace:"AUTHORS"`
	ProjectPaths               FilterGroup  `group:"project-paths" namespace:"project-paths" env-namespace:"PROJECT_PATHS"`
	ApprovedByMe               NillableBool `long:"approved-by-me" choice:"true" choice:"false" choice:"" description:"list only merge requests approved by me"`
	WithoutMyUnresolvedThreads bool         `long:"without-my-unresolved-threads" description:"list only merge requests without MY unresolved threads, but lists threads where my action is required"`
	NotEnoughApprovals         NillableBool `long:"not-enough-approvals" description:"list only merge requests with not enough approvals, but show the ones where I've been requested as a reviewer and didn't approve it"`
	Sort                       struct {
		By    string         `long:"by" choice:"created" choice:"updated" choice:"title" default:"created" description:"sort by the given field"`
		Order misc.SortOrder `long:"order" choice:"asc" choice:"desc" default:"desc" description:"sort in the given order"`
	} `group:"sort" namespace:"sort" env-namespace:"SORT"`
	Pagination struct {
		Page    int `long:"page" description:"page number"`
		PerPage int `long:"per-page" description:"number of items per page"`
	} `group:"pagination" namespace:"pagination" env-namespace:"PAGINATION" description:"pagination options, provide none to list all"`
	Action       string        `long:"action" choice:"open" choice:"copy" default:"open" description:"action to perform on pressing enter"`
	PollInterval time.Duration `long:"poll-interval" default:"5m" description:"interval to poll for new merge requests, 0 means no polling, only manual refresh"`
}

func (c List) validateBackendFilters() error {
	type filter struct {
		name    string
		present bool
	}

	filters := []filter{
		{name: "state", present: c.State != ""},
		{name: "labels", present: !c.Labels.Empty()},
		{name: "authors", present: !c.Authors.Empty()},
		{name: "pagination", present: c.Pagination.Page != 0 && c.Pagination.PerPage != 0},
	}

	for _, f := range filters {
		if f.present {
			return nil
		}
	}

	return fmt.Errorf("at least one backend-side filter must be present, available filters: %v",
		lo.Map(filters, func(f filter, _ int) string { return f.name }))
}

// Execute runs the command.
func (c List) Execute([]string) error {
	ctx := context.Background()

	req := service.ListPRsRequest{
		ListPRsRequest: engine.ListPRsRequest{
			State:  c.State,
			Labels: misc.Filter[string]{Include: c.Labels.Include, Exclude: c.Labels.Exclude},
			Sort: misc.Sort{
				By:    transformSortBy(c.Sort.By),
				Order: c.Sort.Order,
			},
			Pagination: misc.Pagination{Page: c.Pagination.Page, PerPage: c.Pagination.PerPage},
		},
		ApprovedByMe:               c.ApprovedByMe.Value(),
		WithoutMyUnresolvedThreads: c.WithoutMyUnresolvedThreads,
		SatisfiesApprovalRules:     Not(c.NotEnoughApprovals).Value(),
		Authors:                    misc.Filter[string]{Include: c.Authors.Include, Exclude: c.Authors.Exclude},
		ProjectPaths:               misc.Filter[string]{Include: c.ProjectPaths.Include, Exclude: c.ProjectPaths.Exclude},
	}

	if err := c.validateBackendFilters(); err != nil {
		return fmt.Errorf("validate backend filters: %w", err)
	}

	svc, err := c.PrepareService(ctx)
	if err != nil {
		return fmt.Errorf("init service: %w", err)
	}

	tbl, err := tui.NewListPR(ctx, tui.ListPRParams{
		Service:      service.NewtracingServiceWithTracing(svc, "PrepareService", misc.AttributesSpanDecorator),
		Request:      req,
		OpenOnEnter:  c.Action == "open",
		PollInterval: c.PollInterval,
		Version:      c.Version,
	})
	if err != nil {
		return fmt.Errorf("initialize list prs tui: %w", err)
	}

	if err := teax.Run(ctx, tbl); err != nil {
		return fmt.Errorf("run list mrs tui: %w", err)
	}

	return nil
}

func transformSortBy(by string) misc.SortBy {
	switch by {
	case "created":
		return misc.SortByCreatedAt
	case "updated":
		return misc.SortByUpdatedAt
	case "title":
		return misc.SortByTitle
	default:
		return ""
	}
}
