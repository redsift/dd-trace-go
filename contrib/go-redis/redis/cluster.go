package redis

import (
	"context"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-redis/redis"
)

// Only supported since go-redis/redis@v6.8.3
func NewClusterClient(opts *redis.ClusterOptions) *ClusterClient {
	return NewClusterClientWithServiceName(opts, "redis.cluster-client")
}

// Only supported since go-redis/redis@v6.8.3
func NewClusterClientWithServiceName(opts *redis.ClusterOptions, service string) *ClusterClient {
	return WrapClusterClient(redis.NewClusterClient(opts), "redis.cluster-client")
}

// Only supported since go-redis/redis@v6.8.3
func WrapClusterClient(client *redis.ClusterClient, service string) *ClusterClient {
	t := tracer.DefaultTracer
	cluster := &ClusterClient{
		ClusterClient: client,
		params:        &params{service: service, tracer: t},
	}
	cluster.ForEachNode(func(c *redis.Client) error {
		c.WrapProcess(func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
			return func(cmd redis.Cmder) error {
				err := oldProcess(cmd)
				log.Printf("POST-WRAP: %s|%s|%#v|%#v\n", c.Options().Addr, cmd, err, cmd.Err())
				if err == nil {
					cluster.lastSuccessfulCall(c)
				}
				return err
			}
		})
		return nil
	})
	cluster.WrapProcess(func(oldProcess func(redis.Cmder) error) func(redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			cluster.lastSuccessfulCall(nil)

			raw := cmd.String()
			parts := strings.Split(raw, " ")
			length := len(parts) - 1
			p := cluster.params

			// TODO(gbbr): Change to use cluster context after (and "if") it gets added:
			// https://github.com/go-redis/redis/issues/700
			span := p.tracer.NewChildSpanFromContext("redis.command", context.Background())
			defer span.Finish()
			span.Service = p.service
			span.Resource = parts[0]
			span.SetMeta("redis.raw_command", raw)
			span.SetMeta("redis.args_length", strconv.Itoa(length))

			cluster.mu.RLock()
			if p.host != "" || p.port != "" {
				span.SetMeta("out.host", p.host)
				span.SetMeta("out.port", p.port)
			}
			if p.db != "" {
				span.SetMeta("out.db", p.db)
			}
			cluster.mu.RUnlock()

			err := oldProcess(cmd)
			if err != nil {
				span.SetError(err)
			}
			return err
		}
	})
	t.SetServiceInfo(service, "redis", ext.AppTypeDB)
	return cluster
}

type ClusterClient struct {
	*redis.ClusterClient

	mu sync.RWMutex // guards params (host, port and db)
	*params
}

func (c *ClusterClient) lastSuccessfulCall(client *redis.Client) {
	var host, port, db string
	if client != nil {
		var err error
		opt := client.Options()
		host, port, err = net.SplitHostPort(opt.Addr)
		if err != nil {
			host = opt.Addr
			port = "6379"
		}
		db = strconv.Itoa(opt.DB)
	}
	c.mu.Lock()
	c.params.host, c.params.port, c.params.db = host, port, db
	defer c.mu.Unlock()
}

// TODO(gbbr): Pipeline
