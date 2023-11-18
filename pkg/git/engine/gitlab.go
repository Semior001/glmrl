package engine

import (
	"context"
	"fmt"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/misc"
	cache "github.com/go-pkgz/expirable-cache/v2"
	"github.com/go-pkgz/requester"
	"github.com/go-pkgz/requester/middleware"
	"github.com/go-pkgz/requester/middleware/logger"
	"github.com/samber/lo"
	gl "github.com/xanzy/go-gitlab"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Gitlab implements Interface for Gitlab.
type Gitlab struct {
	cl            *gl.Client
	projectsCache cache.Cache[int, git.Project]
}

// NewGitlab returns a new Gitlab service.
func NewGitlab(token, baseURL string) (*Gitlab, error) {
	rq := requester.New(
		http.Client{
			Transport: otelhttp.NewTransport(
				middleware.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					req.Body = dumpBody(req.Context(), "request.body", req.Body)
					resp, err := http.DefaultTransport.RoundTrip(req)
					if err != nil {
						return nil, err
					}
					resp.Body = dumpBody(req.Context(), "response.body", resp.Body)
					return resp, nil
				}),
				otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
			),
			Timeout: time.Minute,
		},
		logger.New(logger.Func(log.Printf), logger.Prefix("[DEBUG]"), logger.WithBody).Middleware,
	)

	cl, err := gl.NewClient(
		token,
		gl.WithBaseURL(baseURL),
		gl.WithHTTPClient(rq.Client()),
	)
	if err != nil {
		return nil, fmt.Errorf("init gitlab client: %w", err)
	}

	return &Gitlab{
		cl: cl,
		projectsCache: cache.NewCache[int, git.Project]().
			WithLRU().
			WithMaxKeys(100),
	}, nil
}

// ListPullRequests lists pull requests.
func (g *Gitlab) ListPullRequests(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error) {
	opts := &gl.ListMergeRequestsOptions{
		Scope:       lo.ToPtr("all"),
		Labels:      lo.Ternary(len(req.Labels.Include) > 0, (*gl.Labels)(&req.Labels.Include), nil),
		NotLabels:   lo.Ternary(len(req.Labels.Exclude) > 0, (*gl.Labels)(&req.Labels.Exclude), nil),
		OrderBy:     lo.Ternary(req.Sort.By != "", lo.ToPtr(string(req.Sort.By)), nil),
		Sort:        lo.Ternary(req.Sort.Order != "", lo.ToPtr(string(req.Sort.Order)), nil),
		Draft:       lo.Ternary(req.State == git.StateDraft, lo.ToPtr(true), nil),
		WIP:         lo.Ternary(req.State == git.StateDraft, lo.ToPtr("yes"), lo.ToPtr("no")),
		ListOptions: gl.ListOptions{Page: req.Pagination.Page, PerPage: req.Pagination.PerPage},
	}

	// try to reduce the filtering to one of these states, instead of listing all and then filtering
	// opened, closed, locked, or merged
	switch req.State {
	case git.StateOpen:
		opts.State = lo.ToPtr("opened")
	case git.StateClosed:
		opts.State = lo.ToPtr("closed")
	case git.StateMerged:
		opts.State = lo.ToPtr("merged")
	}

	mrs, _, err := g.cl.MergeRequests.ListMergeRequests(opts, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("call api: %w", err)
	}

	result := make([]git.PullRequest, len(mrs))
	ewg, ctx := errgroup.WithContext(ctx)
	for idx, mr := range mrs {
		idx, mr := idx, mr
		ewg.Go(func() error {
			ctx, span := otel.GetTracerProvider().Tracer("gitlab").
				Start(ctx, fmt.Sprintf("Gitlab.loadPR(%d/%d)", mr.ProjectID, mr.IID))
			defer span.End()

			pr, err := g.loadPR(ctx, mr)
			if err != nil {
				return fmt.Errorf("load PR %s: %w", mr.WebURL, err)
			}

			result[idx] = pr
			return nil
		})
	}

	if err = ewg.Wait(); err != nil {
		return nil, fmt.Errorf("wait for goroutines: %w", err)
	}

	return result, nil
}

