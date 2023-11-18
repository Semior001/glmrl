// Package service provides wrappers for git engines with additional methods, common for each
// git engine implementations. Consumers should use Service and should never use the naked engine.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/git/engine"
	"github.com/Semior001/glmrl/pkg/misc"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"log"
)

// Service wraps git engine client with additional functionality.
type Service struct {
	eng engine.Interface
	me  git.User
}

// NewService creates a new service.
func NewService(ctx context.Context, engine engine.Interface) (*Service, error) {
	me, err := engine.GetCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	return &Service{eng: engine, me: me}, nil
}

// ListPRsRequest is a request to list pull requests.
type ListPRsRequest struct {
	engine.ListPRsRequest

	WithoutMyUnresolvedThreads bool
	ApprovedByMe               *bool
	SatisfiesApprovalRules     *bool
	Authors                    misc.Filter[string]
	ProjectPaths               misc.Filter[string]
}

// ListPullRequests calls an underlying git engine client to list pull requests and filters them by the provided
// criteria.
func (s *Service) ListPullRequests(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error) {
	log.Printf("[DEBUG] list pull requests with criteria %+v", req)

	prs, err := s.listPRs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}

	log.Printf("[DEBUG] listed %d pull requests", len(prs))

	filter := func(name string, fn func(git.PullRequest) bool) {
		_, span := otel.GetTracerProvider().Tracer("service").
			Start(ctx, fmt.Sprintf("filter PRs by %s", name))
		defer span.End()

		var filteredURLs []string
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			if !fn(pr) {
				filteredURLs = append(filteredURLs, pr.URL)
				return false
			}
			return true
		})

		b, err := json.Marshal(prs)
		if err != nil {
			b = []byte(fmt.Sprintf("failed to marshal: %v", err))
		}

		span.SetAttributes(attribute.String("result", string(b)))
		span.SetAttributes(attribute.StringSlice("filtered_urls", filteredURLs))
	}

	if req.ApprovedByMe != nil {
		filter("approved by me", func(pr git.PullRequest) bool {
			return lo.ContainsBy(pr.Approvals.By, func(u git.User) bool {
				return u.Username == s.me.Username
			}) == *req.ApprovedByMe
		})
	}

	if req.WithoutMyUnresolvedThreads {
		filter("without my unresolved threads", func(pr git.PullRequest) bool {
			return !lo.ContainsBy(pr.Threads, func(thread git.Comment) bool {
				myUnresolvedThread := thread.Author.Username == s.me.Username && !thread.Resolved
				lastCommentMine := thread.Last().Author.Username == s.me.Username
				return myUnresolvedThread && lastCommentMine
			})
		})
	}

	if req.SatisfiesApprovalRules != nil {
		filter("satisfies approval rules", func(pr git.PullRequest) bool {
			// we should not filter PR that satisfies approval rules, but the current user
			// was explicitly requested to review this MR, and yet he didn't approve it
			approvalRequiredFromMe := lo.ContainsBy(pr.Approvals.RequestedFrom, func(u git.User) bool {
				return u.Username == s.me.Username
			})
			approvedByMe := lo.ContainsBy(pr.Approvals.By, func(u git.User) bool {
				return u.Username == s.me.Username
			})
			return (approvalRequiredFromMe && !approvedByMe) ||
				pr.Approvals.SatisfiesRules == *req.SatisfiesApprovalRules
		})
	}

	if len(req.Authors.Include) > 0 {
		filter("authors include", func(pr git.PullRequest) bool {
			return lo.Contains(req.Authors.Include, pr.Author.Username)
		})
	}

	if len(req.Authors.Exclude) > 0 {
		filter("authors exclude", func(pr git.PullRequest) bool {
			return !lo.Contains(req.Authors.Exclude, pr.Author.Username)
		})
	}

	if len(req.ProjectPaths.Include) > 0 {
		filter("project paths include", func(pr git.PullRequest) bool {
			return lo.Contains(req.ProjectPaths.Include, pr.Project.FullPath)
		})
	}

	if len(req.ProjectPaths.Exclude) > 0 {
		filter("project paths exclude", func(pr git.PullRequest) bool {
			return !lo.Contains(req.ProjectPaths.Exclude, pr.Project.FullPath)
		})
	}

	return prs, nil
}

func (s *Service) listPRs(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error) {
	ctx, span := otel.GetTracerProvider().Tracer("service").
		Start(ctx, fmt.Sprintf("list PRs from engine"))
	defer span.End()

	listFn := s.eng.ListPullRequests
	if req.Pagination.Empty() {
		listFn = func(ctx context.Context, req engine.ListPRsRequest) ([]git.PullRequest, error) {
			req.Pagination.PerPage = 100
			return misc.ListAll(1, func(page int) ([]git.PullRequest, error) {
				req.Pagination.Page = page
				return s.eng.ListPullRequests(ctx, req)
			})
		}
	}

	prs, err := listFn(ctx, req.ListPRsRequest)

	b, marshalErr := json.Marshal(prs)
	if marshalErr != nil {
		b = []byte(fmt.Sprintf("failed to marshal: %v", marshalErr))
	}

	attrs := []attribute.KeyValue{attribute.String("result", string(b))}
	if err != nil {
		attrs = append(attrs, attribute.String("err", err.Error()))
	}

	span.SetAttributes(attrs...)
	return prs, err
}

//go:generate gowrap gen -g -p . -i tracingService -t opentelemetry -o service_trace_gen.go

// tracingService defines a list of Service methods to generate a tracing wrapper.
type tracingService interface {
	ListPullRequests(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error)
}
