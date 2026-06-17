package engine

import (
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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
			if math.Abs(got[i]-c[i]) > 2.0/255.0 {
				t.Fatalf("default LUT should be identity for %v, got %v", c, got)
			}
		}
	}
}

func TestUnityLUTSamplerInterpolatesAllAxes(t *testing.T) {
	img := loadFilterTestImage(t, "../assets/common/filters/default_lut.png")
	got := sampleUnityLUT(img, [3]float64{0.23, 0.47, 0.61})
	want := [3]float64{0.23, 0.47, 0.61}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 2.0/255.0 {
			t.Fatalf("default LUT interpolation mismatch: got %v want %v", got, want)
		}
	}
	if !strings.Contains(lutKage, "sampleTargetLUT") ||
		!strings.Contains(lutKage, "rf :=") ||
		!strings.Contains(lutKage, "gf :=") ||
		!strings.Contains(lutKage, "bf :=") {
		t.Fatalf("LUT shader must interpolate red/green axes manually and blue slices explicitly")
	}
}

func TestLUTShaderUsesSource0CoordinateSpaceForLUTs(t *testing.T) {
	if !strings.Contains(lutKage, "o := imageSrc0Origin()") {
		t.Fatalf("imageSrc1At/imageSrc2At use source0 coordinates; LUT strip offsets must be added to source0 origin")
	}
	if strings.Contains(lutKage, "imageSrc1Origin()") {
		t.Fatalf("adding source1 origin double-applies the secondary image atlas origin and corrupts colors")
	}
	if strings.Contains(lutKage, "imageSrc2Origin()") {
		t.Fatalf("adding source2 origin double-applies the default LUT atlas origin and corrupts colors")
	}
	if !strings.Contains(lutKage, "imageSrc1At(o + vec2") || !strings.Contains(lutKage, "imageSrc2At(o + vec2") {
		t.Fatalf("LUT shader should sample target and default LUTs with source0-origin strip coordinates")
	}
}

func TestFilterSlotsApplyHighestFirst(t *testing.T) {
	// The vfx/filter action documents this as a charting rule: higher slots are
	// applied first. Reversing it changes the final grade whenever charts stack
	// LUTs, which is visible in Fan Club Dance.
	want := []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	if len(filterApplyOrder) != len(want) {
		t.Fatalf("filterApplyOrder length = %d, want %d", len(filterApplyOrder), len(want))
	}
	for i := range want {
		if filterApplyOrder[i] != want[i] {
			t.Fatalf("filter slot order = %v, want %v", filterApplyOrder, want)
		}
	}
}

func TestLUTShaderBlendsInLUTSpace(t *testing.T) {
	if !strings.Contains(lutKage, "imageSrc2At") {
		t.Fatalf("LUT shader should sample default_lut instead of approximating it with source color")
	}
	if !strings.Contains(lutKage, "defaultColor := mix(defaultLo, defaultHi, bf)") {
		t.Fatalf("LUT shader should interpolate default_lut with the same B-slice fraction")
	}
	if !strings.Contains(lutKage, "mix(defaultColor, graded, Blend)") {
		t.Fatalf("LUT shader should blend LUT outputs, matching AmplifyColor BlendCache")
	}
}

func TestLUTShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(lutKage)); err != nil {
		t.Fatalf("LUT shader should compile: %v", err)
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
	bHi := math.Min(bLo+1, 31)
	bf := b - bLo
	r := clamp01(c[0]) * 31
	r0 := math.Floor(r)
	r1 := math.Min(r0+1, 31)
	rf := r - r0
	g := (1 - clamp01(c[1])) * 31
	g0 := math.Floor(g)
	g1 := math.Min(g0+1, 31)
	gf := g - g0
	lo := sampleUnityLUTSlice(img, bLo, r0, r1, g0, g1, rf, gf)
	hi := sampleUnityLUTSlice(img, bHi, r0, r1, g0, g1, rf, gf)
	return [3]float64{
		lo[0] + (hi[0]-lo[0])*bf,
		lo[1] + (hi[1]-lo[1])*bf,
		lo[2] + (hi[2]-lo[2])*bf,
	}
}

func sampleUnityLUTSlice(img image.Image, b, r0, r1, g0, g1, rf, gf float64) [3]float64 {
	c00 := colorToFloat(img.At(int(b*32+r0), int(g0)))
	c10 := colorToFloat(img.At(int(b*32+r1), int(g0)))
	c01 := colorToFloat(img.At(int(b*32+r0), int(g1)))
	c11 := colorToFloat(img.At(int(b*32+r1), int(g1)))
	return [3]float64{
		lerp(lerp(c00[0], c10[0], rf), lerp(c01[0], c11[0], rf), gf),
		lerp(lerp(c00[1], c10[1], rf), lerp(c01[1], c11[1], rf), gf),
		lerp(lerp(c00[2], c10[2], rf), lerp(c01[2], c11[2], rf), gf),
	}
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func colorToFloat(c color.Color) [3]float64 {
	r, g, b, _ := c.RGBA()
	return [3]float64{float64(r) / 0xffff, float64(g) / 0xffff, float64(b) / 0xffff}
}
