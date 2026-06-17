package kart

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

// loadDataOnly 只读 JSON（不解码图集/音频），用于无窗口环境下验证采样数学。
func loadDataOnly(t *testing.T) *Assets {
	t.Helper()
	dir := filepath.Join("..", "assets", "karateman")
	as := &Assets{Anims: map[string]*kmdata.Anim{}}
	for name, dst := range map[string]any{
		"sprites.json": &as.Sheet,
		"rig.json":     &as.Rig,
		"anims.json":   &as.Anims,
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Skipf("assets not extracted: %v", err)
		}
		if err := json.Unmarshal(b, dst); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	return as
}

func TestRigSampleSanity(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)

	for _, anim := range []string{"Beat", "Jab"} {
		rig.Play(anim, 0)
		var poses [][]Aff
		for _, at := range []float64{0, 0.1, 0.2, 0.28} {
			rig.Sample(at)
			cp := make([]Aff, len(rig.world))
			copy(cp, rig.world)
			poses = append(poses, cp)
			for i, w := range rig.world {
				for _, v := range []float64{w.A, w.B, w.C, w.D, w.Tx, w.Ty} {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Fatalf("%s: node %d (%s) produced non-finite transform at t=%v",
							anim, i, as.Rig.Nodes[i].Path, at)
					}
				}
			}
		}
		// 动画应当让至少一个节点的世界变换随时间变化
		changed := false
		for i := range poses[0] {
			if poses[0][i] != poses[1][i] {
				changed = true
				break
			}
		}
		if !changed {
			t.Errorf("%s: no node transform changed between t=0 and t=0.1", anim)
		}
	}
}

func TestJabSpriteSwap(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)
	rig.Play("Jab", 0)

	arm := rig.byPath["LeftArm"]
	rig.Sample(0.0)
	first := rig.state[arm].sprite
	rig.Sample(0.27)
	last := rig.state[arm].sprite
	if first == last {
		t.Errorf("Jab: LeftArm sprite did not swap (%q -> %q)", first, last)
	}
	if first == "" || last == "" {
		t.Errorf("Jab: empty sprite name (%q -> %q)", first, last)
	}
}

func TestRigLayerOverridesSubtreeWithoutReplacingBaseClip(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)
	rig.Play("Jab", 0)
	rig.Sample(0.1)
	leftArm := rig.byPath["LeftArm"]
	baseArmSprite := rig.state[leftArm].sprite
	baseArmRot := rig.state[leftArm].rot

	rig.Play("Jab", 0)
	rig.PlayLayer("face", "Head", "karateman/Head/Face08", 0)
	rig.Sample(0.1)
	head := rig.byPath["Head"]
	if got := rig.state[head].sprite; got != "karateman_head_8" {
		t.Fatalf("Head layer sprite = %q, want karateman_head_8", got)
	}
	if got := rig.state[leftArm].sprite; got != baseArmSprite {
		t.Fatalf("base Jab arm sprite was replaced by face layer: %q -> %q", baseArmSprite, got)
	}
	if got := rig.state[leftArm].rot; got != baseArmRot {
		t.Fatalf("base Jab arm rotation was replaced by face layer: %v -> %v", baseArmRot, got)
	}
}

func TestRigLayerAcceptsRelativeClipPaths(t *testing.T) {
	as := loadDataOnly(t)
	as.Anims["relativeFace"] = &kmdata.Anim{
		Duration: 0.016666668,
		Sprites: map[string][]kmdata.SwapKey{
			"": {{T: 0, Name: "karateman_head_7"}},
		},
	}
	rig := NewRig(as)
	rig.Play("Beat", 0)
	rig.PlayLayer("face", "Head", "relativeFace", 0)
	rig.Sample(0)
	head := rig.byPath["Head"]
	if got := rig.state[head].sprite; got != "karateman_head_7" {
		t.Fatalf("relative Head layer sprite = %q, want karateman_head_7", got)
	}
}

