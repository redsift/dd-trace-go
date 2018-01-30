package redis

import (
	"net"
	"strconv"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-redis/redis"
)

func NewRing(opts *redis.RingOptions) *Ring {
	return NewRingWithServiceName(opts, "redis.client")
}

func NewRingWithServiceName(opts *redis.RingOptions, service string) *Ring {
	t := tracer.DefaultTracer
	r := redis.NewRing(opts)
	r.ForEachShard(func(c *redis.Client) error {
		opt := c.Options()
		host, port, err := net.SplitHostPort(opt.Addr)
		if err != nil {
			host = opt.Addr
			port = "6379"
		}
		params := &params{
			host:    host,
			port:    port,
			db:      strconv.Itoa(opt.DB),
			service: service,
			tracer:  t,
		}
		c.WrapProcess(createWrapperFromClient(&Client{c, params}))
		return nil
	})
	t.SetServiceInfo(service, "redis", ext.AppTypeDB)
	return &Ring{Ring: r, params: &params{service: service, tracer: t}}
}

type Ring struct {
	*redis.Ring
	*params
}

// Pipeline creates a Pipeline from a Ring.
func (c *Ring) Pipeline() redis.Pipeliner {
	// TODO(gbbr): Find a way to track selected shard.
	return &Pipeliner{c.Ring.Pipeline(), c.params}
}

// TODO(gbbr): Use WrapProcess and WrapProcessPipeline
