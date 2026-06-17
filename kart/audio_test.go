package kart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeWAVCompatExtensiblePCM(t *testing.T) {
	raw := readAudioFixture(t, "spaceSoccer", "missNeutral.wav")
	pcm, err := DecodePCM(raw, ".wav", 44100)
	if err != nil {
		t.Fatalf("decode extensible PCM: %v", err)
	}
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("decoded PCM length = %d, want non-empty stereo frames", len(pcm))
	}
}

func TestDecodeWAVCompatIEEEFloat(t *testing.T) {
	raw := readAudioFixture(t, "bigRockFinish", "cymbal.wav")
	pcm, err := DecodePCM(raw, ".wav", 44100)
	if err != nil {
		t.Fatalf("decode IEEE float WAV: %v", err)
	}
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("decoded PCM length = %d, want non-empty stereo frames", len(pcm))
	}
}

func TestExtensibleWaveSubFormatRejectsUnknownTail(t *testing.T) {
	if got := extensibleWaveSubFormat([]byte{1, 0, 0, 0, 1, 2, 3}); got != 0 {
		t.Fatalf("short GUID subtype = %d, want 0", got)
	}
}

func readAudioFixture(t *testing.T, game, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "assets", game, "sounds", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("audio fixture not present: %v", err)
	}
	return raw
}
