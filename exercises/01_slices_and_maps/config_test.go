package slicesmaps_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/01_slices_and_maps"
)

func TestPart6DeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		want     map[string]any
	}{
		{
			name:     "recursive merge keeps siblings",
			base:     map[string]any{"db": map[string]any{"host": "a", "pool": map[string]any{"size": 5}}},
			override: map[string]any{"db": map[string]any{"pool": map[string]any{"size": 10}}},
			want:     map[string]any{"db": map[string]any{"host": "a", "pool": map[string]any{"size": 10}}},
		},
		{
			name:     "lists are replaced",
			base:     map[string]any{"tags": []any{"a", "b"}},
			override: map[string]any{"tags": []any{"c"}},
			want:     map[string]any{"tags": []any{"c"}},
		},
		{
			name:     "nil override wins",
			base:     map[string]any{"x": 1},
			override: map[string]any{"x": nil},
			want:     map[string]any{"x": nil},
		},
		{
			name:     "mapping replaces scalar",
			base:     map[string]any{"x": 1},
			override: map[string]any{"x": map[string]any{"a": 1}},
			want:     map[string]any{"x": map[string]any{"a": 1}},
		},
		{
			name:     "scalar replaces mapping",
			base:     map[string]any{"x": map[string]any{"a": 1}},
			override: map[string]any{"x": 2},
			want:     map[string]any{"x": 2},
		},
		{
			name:     "empty override keeps base",
			base:     map[string]any{"x": 1, "y": map[string]any{"z": "w"}},
			override: map[string]any{},
			want:     map[string]any{"x": 1, "y": map[string]any{"z": "w"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slicesmaps.DeepMerge(tc.base, tc.override)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("DeepMerge mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart6DeepMergeDoesNotAlias(t *testing.T) {
	newBase := func() map[string]any {
		return map[string]any{"db": map[string]any{"host": "a"}, "tags": []any{"x"}}
	}
	newOverride := func() map[string]any {
		return map[string]any{"cache": map[string]any{"ttl": 1}}
	}
	base, override := newBase(), newOverride()
	got := slicesmaps.DeepMerge(base, override)

	gdb, ok := got["db"].(map[string]any)
	if !ok {
		t.Fatalf("got[db] = %T, want map[string]any", got["db"])
	}
	gdb["host"] = "mutated"
	gtags, ok := got["tags"].([]any)
	if !ok {
		t.Fatalf("got[tags] = %T, want []any", got["tags"])
	}
	gtags[0] = "mutated"
	gcache, ok := got["cache"].(map[string]any)
	if !ok {
		t.Fatalf("got[cache] = %T, want map[string]any", got["cache"])
	}
	gcache["ttl"] = 99

	if diff := cmp.Diff(newBase(), base); diff != "" {
		t.Fatalf("DeepMerge mutated or aliased base (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(newOverride(), override); diff != "" {
		t.Fatalf("DeepMerge mutated or aliased override (-want +got):\n%s", diff)
	}
}

func TestPart6ParseEnv(t *testing.T) {
	t.Run("nests double-underscore segments and lowercases", func(t *testing.T) {
		got, err := slicesmaps.ParseEnv(map[string]string{"APP_DB__POOL__SIZE": "10"}, "APP")
		if err != nil {
			t.Fatalf("ParseEnv: %v", err)
		}
		want := map[string]any{"db": map[string]any{"pool": map[string]any{"size": 10}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("ParseEnv mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("ignores variables outside the prefix", func(t *testing.T) {
		env := map[string]string{"HOME": "/root", "APP_NAME": "svc", "APPX_NAME": "no", "APP": "bare"}
		got, err := slicesmaps.ParseEnv(env, "APP")
		if err != nil {
			t.Fatalf("ParseEnv: %v", err)
		}
		want := map[string]any{"name": "svc"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("ParseEnv mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("empty environment", func(t *testing.T) {
		got, err := slicesmaps.ParseEnv(map[string]string{}, "APP")
		if err != nil {
			t.Fatalf("ParseEnv: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("ParseEnv(empty) = %v, want empty", got)
		}
	})

	coercions := []struct {
		raw  string
		want any
	}{
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"-7", -7},
		{"0", 0},
		{"007", 7},
		{"+5", "+5"},
		{"1.5", "1.5"},
		{"", ""},
		{"-", "-"},
		{"99999999999999999999999", "99999999999999999999999"},
		{"hello", "hello"},
	}
	for _, tc := range coercions {
		t.Run(fmt.Sprintf("value %q", tc.raw), func(t *testing.T) {
			got, err := slicesmaps.ParseEnv(map[string]string{"APP_V": tc.raw}, "APP")
			if err != nil {
				t.Fatalf("ParseEnv: %v", err)
			}
			want := map[string]any{"v": tc.want}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("ParseEnv coercion mismatch (-want +got):\n%s", diff)
			}
		})
	}

	errCases := []struct {
		name string
		env  map[string]string
	}{
		{"empty path after prefix", map[string]string{"APP_": "x"}},
		{"empty segment", map[string]string{"APP_DB____HOST": "x"}},
		{"key is both value and branch", map[string]string{"APP_DB": "1", "APP_DB__HOST": "x"}},
		{"case-insensitive collision", map[string]string{"APP_HOST": "a", "APP_host": "b"}},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := slicesmaps.ParseEnv(tc.env, "APP"); !errors.Is(err, slicesmaps.ErrInvalidEnv) {
				t.Fatalf("ParseEnv error = %v, want ErrInvalidEnv", err)
			}
		})
	}
}

func TestPart6Config(t *testing.T) {
	layers := func() slicesmaps.Layers {
		return slicesmaps.Layers{
			Defaults: map[string]any{
				"db":     map[string]any{"host": "localhost", "pool": map[string]any{"size": 5}},
				"debug":  false,
				"region": "eu",
			},
			File: map[string]any{
				"db":    map[string]any{"host": "file-host"},
				"debug": true,
			},
			Env: map[string]any{
				"db": map[string]any{"pool": map[string]any{"size": 10}},
			},
			CLI: map[string]any{"region": "us"},
		}
	}
	mergedWant := map[string]any{
		"db":     map[string]any{"host": "file-host", "pool": map[string]any{"size": 10}},
		"debug":  true,
		"region": "us",
	}

	t.Run("highest layer containing the path wins", func(t *testing.T) {
		c := slicesmaps.NewConfig(layers())
		tests := []struct {
			path   string
			want   any
			source string
		}{
			{"db.host", "file-host", "file"},
			{"db.pool.size", 10, "env"},
			{"debug", true, "file"},
			{"region", "us", "cli"},
		}
		for _, tc := range tests {
			t.Run(tc.path, func(t *testing.T) {
				v, ok := c.Get(tc.path)
				if !ok {
					t.Fatalf("Get(%q) absent, want present", tc.path)
				}
				if diff := cmp.Diff(tc.want, v); diff != "" {
					t.Fatalf("Get(%q) mismatch (-want +got):\n%s", tc.path, diff)
				}
				src, ok := c.Source(tc.path)
				if !ok || src != tc.source {
					t.Fatalf("Source(%q) = %q, %v; want %q, true", tc.path, src, ok, tc.source)
				}
			})
		}
	})
	t.Run("missing path is absent", func(t *testing.T) {
		c := slicesmaps.NewConfig(layers())
		if v, ok := c.Get("region"); !ok || v != "us" {
			t.Fatalf("control: Get(region) = %v, %v; want us, true", v, ok)
		}
		if _, ok := c.Get("db.nope"); ok {
			t.Fatal("Get(db.nope) present, want absent")
		}
		if _, ok := c.Source("nope"); ok {
			t.Fatal("Source(nope) present, want absent")
		}
	})
	t.Run("nil value is present", func(t *testing.T) {
		c := slicesmaps.NewConfig(slicesmaps.Layers{
			Defaults: map[string]any{"proxy": "http://p"},
			CLI:      map[string]any{"proxy": nil},
		})
		v, ok := c.Get("proxy")
		if !ok || v != nil {
			t.Fatalf("Get(proxy) = %v, %v; want nil, true", v, ok)
		}
		if src, ok := c.Source("proxy"); !ok || src != "cli" {
			t.Fatalf("Source(proxy) = %q, %v; want cli, true", src, ok)
		}
	})
	t.Run("path through a scalar is absent", func(t *testing.T) {
		c := slicesmaps.NewConfig(slicesmaps.Layers{
			Defaults: map[string]any{"db": "plain"},
		})
		if v, ok := c.Get("db"); !ok || v != "plain" {
			t.Fatalf("control: Get(db) = %v, %v; want plain, true", v, ok)
		}
		if _, ok := c.Get("db.host"); ok {
			t.Fatal("Get(db.host) present through a scalar, want absent")
		}
	})
	t.Run("layers are snapshotted", func(t *testing.T) {
		defaults := map[string]any{"region": "eu", "db": map[string]any{"host": "h"}}
		c := slicesmaps.NewConfig(slicesmaps.Layers{Defaults: defaults})
		if v, ok := c.Get("region"); !ok || v != "eu" {
			t.Fatalf("control: Get(region) = %v, %v; want eu, true", v, ok)
		}
		defaults["region"] = "changed"
		defaults["db"].(map[string]any)["host"] = "changed"
		if v, ok := c.Get("region"); !ok || v != "eu" {
			t.Fatalf("Get(region) after input mutation = %v, %v; want eu, true", v, ok)
		}
		if v, ok := c.Get("db.host"); !ok || v != "h" {
			t.Fatalf("Get(db.host) after input mutation = %v, %v; want h, true", v, ok)
		}
	})
	t.Run("nil layers are empty", func(t *testing.T) {
		c := slicesmaps.NewConfig(slicesmaps.Layers{Defaults: map[string]any{"a": 1}})
		if v, ok := c.Get("a"); !ok || v != 1 {
			t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
		}
		if diff := cmp.Diff(map[string]any{"a": 1}, c.AsMap()); diff != "" {
			t.Fatalf("AsMap mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("AsMap is merged", func(t *testing.T) {
		c := slicesmaps.NewConfig(layers())
		if diff := cmp.Diff(mergedWant, c.AsMap()); diff != "" {
			t.Fatalf("AsMap mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("AsMap is a fresh copy", func(t *testing.T) {
		c := slicesmaps.NewConfig(layers())
		if diff := cmp.Diff(mergedWant, c.AsMap()); diff != "" {
			t.Fatalf("control: AsMap mismatch (-want +got):\n%s", diff)
		}
		m := c.AsMap()
		m["region"] = "x"
		if db, ok := m["db"].(map[string]any); ok {
			db["host"] = "x"
		}
		if v, ok := c.Get("region"); !ok || v != "us" {
			t.Fatalf("Get(region) after mutating AsMap = %v, %v; want us, true", v, ok)
		}
		if diff := cmp.Diff(mergedWant, c.AsMap()); diff != "" {
			t.Fatalf("AsMap changed after caller mutation (-want +got):\n%s", diff)
		}
	})
}
