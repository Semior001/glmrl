// Package engine provides implementations of the git engine client.
package engine

import (
	"bytes"
	"context"
	"github.com/Semior001/glmrl/pkg/git"
	"github.com/Semior001/glmrl/pkg/misc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"log"
)

// ListPRsRequest is a request to list pull requests.
// Bools with pointers are used to specify whether to include (true) or exclude (false) pull requests with a
// specific state. If a pointer is nil, pull requests with the corresponding state are not filtered.
type ListPRsRequest struct {
	State      git.State
	Labels     misc.Filter[string]
	Sort       misc.Sort
	Pagination misc.Pagination
}

//go:generate gowrap gen -g -p . -i Interface -t opentelemetry -o engine_trace_gen.go

// Interface defines methods each git engine client should implement.
type Interface interface {
	// ListPullRequests lists pull requests.
	ListPullRequests(ctx context.Context, req ListPRsRequest) ([]git.PullRequest, error)
	// GetCurrentUser returns the current user.
	GetCurrentUser(ctx context.Context) (git.User, error)
}

// dumpBody dumps the reader's content to span's attributes and makes a new reader from it.
func dumpBody(ctx context.Context, key string, rd io.ReadCloser) io.ReadCloser {
	span := trace.SpanFromContext(ctx)
	if rd == nil {
		span.SetAttributes(attribute.String(key, "nil"))
		return nil
	}

	b, err := io.ReadAll(rd)
	if err != nil {
		log.Printf("[WARN] read body: %v", err)
		return io.NopCloser(io.MultiReader(bytes.NewReader(b), rd))
	}

	span.SetAttributes(attribute.String(key, string(b)))

	return io.NopCloser(io.MultiReader(bytes.NewReader(b), rd))
}
