// Package ddtrace contains the interfaces that specify the implementations of Datadog's
// tracing library, as well as a set of sub-packages containing various implementations:
// our native implementation ("tracer"), a wrapper that can be used with Opentracing
// ("opentracer") and a mock tracer to be used for testing ("mocktracer"). Additionally,
// package "ext" provides a set of tag names and values specific to Datadog's APM product.
//
// To get started, visit the documentation for any of the packages you'd like to begin
// with by accessing the subdirectories of this package: https://godoc.org/gopkg.in/DataDog/dd-trace-go.v1/ddtrace#pkg-subdirectories.
package ddtrace // import "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"

import "time"

// Tracer specifies an implementation of the Datadog tracer which allows starting
// and propagating spans. The official implementation if exposed as functions
// within the "tracer" package.
type Tracer interface {
	// StartSpan starts a span with the given operation name and options.
	StartSpan(operationName string, opts ...StartSpanOption) Span

	// Extract extracts a span context from a given carrier. Note that baggage item
	// keys will always be lower-cased to maintain consistency. It is impossible to
	// maintain the original casing due to MIME header canonicalization standards.
	Extract(carrier interface{}) (SpanContext, error)

	// Inject injects a span context into the given carrier.
	Inject(context SpanContext, carrier interface{}) error

	// Stop stops the active tracer and sets the global tracer to a no-op. Calls to
	// Stop should be idempotent.
	Stop()
}

// Span represents a chunk of computation time. Spans have names, durations,
// timestamps and other metadata. A Tracer is used to create hierarchies of
// spans in a request, buffer and submit them to the server.
type Span interface {
	// SetTag sets a key/value pair as metadata on the span.
	SetTag(key string, value interface{})

	// SetOperationName sets the operation name for this span. An operation name should be
	// a representative name for a group of spans (e.g. "grpc.server" or "http.request").
	SetOperationName(operationName string)

	// BaggageItem returns the baggage item held by the given key.
	BaggageItem(key string) string

	// SetBaggageItem sets a new baggage item at the given key. The baggage
	// item should propagate to all descendant spans, both in- and cross-process.
	SetBaggageItem(key, val string)

	// Finish finishes the current span with the given options. Finish calls should be idempotent.
	Finish(opts ...FinishOption)

	// Context returns the SpanContext of this Span.
	Context() SpanContext
}

// A SpanInspector allows inspecting of a span's properties. It is
// used in the processing pipeline to investigate a trace's spans.
type SpanInspector interface {
	// OperationName returns the operation name of this span.
	OperationName() string

	// Tag returns the tag value at the given key.
	Tag(key string) interface{}
}

// SpanContext represents a span state that can propagate to descendant spans
// and across process boundaries. It contains all the information needed to
// spawn a direct descendant of the span that it belongs to. It can be used
// to create distributed tracing by propagating it using the provided interfaces.
type SpanContext interface {
	// ForeachBaggageItem provides an iterator over the key/value pairs set as
	// baggage within this context. Iteration stops when the handler returns
	// false.
	ForeachBaggageItem(handler func(k, v string) bool)
}

// ProcessFunc is used to transform or drop traces before they are encoded. It is called
// during the processing pipeline by receiving each span of the finished trace, along with
// a SpanInspector, the index of the span and the total number of spans in the trace. It
// has to return two boolean values: if stop is true, the iteration stops; if drop is true,
// the pipeline ends and the trace is dropped.
type ProcessFunc func(span Span, inspect SpanInspector, index, total int) (stop, drop bool)

// StartSpanOption is a configuration option that can be used with a Tracer's StartSpan method.
type StartSpanOption func(cfg *StartSpanConfig)

// FinishOption is a configuration option that can be used with a Span's Finish method.
type FinishOption func(cfg *FinishConfig)

// FinishConfig holds the configuration for finishing a span. It is usually passed around by
// reference to one or more FinishOption functions which shape it into its final form.
type FinishConfig struct {
	// FinishTime represents the time that should be set as finishing time for the
	// span. Implementations should use the current time when FinishTime.IsZero().
	FinishTime time.Time

	// Error holds an optional error that should be set on the span before
	// finishing.
	Error error
}

// StartSpanConfig holds the configuration for starting a new span. It is usually passed
// around by reference to one or more StartSpanOption functions which shape it into its
// final form.
type StartSpanConfig struct {
	// Parent holds the SpanContext that should be used as a parent for the
	// new span. If nil, implementations should return a root span.
	Parent SpanContext

	// StartTime holds the time that should be used as the start time of the span.
	// Implementations should use the current time when StartTime.IsZero().
	StartTime time.Time

	// Tags holds a set of key/value pairs that should be set as metadata on the
	// new span.
	Tags map[string]interface{}
}
