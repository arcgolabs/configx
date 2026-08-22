package configx

import (
	"slices"
	"testing"

	"github.com/knadh/koanf/v2"
)

func TestConfigGetIntSliceReturnsCopy(t *testing.T) {
	k := koanf.New(".")
	want := []int{1, 2}
	if err := k.Set("ints", want); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{k: k}

	got := cfg.GetIntSlice("ints")
	got[0] = 99

	if current := cfg.GetIntSlice("ints"); !slices.Equal(current, want) {
		t.Fatalf("GetIntSlice returned mutable config storage: got %v, want %v", current, want)
	}
}
