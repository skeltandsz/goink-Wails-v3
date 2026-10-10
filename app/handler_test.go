package app

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	a := New(logger)
	require.NotNil(t, a)
	assert.NotNil(t, a.logger, "logger should be set by New")
}

// ---------------------------------------------------------------------------
// IsInitialized
// ---------------------------------------------------------------------------

func TestIsInitialized_BeforeStartup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	a := New(logger)
	assert.False(t, a.IsInitialized(), "fresh App should not be initialized before startup/setup")
}

func TestIsInitialized_AfterSetup(t *testing.T) {
	a := setupTestApp(t)
	assert.True(t, a.IsInitialized(), "setupTestApp should set cfg so IsInitialized returns true")
}

// ---------------------------------------------------------------------------
// ServiceShutdown
// ---------------------------------------------------------------------------

func TestServiceShutdown(t *testing.T) {
	a := setupTestApp(t)

	// ServiceShutdown should not panic even on a fully initialized app.
	assert.NotPanics(t, func() {
		err := a.ServiceShutdown()
		assert.NoError(t, err)
	}, "ServiceShutdown should not panic on a fully initialized app")
}
