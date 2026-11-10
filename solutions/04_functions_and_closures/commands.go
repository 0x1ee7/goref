package funcs

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

var (
	// ErrInvalidCommand is returned for an empty name or a nil command.
	ErrInvalidCommand = errors.New("funcs: invalid command")
	// ErrDuplicateCommand is returned when a name is registered twice.
	ErrDuplicateCommand = errors.New("funcs: duplicate command")
	// ErrUnknownCommand is returned by Run for a name that was never registered.
	ErrUnknownCommand = errors.New("funcs: unknown command")
)

// Command is a named operation that receives its arguments as strings.
type Command func(ctx context.Context, args []string) error

// Registry maps command names to functions. The zero value is ready to use and
// safe for concurrent use.
type Registry struct {
	mu   sync.RWMutex
	cmds map[string]Command
}

// Register adds cmd under name. It rejects an empty name or nil cmd with
// ErrInvalidCommand, and a repeated name with ErrDuplicateCommand.
func (r *Registry) Register(name string, cmd Command) error {
	if name == "" || cmd == nil {
		return fmt.Errorf("register %q: %w", name, ErrInvalidCommand)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.cmds[name]; ok {
		return fmt.Errorf("register %q: %w", name, ErrDuplicateCommand)
	}
	if r.cmds == nil {
		r.cmds = make(map[string]Command)
	}
	r.cmds[name] = cmd
	return nil
}

// Run calls the command registered under name with args. An unknown name
// returns ErrUnknownCommand. Otherwise the command's error is returned as is.
func (r *Registry) Run(ctx context.Context, name string, args []string) error {
	r.mu.RLock()
	cmd, ok := r.cmds[name]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("run %q: %w", name, ErrUnknownCommand)
	}
	return cmd(ctx, args)
}

// Names returns the registered command names in sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.cmds))
}
