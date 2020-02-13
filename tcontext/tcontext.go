package tcontext

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ContextKey string

const ContextID = ContextKey("id")

type Context struct {
	id  string
	ctx context.Context
}

func (c Context) ID() string {
	return c.id
}

func (c Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c Context) Err() error {
	return c.ctx.Err()
}

func (c Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

func New(id ...string) Context {
	if len(id) == 0 {
		newCtx := context.WithValue(context.Background(), ContextID, uuid.New().String())
		return Context{id: newCtx.Value(ContextID).(string), ctx: newCtx}
	}
	return Context{id: id[0], ctx: context.Background()}
}

func From(parent context.Context) Context {
	newCtx := context.WithValue(parent, ContextID, uuid.New().String())
	return Context{id: newCtx.Value(ContextID).(string), ctx: newCtx}
}

func WithID(parent context.Context, id string) Context {
	newCtx := context.WithValue(parent, ContextID, id)
	return Context{id: newCtx.Value(ContextID).(string), ctx: newCtx}
}

func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	newCtx := context.WithValue(parent, key, val)
	return Context{id: newCtx.Value(ContextID).(string), ctx: newCtx}
}

func WithCancel(parent context.Context, id string) (context.Context, context.CancelFunc) {
	newCtx, cancel := context.WithCancel(parent)
	return Context{id: id, ctx: newCtx}, cancel
}

func WithDeadline(parent context.Context, id string, deadline time.Time) (context.Context, context.CancelFunc) {
	newCtx, cancel := context.WithDeadline(parent, deadline)
	return Context{id: id, ctx: newCtx}, cancel
}

func WithTimeout(parent context.Context, id string, timeout time.Duration) (context.Context, context.CancelFunc) {
	newCtx, cancel := context.WithTimeout(parent, timeout)
	return Context{id: id, ctx: newCtx}, cancel
}
