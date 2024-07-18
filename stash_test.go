package rpc

import (
	"context"
	"fmt"
	"testing"
)

func TestPutStash(t *testing.T) {
	ctx := context.Background()
	stash := "test stash"

	updatedCtx := putStash(ctx, stash)

	value := updatedCtx.Value(stashKeyValue)
	if value == nil {
		t.Errorf("Expected stash value to be set in the context, but got nil")
	}

	serialized, ok := value.(string)
	if !ok {
		t.Errorf("Expected stash value to be of type string, but got %T", value)
	}

	if serialized != stash {
		t.Errorf("Expected stash value to be %q, but got %q", stash, serialized)
	}
}

func TestGetStash(t *testing.T) {
	ctx := context.Background()
	stash := "test stash"
	ctx = putStash(ctx, stash)

	serialized := getStash(ctx)

	if serialized != stash {
		t.Errorf("Expected stash value to be %q, but got %q", stash, serialized)
	}
}

func TestParseStash(t *testing.T) {
	ctx := context.Background()
	stash := "test stash"
	ctx = putStash(ctx, fmt.Sprintf("%q", stash))

	var parsedStash string
	err := ParseStash(ctx, &parsedStash)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if parsedStash != stash {
		t.Errorf("Expected parsed stash to be %q, but got %q", stash, parsedStash)
	}
}

func TestParseStash_NotFound(t *testing.T) {
	ctx := context.Background()

	var parsedStash string
	err := ParseStash(ctx, &parsedStash)

	if err != ErrNoStash {
		t.Errorf("Expected error to be %v, but got %v", ErrNoStash, err)
	}
}

func TestParseStash_UnexpectedType(t *testing.T) {
	ctx := context.Background()
	stash := "{dsfdsf"
	ctx = putStash(ctx, stash)

	var parsedStash string
	err := ParseStash(ctx, &parsedStash)

	if err == nil {
		t.Errorf("Expected error, but got nil")
	}
}

func TestSetStash(t *testing.T) {
	ctx := context.Background()
	stash := "test stash"

	updatedCtx, err := SetStash(ctx, stash)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	value := getStash(updatedCtx)
	if value == "" {
		t.Errorf("Expected stash value to be set in the context, but got nil")
	}

	if value != fmt.Sprintf("%q", stash) {
		t.Errorf("Expected stash value to be %q, but got %q", stash, value)
	}
}

func TestSetStash_NilContext(t *testing.T) {
	var ctx context.Context
	stash := "test stash"

	_, err := SetStash(ctx, stash)
	if err != ErrNilContext {
		t.Errorf("Expected error to be %v, but got %v", ErrNilContext, err)
	}
}

func TestSetStash_MarshallingError(t *testing.T) {
	ctx := context.Background()
	stash := make(chan int) // Invalid type for JSON marshalling

	_, err := SetStash(ctx, stash)
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}
}
