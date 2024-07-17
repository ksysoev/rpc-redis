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

func SetStash(ctx context.Context, stash any) (context.Context, error) {
	ctx = context.WithValue(ctx, stashKeyValue, stash)

	return ctx, nil
}

func ParseStash(ctx context.Context, v any) error {
	stash, ok := ctx.Value(stashKeyValue).(any)
	if !ok {
		return ErrNoStash
	}

	stashStr, ok := stash.(string)
	if !ok {
		return ErrUnexpectedStashType
	}

	return json.Unmarshal([]byte(stashStr), v)
}

func serializeStash(ctx context.Context) (string, error) {
	stash := ctx.Value(stashKeyValue)
	if stash == nil {
		return "", ErrNoStash
	}

	serialized, err := json.Marshal(stash)
	if err != nil {
		return "", fmt.Errorf("error marshalling stash: %w", err)
	}

	return string(serialized), nil
}

func putStash(ctx context.Context, stash string) context.Context {
	return context.WithValue(ctx, stashKeyValue, stash)
}
