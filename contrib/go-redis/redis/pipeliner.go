package redis

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/go-redis/redis"
)

var _ redis.Pipeliner = (*Pipeliner)(nil)

// Pipeline is used to trace pipelines executed on a Redis server.
type Pipeliner struct {
	redis.Pipeliner
	*params
}

// Exec calls Pipeline.Exec() ensuring that the resulting Redis calls are traced.
func (c *Pipeliner) Exec() ([]redis.Cmder, error) {
	params := c.params
	span := params.tracer.NewRootSpan("redis.command", params.service, "redis")
	defer span.Finish()
	if params.host != "" {
		span.SetMeta("out.host", params.host)
	}
	if params.port != "" {
		span.SetMeta("out.port", params.port)
	}
	if params.db != "" {
		span.SetMeta("out.db", params.db)
	}
	cmds, err := c.Pipeliner.Exec()
	if err != nil {
		span.SetError(err)
	}
	span.Resource = commandsToString(cmds)
	span.SetMeta("redis.pipeline_length", strconv.Itoa(len(cmds)))

	return cmds, err
}

// ExecWithContext calls Pipeline.Exec(). It ensures that the resulting Redis calls
// are traced, and that emitted spans are children of the given Context.
func ExecWithContext(p redis.Pipeliner, ctx context.Context) ([]redis.Cmder, error) {
	c, ok := p.(*Pipeliner)
	if !ok {
		return nil, errors.New("pipeliner is not of type *redistrace.Pipeliner")
	}
	params := c.params
	span := params.tracer.NewChildSpanFromContext("redis.command", ctx)
	defer span.Finish()
	span.Service = params.service
	if params.host != "" {
		span.SetMeta("out.host", params.host)
	}
	if params.port != "" {
		span.SetMeta("out.port", params.port)
	}
	if params.db != "" {
		span.SetMeta("out.db", params.db)
	}
	cmds, err := c.Pipeliner.Exec()
	if err != nil {
		span.SetError(err)
	}
	span.Resource = commandsToString(cmds)
	span.SetMeta("redis.pipeline_length", strconv.Itoa(len(cmds)))

	return cmds, err
}

// commandsToString returns a string representation of a slice of redis Commands, separated by newlines.
func commandsToString(cmds []redis.Cmder) string {
	var b bytes.Buffer
	for _, cmd := range cmds {
		b.WriteString(cmd.String())
		b.WriteString("\n")
	}
	return b.String()
}
