package rockers

import (
	"math"
	"testing"
)

func TestPremadeSampleMappingMatchesUnityEnum(t *testing.T) {
	cases := map[int]string{
		sampleNone:                "",
		sampleChordA:              "rocker/rockerChordA",
		sampleChordG:              "rocker/rockerChordG",
		sampleRemix6ChordA:        "rocker/rockerRemix6ChordA",
		sampleRemix10ChordD:       "rocker/rockerRemix10ChordD",
		sampleRemix10ChordFSharpm: "rocker/rockerRemix10ChordF#m",
		sampleDoremiChordGsus4:    "doremi/doremiChordGsus4",
	}
	for idx, want := range cases {
		if got := sampleAt(idx).key; got != want {
			t.Fatalf("sampleAt(%d) = %q, want %q", idx, got, want)
		}
	}
}

func TestStringVolumeUsesStringSlotCount(t *testing.T) {
	if got := stringVolume(6); math.Abs(got-0.62) > 1e-9 {
		t.Fatalf("stringVolume(6) = %.3f, want 0.620", got)
	}
	if got := stringVolume(4); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("stringVolume(4) = %.3f, want 0.750", got)
	}
}

func TestCameraMovesChainWithEaseInOutQuad(t *testing.T) {
	m := &Module{}
	m.addCameraMove(10, -2.76)
	m.addCameraMove(20, 2.8)
	m.Ready()

	if got := m.cameraX(9.5); got != 0 {
		t.Fatalf("camera before first move = %v, want 0", got)
	}
	if got := m.cameraX(11); math.Abs(got+2.76) > 1e-9 {
		t.Fatalf("camera after first move = %v, want -2.76", got)
	}
	if got := m.cameraX(20); math.Abs(got+2.76) > 1e-9 {
		t.Fatalf("camera at second start = %v, want previous target -2.76", got)
	}
	if got := m.cameraX(21); math.Abs(got-2.8) > 1e-9 {
		t.Fatalf("camera after second move = %v, want 2.8", got)
	}
}