// GetCurrentUser returns the current user.
func (g *Gitlab) GetCurrentUser(ctx context.Context) (git.User, error) {
	u, _, err := g.cl.Users.CurrentUser(gl.WithContext(ctx))
	if err != nil {
		return git.User{}, fmt.Errorf("call api to get current user: %w", err)
	}
	return g.transformUser(&gl.BasicUser{Username: u.Username}), nil
}

func (g *Gitlab) loadPR(ctx context.Context, mr *gl.MergeRequest) (pr git.PullRequest, err error) {
	pr = g.transformMergeRequest(mr)

	ewg, ctx := errgroup.WithContext(ctx)
	ewg.Go(func() error {
		if pr.Project, err = g.getProject(ctx, mr.ProjectID); err != nil {
			return fmt.Errorf("get project %d: %w", mr.ProjectID, err)
		}
		return nil
	})
	ewg.Go(func() error {
		approvals, _, err := g.cl.MergeRequests.GetMergeRequestApprovals(mr.ProjectID, mr.IID, nil, gl.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("call api to get MR approvals: %w", err)
		}

		pr.Approvals.By = misc.Map(approvals.ApprovedBy, func(u *gl.MergeRequestApproverUser) git.User { return g.transformUser(u.User) })
		pr.Approvals.SatisfiesRules = approvals.Approved
		pr.Approvals.Required = approvals.ApprovalsRequired
		return nil
	})
	ewg.Go(func() error {
		if pr.History, err = g.assembleHistory(ctx, mr.ProjectID, mr.IID); err != nil {
			return fmt.Errorf("assemble history: %w", err)
		}
		pr.Threads = g.buildThreads(pr.History)
		return nil
	})

	if err = ewg.Wait(); err != nil {
		return git.PullRequest{}, fmt.Errorf("wait for goroutines: %w", err)
	}

	return pr, nil
}

func (g *Gitlab) assembleHistory(ctx context.Context, pid, iid int) ([]git.Event, error) {
	evSet := map[git.Event]struct{}{}
	rootThreads := map[string]struct{}{}

	notes, _, err := g.cl.Notes.ListMergeRequestNotes(pid, iid, nil, gl.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("call api to get MR notes: %w", err)
	}

	sort.Slice(notes, func(i, j int) bool { return notes[i].CreatedAt.Before(*notes[j].CreatedAt) })

	for _, note := range notes {
		evs, transformed := g.transformNote(rootThreads, note)
		if !transformed {
			continue
		}

		for _, ev := range evs {
			evSet[ev] = struct{}{}
			if ev.Type == git.EventTypeCommented {
				rootThreads[ev.ObjectID] = struct{}{}
			}
		}
	}

	var evs []git.Event
	for ev := range evSet {
		evs = append(evs, ev)
	}

	// sort in ascending order
	sort.Slice(evs, func(i, j int) bool { return evs[i].Timestamp.Before(evs[j].Timestamp) })

	return evs, nil
}

func (g *Gitlab) threadPos(note *gl.Note) string {
	if note.Position == nil {
		return ""
	}

	return fmt.Sprintf("%s:%d", note.Position.NewPath, note.Position.NewLine)
}

