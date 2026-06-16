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
	H := float64(layout.contentH + 2*pad)
	wantPivotY := 1 - float64(pad+layout.contentH)/H
	if math.Abs(layout.pivotY-wantPivotY) > 1e-9 {
		t.Fatalf("bottom alignment pivotY = %.12f, want %.12f", layout.pivotY, wantPivotY)
	}
}

func TestTextLayoutMultilineBreaksAndCentersPerLine(t *testing.T) {
	a := textTestAssets()
	tn := kmdata.TextNode{
		Path: "Text", Font: "go.ttf", Size: 10,
		Rect: [2]float64{4, 3}, HAlign: tmpHCenter, VAlign: tmpVMiddle,
	}
	layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "A\nBB\nC"}})
	if err != nil {
		t.Fatal(err)
	}
	defer layout.close()

	if got := len(layout.lines); got != 3 {
		t.Fatalf("line count = %d, want 3", got)
	}
	if got := len(layout.spans); got != 3 {
		t.Fatalf("span count = %d, want 3", got)
	}
	if layout.lines[1].width <= layout.lines[0].width {
		t.Fatalf("middle line should be widest: widths=%v", []int{layout.lines[0].width, layout.lines[1].width, layout.lines[2].width})
	}
	if layout.lines[0].dotX <= layout.lines[1].dotX {
		t.Fatalf("short centered line should start to the right of widest line: dotX=%v", []int{layout.lines[0].dotX, layout.lines[1].dotX})
	}
	lineH := layout.ascent + layout.descent
	if layout.lines[1].baseline-layout.lines[0].baseline != lineH ||
		layout.lines[2].baseline-layout.lines[1].baseline != lineH {
		t.Fatalf("baselines are not one line-height apart: %+v lineH=%d", layout.lines, lineH)
	}
	if len(layout.charX) != 4 {
		t.Fatalf("charX len = %d, want 4", len(layout.charX))
	}
	if layout.charX[1] != 0 || layout.charX[2] <= 0 || layout.charX[3] != 0 {
		t.Fatalf("charX should reset after each newline, got %v", layout.charX)
	}
}

func TestTextLayoutNewlineOnlyHasNoRenderableContent(t *testing.T) {
	a := textTestAssets()
	tn := kmdata.TextNode{
		Path: "Text", Font: "go.ttf", Size: 10,
		Rect: [2]float64{4, 1}, HAlign: tmpHCenter, VAlign: tmpVMiddle,
	}
	layout, err := a.layoutTextRuns(&tn, []TextRun{{Text: "\n"}})
	if err != nil {
		t.Fatal(err)
	}
	defer layout.close()
	if layout.content {
		t.Fatal("newline-only TMP text should not create a visible sprite")
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
