package configx_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_Close_StopsReloads(t *testing.T) {
	path := tempYAML(t, "active: true\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	var reloadCount atomic.Int32
	w.OnChange(func(cfg *configx.Config, _ error) {
		if cfg != nil {
			reloadCount.Add(1)
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	startErr := make(chan error, 1)
	go func() {
		close(started)
		startErr <- w.Start(ctx)
	}()

	<-started
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, w.Close())
	select {
	case err := <-startErr:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
	}

	writeYAML(t, path, "active: false\n")
	time.Sleep(200 * time.Millisecond)

	assert.EqualValues(t, 0, reloadCount.Load())
}

func TestWatcher_Close_IsIdempotent(t *testing.T) {
	w, err := configx.NewWatcher(configx.WithDefaults(map[string]any{"x": 1}))
	require.NoError(t, err)

	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
}

func TestWatcher_Start_ReturnsWhenContextCancelled(t *testing.T) {
	path := tempYAML(t, "x: 1\n")

	w, err := configx.NewWatcher(configx.WithFiles(path))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Start(ctx) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestWatcher_Start_NoFiles_ReturnsOnContextCancel(t *testing.T) {
	w, err := configx.NewWatcher(configx.WithDefaults(map[string]any{"k": "v"}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Start(ctx) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation (no files)")
	}
}

func TestWatcher_EnvSeparator_DoubleUnderscore(t *testing.T) {
	t.Setenv("APP_DB__HOST", "localhost")
	t.Setenv("APP_MAX_RETRY", "5")

	w, err := configx.NewWatcher(
		configx.WithEnvPrefix("APP"),
		configx.WithEnvSeparator("__"),
		configx.WithPriority(configx.SourceEnv),
	)
	require.NoError(t, err)

	assert.Equal(t, "localhost", w.Config().GetString("db.host"))
	assert.Equal(t, "5", w.Config().GetString("max_retry"))
}

func TestNewWatcher_UnsupportedFileFormat_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "config.ini")
	require.NoError(t, os.WriteFile(iniPath, []byte("[section]\nkey=value\n"), 0o600))

	_, err := configx.NewWatcher(configx.WithFiles(iniPath))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, configx.ErrUnsupportedFileFormat))
}

func TestWatcher_ConcurrentConfigReads(t *testing.T) {
	path := tempYAML(t, "counter: 0\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(20*time.Millisecond),
	)
	require.NoError(t, err)

	cancel := startWatcher(t, w)
	defer cancel()

	var wg sync.WaitGroup

	wg.Go(func() {
		for i := 1; i <= 5; i++ {
			writeYAML(t, path, fmt.Sprintf("counter: %d\n", i))
			time.Sleep(80 * time.Millisecond)
		}
	})

	for range 10 {
		wg.Go(func() {
			for range 20 {
				cfg := w.Config()
				_ = cfg.GetInt("counter")
				time.Sleep(5 * time.Millisecond)
			}
		})
	}

	wg.Wait()
}

func TestWatcherT_HotReload_TypedValueChanges(t *testing.T) {
	type typedCfg struct {
		Name string `validate:"required"`
		Port int    `validate:"gte=1"`
	}

	path := tempYAML(t, "name: before\nport: 1111\n")
	w, err := configx.NewWatcherT[typedCfg](
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	require.NoError(t, err)

	changed := make(chan typedCfg, 1)
	w.OnChange(func(cfg typedCfg, err error) {
		require.NoError(t, err)
		changed <- cfg
	})

	ctx, cancel := context.WithCancel(t.Context())
	startErr := make(chan error, 1)
	t.Cleanup(func() {
		cancel()
		require.NoError(t, w.Close())
		select {
		case err := <-startErr:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for typed watcher shutdown")
		}
	})
	go func() {
		startErr <- w.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	writeYAML(t, path, "name: after\nport: 2222\n")
	select {
	case got := <-changed:
		assert.Equal(t, "after", got.Name)
		assert.Equal(t, 2222, got.Port)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for typed watcher reload")
	}

	latest := w.Config()
	assert.Equal(t, "after", latest.Name)
	assert.Equal(t, 2222, latest.Port)
}
