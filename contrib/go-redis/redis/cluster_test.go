package redis

import (
	"log"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
)

func TestClusterClient(t *testing.T) {
	assert := assert.New(t)
	originalTracer := tracer.DefaultTracer
	testTracer, transport := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer
	defer func() {
		tracer.DefaultTracer = originalTracer
	}()

	c := NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{
			":7000",
			":7001",
			":7002",
		},
	})
	defer c.Close()

	_, err := c.Set("test_key", "test_value", 0).Result()
	assert.Nil(err)
	testTracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	log.Printf("%v", spans[0])
}
