package client

import (
	"context"
	"testing"
)

func TestRange(t *testing.T) {
	c, err := NewClient("https://httpbin.org/range/1024")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	supported, err := c.CheckServerRangeRequest(ctx)
	if !supported {
		t.Errorf("expected true, got false")
	}
}
