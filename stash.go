package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type stashKey int

const stashKeyValue stashKey = 0

var ErrNoStash = errors.New("stash not found in context")
var ErrUnexpectedStashType = errors.New("unexpected stash type")
var ErrNilContext = errors.New("context is nil")

// SetStash sets the stash value in the context.
// It marshals the stash value to JSON and stores it in the context using a specific key.
// If the context is nil, it returns an error.
// If there is an error while marshalling the stash value, it returns an error with the specific error message.
// Otherwise, it returns the updated context with the stash value set.
func SetStash(ctx context.Context, stash any) (context.Context, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	stashStr, err := json.Marshal(stash)
	if err != nil {
		return nil, fmt.Errorf("error marshalling stash: %w", err)
	}

	ctx = context.WithValue(ctx, stashKeyValue, string(stashStr))

	return ctx, nil
}

// ParseStash parses the stash value from the context and unmarshals it into the provided value.
// It returns an error if the stash value is not found in the context or if it has an unexpected type.
func ParseStash(ctx context.Context, v any) error {
	stash := ctx.Value(stashKeyValue)
	if stash == nil {
		return ErrNoStash
	}

	stashStr, ok := stash.(string)
	if !ok {
		return ErrUnexpectedStashType
	}

	if err := json.Unmarshal([]byte(stashStr), v); err != nil {
		return fmt.Errorf("error unmarshalling stash: %w", err)
	}

	return nil
}

// getStash retrieves the serialized stash from the provided context.
// It returns the serialized stash as a byte slice and an error, if any.
func getStash(ctx context.Context) string {
	stash := ctx.Value(stashKeyValue)
	if stash == nil {
		return ""
	}

	serialized, ok := stash.(string)
	if !ok {
		panic(ErrUnexpectedStashType)
	}

	return serialized
}

// putStash puts the given stash into the context and returns the updated context.
// If the provided context is nil, it panics with ErrNilContext.
func putStash(ctx context.Context, stash string) context.Context {
	return context.WithValue(ctx, stashKeyValue, stash)
}
