package engine

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"hsdemo/kart"
)

func loadResultAudio(dir string) resultAudioAssets {
	out := resultAudioAssets{clips: map[string][]byte{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("engine: no result sound directory %s", dir)
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".wav" && ext != ".ogg" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		pcm, err := kart.DecodePCM(raw, ext, SampleRate)
		if err != nil {
			log.Printf("engine: decode result sound %s: %v", e.Name(), err)
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		out.clips[key] = pcm
	}
	return out
}
