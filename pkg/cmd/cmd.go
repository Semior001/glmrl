package cmd

import (
	"context"
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/samber/lo"
)

// CommonOpts contains common options for all commands.
type CommonOpts struct {
	PrepareService func(ctx context.Context) (*service.Service, error)
	Version        string
}

func (c *CommonOpts) Set(opts CommonOpts) {
	c.PrepareService = opts.PrepareService
	c.Version = opts.Version
}

// FilterGroup is a group of include/exclude filters
type FilterGroup struct {
	Include []string `long:"include" description:"list only entries that include the given value"`
	Exclude []string `long:"exclude" description:"list only entries that exclude the given value"`
}

// Empty returns true if the filter group is empty.
func (g FilterGroup) Empty() bool {
	return len(g.Include) == 0 && len(g.Exclude) == 0
}

// NillableBool is a bool that can be nil
type NillableBool string

// Value returns the value of the bool.
func (b NillableBool) Value() *bool {
	switch b {
	case "true":
		return lo.ToPtr(true)
	case "false":
		return lo.ToPtr(false)
	default:
		return nil
	}
}

// Not returns the opposite value of the nillable bool, except for nil, which is nil.
func Not(b NillableBool) NillableBool {
	switch b {
	case "true":
		return "false"
	case "false":
		return "true"
	default:
		return ""
	}
}