func TestRigRuntimeOverrides(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)

	head := rig.byPath["Head"]
	wig := rig.byPath["Head/Wig"]
	body := rig.byPath["Body"]

	rig.SetActive("Head/Wig", false)
	rig.SetColor("Body", [4]float64{0.25, 0.5, 0.75, 0.6})
	pal := Palette{Alpha: [4]float64{1, 0, 0, 1}, Fill: [4]float64{0, 1, 0, 1}, Outline: [4]float64{0, 0, 1, 1}}
	rig.SetPalette("Head", pal)
	rig.Sample(0)

	if !rig.actives[head] {
		t.Fatal("Head should stay active when only Head/Wig is disabled")
	}
	if rig.actives[wig] {
		t.Fatal("Head/Wig should honor SetActive(false)")
	}
	if rig.state[body].color != [4]float64{0.25, 0.5, 0.75, 0.6} {
		t.Fatalf("Body color override = %#v", rig.state[body].color)
	}
	if got, ok := rig.paletteForNode(head); !ok || got != pal {
		t.Fatalf("Head palette override = %#v, %v", got, ok)
	}
	if got, ok := rig.paletteForNode(wig); !ok || got != pal {
		t.Fatalf("Head/Wig should inherit Head subtree palette, got %#v, %v", got, ok)
	}
	if _, ok := rig.paletteForNode(body); ok {
		t.Fatal("Body should not receive Head subtree palette")
	}
}

func TestBBoxFinite(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)
	minX, minY, maxX, maxY := rig.BBox()
	if !(minX < maxX && minY < maxY) {
		t.Fatalf("degenerate bbox: [%v %v %v %v]", minX, minY, maxX, maxY)
	}
	if maxY-minY < 1 || maxY-minY > 30 {
		t.Errorf("suspicious rig height: %.2f units", maxY-minY)
	}
}

func TestLoadReadsOptionalMeshData(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "meshtex", "grid.png"))
	writeTestJSON(t, dir, "sprites.json", kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}})
	writeTestJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeTestJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1, Scale: [2]float64{1, 1}}}})
	writeTestJSON(t, dir, "roles.json", kmdata.Roles{})
	writeTestJSON(t, dir, "materials.json", map[string]kmdata.Material{
		"SpriteMat": {
			Name: "SpriteMat",
			Floats: map[string]float64{
				"_DoodleFrameTime":  0.25,
				"_DoodleFrameCount": 24,
			},
			Colors: map[string][4]float64{
				"_DoodleMaxOffset":  {0.003, 0.003, 0, 0},
				"_DoodleNoiseScale": {15, 15, 0, 0},
			},
		},
	})
	writeTestJSON(t, dir, "meshes.json", kmdata.MeshData{
		Bindings: []kmdata.MeshBinding{{
			Path:     "Root/Plane",
			Renderer: "MeshRenderer",
			Mesh:     kmdata.AssetRef{FileID: 10209, GUID: "0"},
			Materials: []kmdata.AssetRef{{
				FileID: 2100000,
				GUID:   "mat-guid",
				Name:   "GridPlane",
				Path:   "Bundled/Games/BuiltToScaleDS/Models/Materials/World/GridPlane.mat",
			}},
			Enabled: true,
		}},
		Materials: map[string]kmdata.Material{
			"mat-guid": {
				Name: "GridPlane",
				GUID: "mat-guid",
				Textures: map[string]kmdata.TextureEnv{
					"_MainTex": {Texture: kmdata.AssetRef{GUID: "tex-guid", Name: "grid"}, Image: "meshtex/grid.png"},
				},
			},
		},
	})

	as, err := Load(dir, 48000)
	if err != nil {
		t.Fatal(err)
	}
	if len(as.Meshes.Bindings) != 1 {
		t.Fatalf("mesh bindings = %d, want 1", len(as.Meshes.Bindings))
	}
	if got := as.Meshes.Bindings[0].Mesh.FileID; got != 10209 {
		t.Fatalf("builtin mesh fileID = %d, want 10209", got)
	}
	if as.Meshes.Materials["mat-guid"].Textures["_MainTex"].Texture.GUID != "tex-guid" {
		t.Fatalf("material texture not loaded: %#v", as.Meshes.Materials)
	}
	if as.MeshTex["meshtex/grid.png"] == nil {
		t.Fatalf("mesh texture image not decoded: %#v", as.MeshTex)
	}
	if got := as.Materials["SpriteMat"].Colors["_DoodleNoiseScale"]; got[0] != 15 || got[1] != 15 {
		t.Fatalf("sprite material params not loaded: %#v", as.Materials)
	}
}

func writeTestJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
