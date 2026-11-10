package funcs

import (
	"context"
	"errors"
)

var (
	// ErrInvalidCommand is returned by Register for an empty name or nil Command.
	ErrInvalidCommand = errors.New("funcs: invalid command")
	// ErrDuplicateCommand is returned by Register when the name is taken.
	ErrDuplicateCommand = errors.New("funcs: duplicate command")
	// ErrUnknownCommand is returned by Run for an unregistered name.
	ErrUnknownCommand = errors.New("funcs: unknown command")
)

// Command is a CLI-style command. Its error is returned to the caller of Run.
type Command func(ctx context.Context, args []string) error

// Registry maps command names to Commands. The zero value is ready to use and
// each Registry is independent.
type Registry struct{}

// Register adds cmd under name. Errors wrap ErrInvalidCommand or
// ErrDuplicateCommand.
func (r *Registry) Register(name string, cmd Command) error { return nil }

// Run calls the command registered under name with args and returns its error
// unchanged. An unknown name returns an error wrapping ErrUnknownCommand.
func (r *Registry) Run(ctx context.Context, name string, args []string) error { return nil }

// Names returns the registered names, sorted.
func (r *Registry) Names() []string { return nil }
