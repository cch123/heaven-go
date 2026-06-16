package kart

import (
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/kmdata"
)

func TestTextLayoutAcceptsTMPAlignmentEnums(t *testing.T) {
	a := textTestAssets()
	tn := kmdata.TextNode{
		Path: "Text", Font: "go.ttf", Size: 10,
		Rect: [2]float64{4, 1},
	}
	hs := []int{tmpHLeft, tmpHCenter, tmpHRight, tmpHJustified, tmpHFlush, tmpHGeometry}
	vs := []int{tmpVTop, tmpVMiddle, tmpVBottom, tmpVBaseline, tmpVMidline, tmpVCapline}
	for _, h := range hs {
		for _, v := range vs {
			tn.HAlign, tn.VAlign = h, v
			layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "TMP"}})
			if err != nil {
				t.Fatalf("layout h=%d v=%d: %v", h, v, err)
			}
			layout.close()
		}
	}
}

func TestTextLayoutRightBottomAnchors(t *testing.T) {
	a := textTestAssets()
	tn := kmdata.TextNode{
		Path: "Text", Font: "go.ttf", Size: 10,
		Rect: [2]float64{4, 1}, HAlign: tmpHRight, VAlign: tmpVBottom,
	}
	layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "TMP"}})
	if err != nil {
		t.Fatal(err)
	}
	defer layout.close()

	const pad = 4
	if layout.pivotX != 1 {
		t.Fatalf("right alignment pivotX = %.3f, want 1", layout.pivotX)
	}
	if want := pad + layout.contentW - layout.textW; layout.dotX != want {
		t.Fatalf("right alignment dotX = %d, want %d", layout.dotX, want)
	}
	H := float64(layout.ascent + layout.descent + 2*pad)
	wantPivotY := 1 - float64(pad+layout.ascent+layout.descent)/H
	if math.Abs(layout.pivotY-wantPivotY) > 1e-9 {
		t.Fatalf("bottom alignment pivotY = %.12f, want %.12f", layout.pivotY, wantPivotY)
	}
}

func TestTextLayoutRejectsUnknownAlignment(t *testing.T) {
	a := textTestAssets()
	tn := kmdata.TextNode{
		Path: "Text", Font: "go.ttf", Size: 10,
		Rect: [2]float64{4, 1}, HAlign: 999, VAlign: tmpVMiddle,
	}
	if layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "TMP"}}); err == nil {
		layout.close()
		t.Fatal("unknown horizontal alignment should fail")
	}
	tn.HAlign, tn.VAlign = tmpHCenter, 999
	if layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "TMP"}}); err == nil {
		layout.close()
		t.Fatal("unknown vertical alignment should fail")
	}
}

func textTestAssets() *Assets {
	return &Assets{
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}},
		Rig: kmdata.Rig{Nodes: []kmdata.Node{{
			Name: "Text", Path: "Text", Parent: -1, Scale: [2]float64{1, 1},
		}}},
		Fonts: map[string][]byte{"go.ttf": goregular.TTF},
	}
}
