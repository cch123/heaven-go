package engine

import (
	"math"
	"testing"
)

func TestTimingOKScaleMatchesUnityFracAsymmetry(t *testing.T) {
	const eps = 1e-6
	tests := []struct {
		name   string
		signed float64
		want   float64
	}{
		{name: "late just outside ace", signed: WinAce + eps, want: 1 - eps/(WinJust-WinAce)/2},
		{name: "late at ok edge", signed: WinJust, want: 0.5},
		{name: "early just outside ace", signed: -WinAce - eps, want: 0.5 + eps/(WinJust-WinAce)/2},
		{name: "early at ok edge", signed: -WinJust, want: 1},
		{name: "early halfway", signed: -(WinAce + WinJust) / 2, want: 0.75},
	}
	for _, tt := range tests {
		h := timingHit{signed: tt.signed, y: timingBarNorm(tt.signed), rating: JudgeJust}
		if got := timingOKScale(h); math.Abs(got-tt.want) > 1e-9 {
			t.Fatalf("%s scale = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTimingBarNormMatchesPrefabSegments(t *testing.T) {
	tests := []struct {
		name   string
		signed float64
		want   float64
	}{
		{name: "center", signed: 0, want: 0},
		{name: "ace edge", signed: WinAce, want: timingBarAceNorm},
		{name: "ok edge", signed: WinJust, want: timingBarOKNorm},
		{name: "ng edge", signed: WinNG, want: 1},
		{name: "early mirrors late", signed: -WinJust, want: -timingBarOKNorm},
	}
	for _, tt := range tests {
		if got := timingBarNorm(tt.signed); math.Abs(got-tt.want) > 1e-7 {
			t.Fatalf("%s norm = %v, want %v", tt.name, got, tt.want)
		}
	}
}
