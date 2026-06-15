package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountInStyleNames(t *testing.T) {
	cases := []struct {
		typ       int
		count     string
		goSound   string
		andSound  string
		wantCount string
	}{
		{typ: 0, count: "one", goSound: "go1", andSound: "and", wantCount: "one1"},
		{typ: 1, count: "two", goSound: "go2", andSound: "and", wantCount: "two2"},
		{typ: 2, count: "three", wantCount: "cowbell"},
		{typ: 3, count: "four", goSound: "gba/go", andSound: "gba/and", wantCount: "gba/four"},
		{typ: 4, count: "one", goSound: "dsmale/go", andSound: "dsmale/and", wantCount: "dsmale/one"},
		{typ: 5, count: "two", goSound: "dsfemale/go", andSound: "dsfemale/and", wantCount: "dsfemale/two"},
	}
	for _, tc := range cases {
		st := countStyle(tc.typ)
		if got := st.count(tc.count); got != tc.wantCount {
			t.Errorf("type %d count(%q) = %q, want %q", tc.typ, tc.count, got, tc.wantCount)
		}
		if tc.goSound != "" && st.goSound() != tc.goSound {
			got := st.goSound()
			t.Errorf("type %d goSound = %q, want %q", tc.typ, got, tc.goSound)
		}
		if tc.andSound != "" && st.and() != tc.andSound {
			got := st.and()
			t.Errorf("type %d and = %q, want %q", tc.typ, got, tc.andSound)
		}
	}
}

func TestCommonSoundsLoadCountInSubfolders(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "assets", "common", "sounds", "gba", "one.wav")); err != nil {
		t.Skipf("common count-in subfolders not extracted: %v", err)
	}
	app := &App{
		appConfig:           appConfig{assetsRoot: filepath.Join("..", "assets")},
		effectsRuntimeState: effectsRuntimeState{commonSounds: map[string][]byte{}},
	}
	app.loadCommonSounds()
	for _, key := range []string{
		"one1", "go2", "and", "cowbell",
		"gba/one", "gba/go", "gba/and",
		"dsmale/two", "dsmale/go",
		"dsfemale/three", "dsfemale/and",
	} {
		if _, ok := app.commonSounds[key]; !ok {
			t.Errorf("common sound %q not loaded", key)
		}
	}
}
