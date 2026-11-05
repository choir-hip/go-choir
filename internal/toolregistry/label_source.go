package toolregistry

import (
	"context"
	"sort"
	"strings"
)

// The run's labels at a model call (MA3.6, R15). A model call is not a
// disclosure sink: the provider is chosen by the user's VM policy and no rule
// constrains it. It is LOGGED instead, with the resolved provider and model
// and the label set the run carried when it made the call, so a reader of the
// tape can see which provider saw data of which class. The runtime installs a
// source that reads the run's accumulator; the tool loop reads it at each
// call.

type labelSourceKey struct{}

// WithLabelSource installs a function that returns the run's current label
// set. A nil source leaves ctx unchanged.
func WithLabelSource(ctx context.Context, source func() []string) context.Context {
	if source == nil {
		return ctx
	}
	return context.WithValue(ctx, labelSourceKey{}, source)
}

// LabelsFrom returns the run's label set from ctx, sorted, or nil.
func LabelsFrom(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	source, ok := ctx.Value(labelSourceKey{}).(func() []string)
	if !ok || source == nil {
		return nil
	}
	labels := append([]string(nil), source()...)
	sort.Strings(labels)
	return labels
}

// ModelCallLine is the structured log line one model call writes: the run, the
// resolved provider and model, and the run's labels.
func ModelCallLine(runID, provider, model string, labels []string) string {
	if len(labels) == 0 {
		return "model call: run=" + runID + " provider=" + provider + " model=" + model + " labels=-"
	}
	return "model call: run=" + runID + " provider=" + provider + " model=" + model + " labels=" + strings.Join(labels, ",")
}
