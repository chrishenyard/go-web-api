package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestLevelFilterHandler_Enabled(t *testing.T) {
	h := &levelFilterHandler{
		Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}),
		level:   slog.LevelWarn,
	}

	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("expected warn to be enabled")
	}

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected info to be filtered out")
	}
}
