package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// TestHandleDescribeReturnsContract is the smallest possible smoke test:
// the describe handler doesn't touch the filesystem, so it works anywhere
// and verifies the wiring (tool registration, return shape) without
// needing a real vault on disk.
func TestHandleDescribeReturnsContract(t *testing.T) {
	res, err := handleDescribe(context.Background(), mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleDescribe: %v", err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatal("describe returned empty result")
	}
	textContent, ok := res.Content[0].(mcpgo.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want mcpgo.TextContent", res.Content[0])
	}
	if !strings.Contains(textContent.Text, "Sparks") {
		t.Errorf("describe output missing 'Sparks' anchor: %q", textContent.Text[:min(200, len(textContent.Text))])
	}
}

// TestJSONOrErrorRoundTrips checks the marshalling helper. It's
// internally crucial — most handlers funnel through it.
func TestJSONOrErrorRoundTrips(t *testing.T) {
	type sample struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	res, err := jsonOrError(sample{A: "hi", B: 42}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Errorf("non-error result reported IsError=true")
	}
	textContent := res.Content[0].(mcpgo.TextContent)
	var got sample
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("output isn't JSON: %v", err)
	}
	if got.A != "hi" || got.B != 42 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
