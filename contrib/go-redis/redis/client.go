// Package redis provides tracing functions for tracing the go-redis/redis package (https://github.com/go-redis/redis).
package redis

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/go-redis/redis"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
)

var _ redis.UniversalClient = (*Client)(nil)

// Client is used to trace requests to a redis server.
type Client struct {
	*redis.Client
	*params
}

// params holds the tracer and a set of parameters used as context when creating spans.
type params struct {
	host    string
	port    string
	db      string
	service string
	tracer  *tracer.Tracer
}

// NewClient returns a new Client that is traced with the default tracer under
// the service name "redis".
func NewClient(opt *redis.Options) *Client {
	return NewClientWithServiceName(opt, "redis.client", tracer.DefaultTracer)
}

// NewClientWithServiceName returns a new Client that is traced using the given tracer and service name.
// If nil is provided as a tracer, the global tracer will be used.
//
// TODO(gbbr): Remove tracer argument when we switch to OpenTracing.
func NewClientWithServiceName(opt *redis.Options, service string, t *tracer.Tracer) *Client {
	return WrapClient(redis.NewClient(opt), service, t)
}

// WrapClient wraps a given redis.Client with a tracer under the given service name.
//
// TODO(gbbr): Remove tracer argument when we switch to OpenTracing.
func WrapClient(c *redis.Client, service string, t *tracer.Tracer) *Client {
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
	t.SetServiceInfo(service, "redis", ext.AppTypeDB)
	tc := &Client{c, params}
	tc.Client.WrapProcess(createWrapperFromClient(tc))
	return tc
}

// Pipeline creates a Pipeline from a Client
func (c *Client) Pipeline() redis.Pipeliner {
	return &Pipeliner{c.Client.Pipeline(), c.params}
}

// SetContext sets a context on a Client. Use it to ensure that emitted spans have the correct parent.
func (c *Client) WithContext(ctx context.Context) *Client {
	c.Client = c.Client.WithContext(ctx)
	return c
}

// createWrapperFromClient returns a new createWrapper function which wraps the processor with tracing
// information obtained from the provided Client. To understand this functionality better see the
// documentation for the github.com/go-redis/redis.(*baseClient).WrapProcess function.
func createWrapperFromClient(tc *Client) func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
	return func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			ctx := tc.Client.Context()
			raw := cmd.String()
			parts := strings.Split(raw, " ")
			length := len(parts) - 1
			p := tc.params

			span := p.tracer.NewChildSpanFromContext("redis.command", ctx)
			defer span.Finish()
			span.Service = p.service
			span.Resource = parts[0]
			span.SetMeta("redis.raw_command", raw)
			span.SetMeta("redis.args_length", strconv.Itoa(length))
			if p.host != "" {
				span.SetMeta("out.host", p.host)
			}
			if p.port != "" {
				span.SetMeta("out.port", p.port)
			}
			span.SetMeta("out.db", p.db)

			err := oldProcess(cmd)
			if err != nil {
				span.SetError(err)
			}
			return err
		}
	}
}
