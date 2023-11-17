package misc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// AttributesSpanDecorator is used alongside with gowrap's generated proxy, to format
// the span with arguments and results as independent span attributes.
//
// Example of go:generate:
//
//	//go:generate gowrap gen -g -p . -i tracingClient -t opentelemetry -o client_trace_gen.go
//
// Example of injecting:
//
//	 someservicepkgClient := someservicepkg.NewtracingClientWithTracing(
//	   someservicepkg.NewClient(factory.ObjectRepository()), // original client
//	   "someservicepkg", // name of the service that will appear in span
//	   gowrap.AttributesSpanDecorator,
//	)
func AttributesSpanDecorator(span trace.Span, params, results map[string]interface{}) {
	for k, v := range params {
		k = fmt.Sprintf("args.%s", k)
		if ignore(v) {
			continue
		}

		bts, err := json.Marshal(v)
		if err != nil {
			span.SetAttributes(attribute.String(k, "can't marshal"))
			continue
		}
		span.SetAttributes(attribute.String(k, string(bts)))
	}

	for k, v := range results {
		k = fmt.Sprintf("results.%s", k)

		if ignore(v) {
			continue
		}

		if err, ok := v.(error); ok && err != nil {
			span.RecordError(err)
			span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
			continue
		}

		bts, err := json.Marshal(v)
		if err != nil {
			span.SetAttributes(attribute.String(k, "can't marshal"))
			continue
		}
		span.SetAttributes(attribute.String(k, string(bts)))
	}
}

func ignore(v interface{}) bool {
	_, ok := v.(context.Context)
	return ok
}
