package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrScopeClosed is returned when work is registered on a scope that is
// closing or closed.
var ErrScopeClosed = errors.New("kernel: scope closed")

// ErrNotProvided is returned by Inject when no live ancestor provides the name.
var ErrNotProvided = errors.New("kernel: service not provided")

// ErrServiceType is returned by Inject when the service has another type.
var ErrServiceType = errors.New("kernel: service has a different type")

// ErrServiceExists is returned by Provide when the scope already provides the name.
var ErrServiceExists = errors.New("kernel: service already provided in this scope")

type effect struct {
	name string
	fn   func(ctx context.Context) error
}

// Scope is a node of the scope tree. Closing a scope cancels its context,
// closes its children, then runs its effects last-in first-out, awaiting each.
type Scope struct {
	name   string
	parent *Scope
	ctx    context.Context
	cancel context.CancelCauseFunc

	mu       sync.Mutex
	closed   bool
	children []*Scope
	effects  []effect
	services map[string]any
	done     chan struct{}
	result   error
}

// NewRoot makes the root of a scope tree.
func NewRoot(name string) *Scope {
	return newScope(name, nil, context.Background())
}

func newScope(name string, parent *Scope, base context.Context) *Scope {
	ctx, cancel := context.WithCancelCause(base)
	return &Scope{name: name, parent: parent, ctx: ctx, cancel: cancel,
		services: map[string]any{}, done: make(chan struct{})}
}

// Name is the scope's name.
func (s *Scope) Name() string { return s.name }

// Context is cancelled, with the close cause, when the scope or an ancestor closes.
func (s *Scope) Context() context.Context { return s.ctx }

// Child opens a child scope. It fails once the scope is closing.
func (s *Scope) Child(name string) (*Scope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrScopeClosed
	}
	c := newScope(name, s, s.ctx)
	s.children = append(s.children, c)
	return c, nil
}

// OnClose registers an effect run at close. Effects run last-in first-out and
// are awaited. The context given to fn is the closer's, not the scope's, so
// cleanup is not born cancelled.
func (s *Scope) OnClose(name string, fn func(ctx context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrScopeClosed
	}
	s.effects = append(s.effects, effect{name: name, fn: fn})
	return nil
}

// Close closes the scope with cause (nil means context.Canceled). The first
// caller runs the close; concurrent callers wait for it and share its result.
// ctx bounds how long each effect is awaited; an effect still running when ctx
// ends is reported as abandoned and Close moves on.
func (s *Scope) Close(ctx context.Context, cause error) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		select {
		case <-s.done:
			return s.result
		case <-ctx.Done():
			return fmt.Errorf("kernel: waiting for close of scope %q: %w", s.name, ctx.Err())
		}
	}
	s.closed = true
	children := s.children
	effects := s.effects
	s.children, s.effects = nil, nil
	s.mu.Unlock()

	if cause == nil {
		cause = context.Canceled
	}
	s.cancel(cause)

	var errs []error
	for i := len(children) - 1; i >= 0; i-- {
		if err := children[i].Close(ctx, cause); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(effects) - 1; i >= 0; i-- {
		if err := runEffect(ctx, s.name, effects[i]); err != nil {
			errs = append(errs, err)
		}
	}

	s.mu.Lock()
	s.services = map[string]any{}
	s.mu.Unlock()
	if s.parent != nil {
		s.parent.forget(s)
	}
	s.result = errors.Join(errs...)
	close(s.done)
	return s.result
}

func (s *Scope) forget(c *Scope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, x := range s.children {
		if x == c {
			s.children = append(s.children[:i:i], s.children[i+1:]...)
			return
		}
	}
}

func runEffect(ctx context.Context, scope string, e effect) error {
	res := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				res <- fmt.Errorf("kernel: effect %q of scope %q panicked: %v", e.name, scope, r)
			}
		}()
		if err := e.fn(ctx); err != nil {
			err = fmt.Errorf("kernel: effect %q of scope %q: %w", e.name, scope, err)
			res <- err
			return
		}
		res <- nil
	}()
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		return fmt.Errorf("kernel: effect %q of scope %q abandoned: %w", e.name, scope, ctx.Err())
	}
}

// Provide makes v available under name to this scope and its descendants,
// until the scope closes.
func Provide[T any](s *Scope, name string, v T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrScopeClosed
	}
	if _, ok := s.services[name]; ok {
		return fmt.Errorf("%w: %q in scope %q", ErrServiceExists, name, s.name)
	}
	s.services[name] = v
	return nil
}

// Inject resolves name from s or the nearest live ancestor providing it.
func Inject[T any](s *Scope, name string) (T, error) {
	var zero T
	for cur := s; cur != nil; cur = cur.parent {
		cur.mu.Lock()
		v, ok := cur.services[name]
		closed := cur.closed
		cur.mu.Unlock()
		if !ok || closed {
			continue
		}
		t, ok := v.(T)
		if !ok {
			return zero, fmt.Errorf("%w: %q", ErrServiceType, name)
		}
		return t, nil
	}
	return zero, fmt.Errorf("%w: %q", ErrNotProvided, name)
}
