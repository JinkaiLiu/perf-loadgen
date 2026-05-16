package engine

import (
	"context"
	"testing"
	"time"
)

func TestQPSPacerSpacesRequests(t *testing.T) {
	t.Parallel()

	pacer := newPacer(20)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	if err := pacer.Wait(ctx); err != nil {
		t.Fatalf("first wait returned error: %v", err)
	}
	if err := pacer.Wait(ctx); err != nil {
		t.Fatalf("second wait returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 45*time.Millisecond {
		t.Fatalf("expected paced delay, got %s", elapsed)
	}
}
