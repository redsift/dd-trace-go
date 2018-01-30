package redis

import (
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/go-redis/redis"
)

func NewUniversalClient(opts *redis.UniversalOptions) redis.UniversalClient {
	return NewUniversalClientWithServiceName(opts, "redis.client")
}

func NewUniversalClientWithServiceName(opts *redis.UniversalOptions, service string) redis.UniversalClient {
	c := redis.NewUniversalClient(opts)
	switch v := c.(type) {
	case *redis.Client:
		return WrapClient(v, service, tracer.DefaultTracer)
	case *redis.ClusterClient:
		return WrapClusterClient(v, service)
	default:
		panic("unsupported client")
	}
}