func (g *Gitlab) transformNote(rootThreads map[string]struct{}, note *gl.Note) (events []git.Event, transformed bool) {
	ev := git.Event{ID: strconv.Itoa(note.ID), Timestamp: lo.FromPtr(note.CreatedAt)}
	ev.Actor = g.transformUser(&gl.BasicUser{Username: note.Author.Username})
	if note.System {
		ev.Actor = git.SystemUser
	}

	switch {
	case strings.Contains(note.Body, "approved this merge request"):
		ev.Type = git.EventTypeApproved
	case strings.Contains(note.Body, "unapproved this merge request"):
		ev.Type = git.EventTypeUnapproved
	}

	if !note.Resolvable {
		return nil, false
	}

	ev.Type = git.EventTypeCommented
	ev.ObjectType = git.ObjectTypeComment
	ev.ObjectID = g.threadPos(note)

	if _, ok := rootThreads[g.threadPos(note)]; ok {
		ev.Type = git.EventTypeReplied
	}

	if note.Resolved {
		resEv := git.Event{
			ID:         fmt.Sprintf("%s!resolved", ev.ID),
			Actor:      g.transformUser(&gl.BasicUser{Username: note.ResolvedBy.Username}),
			Timestamp:  lo.FromPtr(note.ResolvedAt),
			Type:       git.EventTypeThreadResolved,
			ObjectID:   g.threadPos(note),
			ObjectType: git.ObjectTypeComment,
		}

		return []git.Event{ev, resEv}, true
	}

	return []git.Event{ev}, true
}

func (g *Gitlab) getProject(ctx context.Context, pid int) (git.Project, error) {
	if p, ok := g.projectsCache.Get(pid); ok {
		return p, nil
	}

	prj, _, err := g.cl.Projects.GetProject(pid, nil, gl.WithContext(ctx))
	if err != nil {
		return git.Project{}, fmt.Errorf("call api: %w", err)
	}

	p := git.Project{
		ID:       strconv.Itoa(prj.ID),
		URL:      prj.WebURL,
		Name:     prj.Name,
		FullPath: prj.PathWithNamespace,
	}
	g.projectsCache.Set(pid, p, time.Hour)
	return p, nil
}

func (g *Gitlab) transformMergeRequest(mr *gl.MergeRequest) git.PullRequest {
	pr := git.PullRequest{
		URL:    mr.WebURL,
		Number: mr.IID,
		Title:  mr.Title,
		Body:   mr.Description,
		Author: g.transformUser(mr.Author),
		// FIXME: by some reason, library encodes labels as a string, not a slice.
		Labels: lo.Flatten(lo.Map(mr.Labels, func(s string, _ int) []string {
			return strings.Split(s, ",")
		})),
		// closed at in MR points to time when MR was closed without merging,
		// so we use merged at instead.
		ClosedAt:     lo.Ternary(mr.State == "merged", lo.FromPtr(mr.MergedAt), lo.FromPtr(mr.ClosedAt)),
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Assignees:    misc.Map(mr.Assignees, g.transformUser),
		CreatedAt:    lo.FromPtr(mr.CreatedAt),
	}

	pr.Approvals.RequestedFrom = misc.Map(mr.Reviewers, g.transformUser)

	switch {
	case mr.Draft || mr.WorkInProgress:
		pr.State = git.StateDraft
	case mr.State == "opened":
		pr.State = git.StateOpen
	case mr.State == "closed":
		pr.State = git.StateClosed
	case mr.State == "merged":
		pr.State = git.StateMerged
	}

	return pr
}

func (g *Gitlab) transformUser(u *gl.BasicUser) git.User { return git.User{Username: u.Username} }

func (g *Gitlab) buildThreads(history []git.Event) []git.Comment {
	threads := map[string]*git.Comment{}
	for _, ev := range history {
		switch ev.Type {
		case git.EventTypeCommented:
			threads[ev.ObjectID] = &git.Comment{Author: ev.Actor, CreatedAt: ev.Timestamp}
		case git.EventTypeReplied:
			thread, ok := threads[ev.ObjectID]
			if !ok {
				log.Printf("[WARN] thread %q not found", ev.ObjectID)
				continue
			}

			for thread.Child != nil {
				thread = thread.Child
			}

			thread.Child = &git.Comment{Author: ev.Actor, CreatedAt: ev.Timestamp}
		case git.EventTypeThreadResolved:
			thread, ok := threads[ev.ObjectID]
			if !ok {
				log.Printf("[WARN] thread %q not found", ev.ObjectID)
				continue
			}

			thread.Resolved = true
			for thread.Child != nil {
				thread = thread.Child
				thread.Resolved = true
			}
		}
	}

	return lo.Map(lo.Values(threads), func(c *git.Comment, _ int) git.Comment { return *c })
}
