package configx_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_OnChange_NilHandlerIsIgnored(t *testing.T) {
	path := tempYAML(t, "x: 1\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	w.OnChange(nil)
	changed := make(chan int, 1)
	w.OnChange(func(cfg *configx.Config, err error) {
		if err == nil {
			changed <- cfg.GetInt("x")
		}
	})

	startWatcher(t, w)
	writeYAML(t, path, "x: 2\n")

	select {
	case got := <-changed:
		assert.Equal(t, 2, got)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for config reload")
	}
}

func TestWatcher_OnChange_MultipleSubscribers(t *testing.T) {
	path := tempYAML(t, "val: 10\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	const n = 3
	channels := make([]chan int, n)
	for i := range channels {
		ch := make(chan int, 1)
		channels[i] = ch
		capturedCh := ch
		w.OnChange(func(cfg *configx.Config, err error) {
			if err == nil {
				capturedCh <- cfg.GetInt("val")
			}
		})
	}

	startWatcher(t, w)
	writeYAML(t, path, "val: 99\n")

	for i, ch := range channels {
		select {
		case got := <-ch:
			assert.Equal(t, 99, got, "subscriber %d", i)
		case <-time.After(3 * time.Second):
			t.Fatalf("subscriber %d timed out", i)
		}
	}
}

func TestWatcher_OnChange_OrderIsPreserved(t *testing.T) {
	path := tempYAML(t, "n: 0\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	var mu sync.Mutex
	order := make([]int, 0, 3)

	for i := range 3 {
		w.OnChange(func(_ *configx.Config, _ error) {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		})
	}

	startWatcher(t, w)
	writeYAML(t, path, "n: 1\n")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{0, 1, 2}, order)
}

func TestWatcher_OnChange_RegisterDuringNotify_AppliesOnNextNotify(t *testing.T) {
	path := tempYAML(t, "n: 1\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	registered := false
	lateCalls := make(chan int, 1)
	var registerOnce sync.Once

	w.OnChange(func(_ *configx.Config, err error) {
		if err != nil {
			return
		}
		registerOnce.Do(func() {
			w.OnChange(func(cfg *configx.Config, err error) {
				if err == nil {
					lateCalls <- cfg.GetInt("n")
				}
			})
			registered = true
		})
	})

	startWatcher(t, w)
	writeYAML(t, path, "n: 2\n")
	time.Sleep(200 * time.Millisecond)

	assert.True(t, registered)
	select {
	case got := <-lateCalls:
		t.Fatalf("late subscriber should not run on the first notify, got %d", got)
	default:
	}

	writeYAML(t, path, "n: 3\n")
	select {
	case got := <-lateCalls:
		assert.Equal(t, 3, got)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for late subscriber")
	}
}

func TestWatcher_WatchErrHandler_CalledOnReloadError(t *testing.T) {
	path := tempYAML(t, "ok: true\n")

	var handledErr atomic.Value
	errCh := make(chan error, 1)

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
		configx.WithWatchErrHandler(func(e error) {
			if handledErr.CompareAndSwap(nil, e) {
				errCh <- e
			}
		}),
	)
	require.NoError(t, err)

	startWatcher(t, w)
	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid: yaml: [\n"), 0o600))

	select {
	case e := <-errCh:
		assert.Error(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for watch error handler")
	}

	assert.True(t, w.Config().GetBool("ok"))
}

func TestWatcher_OnChange_CalledWithErrorOnReloadFailure(t *testing.T) {
	path := tempYAML(t, "healthy: true\n")

	w, err := configx.NewWatcher(
		configx.WithFiles(path),
		configx.WithWatchDebounce(30*time.Millisecond),
	)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	w.OnChange(func(_ *configx.Config, err error) {
		if err != nil {
			errCh <- err
		}
	})

	startWatcher(t, w)
	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid: yaml: [\n"), 0o600))

	select {
	case e := <-errCh:
		assert.Error(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnChange error callback")
	}
}
