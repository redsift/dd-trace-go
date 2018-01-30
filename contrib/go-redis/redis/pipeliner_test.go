package redis

import (
	"context"
	"testing"
	"time"

	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	opts := &redis.Options{Addr: "127.0.0.1:7006"}
	assert := assert.New(t)
	testTracer, testTransport := tracertest.GetTestTracer()
	testTracer.SetDebugLogging(debug)

	client := NewClientWithServiceName(opts, "my-redis", testTracer)
	pipeline := client.Pipeline()
	pipeline.Expire("pipeline_counter", time.Hour)

	// Exec with context test
	ExecWithContext(pipeline, context.Background())

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	span := spans[0]
	assert.Equal(span.Service, "my-redis")
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("out.port"), "7006")
	assert.Equal(span.GetMeta("redis.pipeline_length"), "1")
	assert.Equal(span.Resource, "expire pipeline_counter 3600: false\n")

	pipeline.Expire("pipeline_counter", time.Hour)
	pipeline.Expire("pipeline_counter_1", time.Minute)

	// Rewriting Exec
	pipeline.Exec()

	testTracer.ForceFlush()
	traces = testTransport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 1)

	span = spans[0]
	assert.Equal(span.Service, "my-redis")
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("redis.pipeline_length"), "2")
	assert.Equal(span.Resource, "expire pipeline_counter 3600: false\nexpire pipeline_counter_1 60: false\n")
}
