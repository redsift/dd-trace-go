package tracer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTracer(t *testing.T) {
	assert := assert.New(t)

	// the default client must be available
	assert.NotNil(DefaultTracer)

	// package free functions must proxy the calls to the
	// default client
	root := NewSpan("pylons.request", "pylons", "/")
	NewChildSpan("pylons.request", root)
	Disable()
	Enable()
}

func TestNewSpan(t *testing.T) {
	assert := assert.New(t)

	// the tracer must create root spans
	tracer := NewTracer()
	span := tracer.NewSpan("pylons.request", "pylons", "/")
	assert.Equal(span.ParentID, uint64(0))
	assert.Equal(span.Service, "pylons")
	assert.Equal(span.Name, "pylons.request")
	assert.Equal(span.Resource, "/")
}

func TestNewSpanChild(t *testing.T) {
	assert := assert.New(t)

	// the tracer must create child spans
	tracer := NewTracer()
	parent := tracer.NewSpan("pylons.request", "pylons", "/")
	child := tracer.NewChildSpan("redis.command", parent)
	// ids and services are inherited
	assert.Equal(child.ParentID, parent.SpanID)
	assert.Equal(child.TraceID, parent.TraceID)
	assert.Equal(child.Service, parent.Service)
	// the resource is not inherited and defaults to the name
	assert.Equal(child.Resource, "redis.command")
	// the tracer instance is the same
	assert.Equal(parent.tracer, tracer)
	assert.Equal(child.tracer, tracer)
}

func TestTracerDisabled(t *testing.T) {
	assert := assert.New(t)

	// disable the tracer and be sure that the span is not added
	tracer := NewTracer()
	tracer.Disable()
	span := tracer.NewSpan("pylons.request", "pylons", "/")
	span.Finish()
	assert.Equal(tracer.buffer.Len(), 0)
}

func TestTracerEnabledAgain(t *testing.T) {
	assert := assert.New(t)

	// disable the tracer and enable it again
	tracer := NewTracer()
	tracer.Disable()
	preSpan := tracer.NewSpan("pylons.request", "pylons", "/")
	preSpan.Finish()
	tracer.Enable()
	postSpan := tracer.NewSpan("pylons.request", "pylons", "/")
	postSpan.Finish()
	assert.Equal(tracer.buffer.Len(), 1)
}

func TestTracerSampler(t *testing.T) {
	assert := assert.New(t)

	sampleRate := 0.5
	tracer := NewTracer()
	tracer.SetSampleRate(sampleRate)

	span := tracer.NewSpan("pylons.request", "pylons", "/")

	// The span might be sampled or not, we don't know, but at least it should have the sample rate metric
	assert.Equal(sampleRate, span.Metrics[SampleRateMetricKey])
}

func TestTracerEdgeSampler(t *testing.T) {
	assert := assert.New(t)

	// a sample rate of 0 should sample nothing
	tracer0 := NewTracer()
	tracer0.SetSampleRate(0)
	// a sample rate of 1 should sample everything
	tracer1 := NewTracer()
	tracer1.SetSampleRate(1)

	count := 10000

	for i := 0; i < count; i++ {
		span0 := tracer0.NewSpan("pylons.request", "pylons", "/")
		span0.Finish()
		span1 := tracer1.NewSpan("pylons.request", "pylons", "/")
		span1.Finish()
	}

	assert.Equal(0, tracer0.buffer.Len())
	assert.Equal(count, tracer1.buffer.Len())
}

// Mock Transport with a real Encoder
type DummyTransport struct {
	pool *encoderPool
}

func (t *DummyTransport) Send(spans []*Span) error {
	encoder := t.pool.Borrow()
	defer t.pool.Return(encoder)
	return encoder.Encode(spans)
}

func BenchmarkTracerAddSpans(b *testing.B) {
	// create a new tracer with a DummyTransport
	tracer := NewTracer()
	tracer.transport = &DummyTransport{pool: newEncoderPool(encoderPoolSize)}

	for n := 0; n < b.N; n++ {
		span := tracer.NewSpan("pylons.request", "pylons", "/")
		span.Finish()
	}
}
