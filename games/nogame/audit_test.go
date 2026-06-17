package nogame

import (
	"math"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func TestNoGameIsExplicitInertModule(t *testing.T) {
	m := New()
	if got := m.ID(); got != "noGame" {
		t.Fatalf("ID = %q, want noGame", got)
	}
	m.OnEvent(&riq.Entity{Datamodel: "noGame/ignored", Beat: 12})
	m.Ready()
	m.OnSwitch(12)
	m.Whiff(12)
	m.Update(0, 12)
}

func TestNoGameUsesOfficialPrefabAssets(t *testing.T) {
	as, err := kart.Load(filepath.Join("..", "..", "assets", "noGame"), engine.SampleRate)
	if err != nil {
		t.Fatalf("Load noGame assets: %v", err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}

	squareIdx, ok := as.NodeIndex("Square")
	if !ok {
		t.Fatalf("official noGame prefab should contain Square node")
	}
	square := as.Rig.Nodes[squareIdx]
	if square.Sprite != kart.UnitySquareSprite {
		t.Fatalf("Square sprite = %q, want %q", square.Sprite, kart.UnitySquareSprite)
	}
	if square.Order != -59 || square.Scale != [2]float64{20, 15} {
		t.Fatalf("Square transform/order = scale %v order %d, want scale [20 15] order -59", square.Scale, square.Order)
	}
	for i, want := range [4]float64{0.15294118, 0.15294118, 0.15294118, 1} {
		if math.Abs(square.Color[i]-want) > 1e-7 {
			t.Fatalf("Square color[%d] = %.8f, want %.8f", i, square.Color[i], want)
		}
	}

	textIdx, ok := as.NodeIndex("Text (TMP)")
	if !ok {
		t.Fatalf("official noGame prefab should contain Text (TMP) node")
	}
	textNode := as.Rig.Nodes[textIdx]
	if textNode.Sprite != "__text_Text (TMP)" {
		t.Fatalf("Text sprite = %q, want generated TMP sprite", textNode.Sprite)
	}
	info, ok := as.Sheet.Sprites[textNode.Sprite]
	if !ok || info.W <= 0 || info.H <= 0 {
		t.Fatalf("generated TMP sprite %q missing or empty: %#v", textNode.Sprite, info)
	}
	if _, ok := as.Fonts["Roboto-Medium.ttf"]; !ok {
		t.Fatalf("official noGame TMP font Roboto-Medium.ttf was not loaded")
	}
}
