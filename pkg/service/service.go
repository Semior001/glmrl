// Package service provides wrappers for git engines with additional methods, common for each
// git engine implementations. Consumers should use Service and should never use the naked engine.
package service

import (
	"context"
	"fmt"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/git/engine"
	"github.com/Semior001/glmrl/pkg/misc"
	"github.com/samber/lo"
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

	ApprovedByMe      *bool
	MyThreadsResolved *bool
	Authors           misc.Filter[string]
	ProjectPaths      misc.Filter[string]
}

// ListPullRequests calls an underlying git engine client to list pull requests and filters them by the provided
// criteria.
func (s *Service) ListPullRequests(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error) {
	log.Printf("[DEBUG] list pull requests with criteria %+v", req)
	prs, err := s.eng.ListPullRequests(ctx, req.ListPRsRequest)
	if err != nil {
		return nil, fmt.Errorf("list pull requests by engine: %w", err)
	}
	log.Printf("[DEBUG] listed %d pull requests", len(prs))

	if req.ApprovedByMe != nil {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return lo.ContainsBy(pr.Approvals.By, func(u git.User) bool {
				return u.Username == s.me.Username
			}) == *req.ApprovedByMe
		})
	}

	if req.MyThreadsResolved != nil {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return lo.ContainsBy(pr.Threads, func(c git.Comment) bool {
				return c.Author.Username == s.me.Username && c.Resolved
			}) == *req.MyThreadsResolved
		})
	}

	if len(req.Authors.Include) > 0 {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return lo.Contains(req.Authors.Include, pr.Author.Username)
		})
	}

	if len(req.Authors.Exclude) > 0 {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return !lo.Contains(req.Authors.Exclude, pr.Author.Username)
		})
	}

	if len(req.ProjectPaths.Include) > 0 {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return lo.Contains(req.ProjectPaths.Include, pr.Project.FullPath)
		})
	}

	if len(req.ProjectPaths.Exclude) > 0 {
		prs = lo.Filter(prs, func(pr git.PullRequest, _ int) bool {
			return !lo.Contains(req.ProjectPaths.Exclude, pr.Project.FullPath)
		})
	}

	return prs, nil
}
