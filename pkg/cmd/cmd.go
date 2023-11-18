package cmd

import (
	"github.com/Semior001/glmrl/pkg/service"
	"github.com/samber/lo"
)

// CommonOpts contains common options for all commands.
type CommonOpts struct {
	Service *service.Service
	Version string
}

func (c *CommonOpts) Set(opts CommonOpts) {
	c.Service = opts.Service
	c.Version = opts.Version
}

// FilterGroup is a group of include/exclude filters
type FilterGroup struct {
	Include []string `long:"include" description:"list only entries that include the given value"`
	Exclude []string `long:"exclude" description:"list only entries that exclude the given value"`
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
