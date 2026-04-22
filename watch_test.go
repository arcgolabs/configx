package configx_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// writeYAML writes content to path, failing the test on error.
func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// tempYAML creates a temp YAML config file and returns its path.
func tempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	writeYAML(t, path, content)
	return path
}

// startWatcher starts w.Start in a background goroutine with a cancellable
// context and registers t.Cleanup to cancel the context and close the watcher.
func startWatcher(t *testing.T, w *configx.Watcher) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	startErr := make(chan error, 1)
	t.Cleanup(func() {
		cancel()
		require.NoError(t, w.Close())
		select {
		case err := <-startErr:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watcher shutdown")
		}
	})
	go func() {
		startErr <- w.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	return cancel
}

// waitForChange waits up to timeout for the next value on ch.
func waitForChange(t *testing.T, ch <-chan *configx.Config, timeout time.Duration) *configx.Config {
	t.Helper()
	select {
	case cfg := <-ch:
		return cfg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for config reload")
		return nil
	}
}

// ── construction ──────────────────────────────────────────────────────────────

func TestNewWatcher_InitialLoad(t *testing.T) {
	path := tempYAML(t, "name: arcgo\nport: 8080\n")

	w, err := configx.NewWatcher(configx.WithFiles(path))
	require.NoError(t, err)

	assert.Equal(t, "arcgo", w.Config().GetString("name"))
	assert.Equal(t, 8080, w.Config().GetInt("port"))
}

func TestNewWatcher_WithDefaults(t *testing.T) {
	w, err := configx.NewWatcher(
		configx.WithDefaults(map[string]any{
			"name": "default-app",
			"port": 9090,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "default-app", w.Config().GetString("name"))
	assert.Equal(t, 9090, w.Config().GetInt("port"))
}

func TestNewWatcher_NoFiles_InitialConfigStillWorks(t *testing.T) {
	w, err := configx.NewWatcher(
		configx.WithDefaults(map[string]any{"key": "val"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "val", w.Config().GetString("key"))
}

func TestNewWatcher_BadFile_ReturnsError(t *testing.T) {
	_, err := configx.NewWatcher(configx.WithFiles("/nonexistent/path/config.yaml"))
	assert.Error(t, err)
}

// ── hot reload ────────────────────────────────────────────────────────────────

func TestWatcher_HotReload_ValueChanges(t *testing.T) {
	path := tempYAML(t, "name: before\nport: 1111\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	changed := make(chan *configx.Config, 1)
	w.OnChange(func(cfg *configx.Config, err error) {
		require.NoError(t, err)
		changed <- cfg
	})

	startWatcher(t, w)
	writeYAML(t, path, "name: after\nport: 2222\n")

	newCfg := waitForChange(t, changed, 3*time.Second)
	assert.Equal(t, "after", newCfg.GetString("name"))
	assert.Equal(t, 2222, newCfg.GetInt("port"))
	assert.Equal(t, "after", w.Config().GetString("name"))
}

func TestWatcher_HotReload_MultipleReloads(t *testing.T) {
	path := tempYAML(t, "version: 1\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	var reloadCount atomic.Int32
	w.OnChange(func(_ *configx.Config, err error) {
		if err == nil {
			reloadCount.Add(1)
		}
	})

	startWatcher(t, w)

	for i := range 3 {
		writeYAML(t, path, fmt.Sprintf("version: %d\n", i+2))
		time.Sleep(120 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	assert.EqualValues(t, 3, reloadCount.Load())
	assert.Equal(t, 4, w.Config().GetInt("version"))
}

// ── debounce ──────────────────────────────────────────────────────────────────

func TestWatcher_Debounce_CollapsesRapidWrites(t *testing.T) {
	path := tempYAML(t, "counter: 0\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(300*time.Millisecond),
	)
	require.NoError(t, err)

	var reloadCount atomic.Int32
	w.OnChange(func(cfg *configx.Config, _ error) {
		if cfg != nil {
			reloadCount.Add(1)
		}
	})

	startWatcher(t, w)

	for i := 1; i <= 5; i++ {
		writeYAML(t, path, fmt.Sprintf("counter: %d\n", i))
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	assert.EqualValues(t, 1, reloadCount.Load())
	assert.Equal(t, 5, w.Config().GetInt("counter"))
}
