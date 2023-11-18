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
)

// List lists all merge requests that satisfy the given criteria.
type List struct {
	CommonOpts
	State             git.State    `long:"state" description:"list only merge requests with the given state"`
	Labels            FilterGroup  `group:"labels" namespace:"labels" env-namespace:"LABELS"`
	Authors           FilterGroup  `group:"authors" namespace:"authors" env-namespace:"AUTHORS"`
	ProjectPaths      FilterGroup  `group:"project-paths" namespace:"project-paths" env-namespace:"PROJECT_PATHS"`
	ApprovedByMe      NillableBool `long:"approved-by-me" choice:"true" choice:"false" choice:"" description:"list only merge requests approved by me"`
	MyThreadsResolved NillableBool `long:"my-threads-resolved" choice:"true" choice:"false" choice:"" description:"list only merge requests with my threads resolved"`
	Sort              struct {
		By    string         `long:"by" choice:"created" choice:"updated" choice:"title" default:"created" description:"sort by the given field"`
		Order misc.SortOrder `long:"order" choice:"asc" choice:"desc" default:"desc" description:"sort in the given order"`
	} `group:"sort" namespace:"sort" env-namespace:"SORT"`
	Pagination struct {
		Page    int `long:"page" description:"page number"`
		PerPage int `long:"per-page" description:"number of items per page"`
	} `group:"pagination" namespace:"pagination" env-namespace:"PAGINATION" description:"pagination options, provide none to list all"`

	Action string `long:"action" choice:"open" choice:"copy" default:"copy" description:"action to perform on pressing enter"`
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
		ApprovedByMe:      c.ApprovedByMe.Value(),
		MyThreadsResolved: c.MyThreadsResolved.Value(),
		Authors:           misc.Filter[string]{Include: c.Authors.Include, Exclude: c.Authors.Exclude},
		ProjectPaths:      misc.Filter[string]{Include: c.ProjectPaths.Include, Exclude: c.ProjectPaths.Exclude},
	}

	tbl, err := tui.NewListPR(ctx, c.Service, req, c.Action == "open")
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
