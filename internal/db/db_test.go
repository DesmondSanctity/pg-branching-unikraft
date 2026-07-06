package db

import (
	"context"
	"errors"
	"testing"
)

func TestPool_NoDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Pool(context.Background()); !errors.Is(err, ErrNoDSN) {
		t.Fatalf("err = %v, want ErrNoDSN", err)
	}
}
