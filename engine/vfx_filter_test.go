package engine

import (
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"strings"
	"testing"
)

func TestFilterBlendMatchesAmplifyColorCache(t *testing.T) {
	f := filterFX{evts: []filterEvt{
		{beat: 0, length: 1, filter: 38, slot: 1, start: 1, end: 1, ease: 0},
		{beat: 4, length: 2, filter: 3, slot: 2, start: 1, end: 0.15, ease: 0},
	}}

	slots := f.activeSlots(0.5)
	if got := slots[1].blend; got != 1 {
		t.Fatalf("BlendAmount=0 should apply the target LUT at full strength, got %.3f", got)
	}

	slots = f.activeSlots(5)
	if got := slots[2].blend; math.Abs(got-0.575) > 1e-9 {
		t.Fatalf("expected inverted LUT strength halfway through event, got %.3f", got)
	}

	slots = f.activeSlots(6)
	if got := slots[2].blend; math.Abs(got-0.15) > 1e-9 {
		t.Fatalf("expected 15%% target LUT strength at event end, got %.3f", got)
	}
}

func TestDefaultLUTUsesUnityTextureYAxis(t *testing.T) {
	img := loadFilterTestImage(t, "../assets/common/filters/default_lut.png")
	for _, c := range [][3]float64{
		{0, 0, 0},
		{1, 1, 1},
		{0.25, 0.5, 0.75},
		{0.9, 0.1, 0.4},
	} {
		got := sampleUnityLUT(img, c)
		for i := range c {
			if math.Abs(got[i]-c[i]) > 1.0/31.0 {
				t.Fatalf("default LUT should be identity for %v, got %v", c, got)
			}
		}
	}
}

func TestLUTShaderUsesSource0CoordinatesForSecondaryImage(t *testing.T) {
	if !strings.Contains(lutKage, "imageSrc0Origin()") {
		t.Fatalf("imageSrc1At expects source0-space coordinates; LUT lookup must add imageSrc0Origin")
	}
	if strings.Contains(lutKage, "imageSrc1Origin()") {
		t.Fatalf("imageSrc1Origin double-applies the secondary image atlas origin and corrupts LUT colors")
	}
}

func TestFilterSlotsApplyUnityComponentOrder(t *testing.T) {
	// Fan Club Dance starts with slot 1 redder and slot 2 bleach. Unity applies
	// the AmplifyColor components in creation order, so bleach must process the
	// already-redder image instead of being overwritten by it.
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if len(filterApplyOrder) != len(want) {
		t.Fatalf("filterApplyOrder length = %d, want %d", len(filterApplyOrder), len(want))
	}
	for i := range want {
		if filterApplyOrder[i] != want[i] {
			t.Fatalf("filter slot order = %v, want %v", filterApplyOrder, want)
		}
	}
}

func TestLUTShaderPreservesSourceAlpha(t *testing.T) {
	if !strings.Contains(lutKage, "srcColor := imageSrc0At(src)") {
		t.Fatalf("LUT shader should keep the original source sample so alpha survives grading")
	}
	if !strings.Contains(lutKage, "srcColor.a") {
		t.Fatalf("LUT shader should preserve source alpha instead of making transparent pixels opaque")
	}
}

func loadFilterTestImage(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func sampleUnityLUT(img image.Image, c [3]float64) [3]float64 {
	b := clamp01(c[2]) * 31
	bLo := math.Floor(b)
	fr := b - bLo
	x0 := int(math.Round(bLo*32 + clamp01(c[0])*31))
	x1 := int(math.Round(math.Min(bLo+1, 31)*32 + clamp01(c[0])*31))
	y := int(math.Round((1 - clamp01(c[1])) * 31))
	lo := colorToFloat(img.At(x0, y))
	hi := colorToFloat(img.At(x1, y))
	return [3]float64{
		lo[0] + (hi[0]-lo[0])*fr,
		lo[1] + (hi[1]-lo[1])*fr,
		lo[2] + (hi[2]-lo[2])*fr,
	}
}

func colorToFloat(c color.Color) [3]float64 {
	r, g, b, _ := c.RGBA()
	return [3]float64{float64(r) / 0xffff, float64(g) / 0xffff, float64(b) / 0xffff}
}
