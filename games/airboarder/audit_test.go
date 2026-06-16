package airboarder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

func TestAirboarderExtractedAssetCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "airboarder")
	for _, name := range []string{"scene.json", "sprites.json", "roles.json", "extra.json", "anims.json", "controllers.json", "animators.json", "meshes.json", "atlas0.png"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	var roles map[string]string
	readJSON(t, filepath.Join(root, "roles.json"), &roles)
	for field, want := range map[string]string{
		"CPU1":      "MovingObjects/airboarders/CPU1/neo_airboy",
		"CPU2":      "MovingObjects/airboarders/CPU2/neo_airboy",
		"Player":    "MovingObjects/airboarders/Player/neo_airboy",
		"Dog":       "MovingObjects/dog",
		"Floor":     "Environment/floor_model",
		"archBasic": "Environment/ArchFinePosition/Arch",
		"wallBasic": "Environment/WallFinePosition/Wall",
	} {
		if got := roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}

	var controllers map[string]struct {
		Default string         `json:"default"`
		States  map[string]any `json:"states"`
	}
	readJSON(t, filepath.Join(root, "controllers.json"), &controllers)
	for ctrl, states := range map[string][]string{
		"airboy":      {"hover", "bop", "duck", "charge", "hold", "jump", "hit1", "hit2", "letsgo"},
		"arch":        {"idle", "move", "shake", "break"},
		"Wall":        {"idle", "move", "shake", "break"},
		"floor_model": {"idle", "moving"},
		"dog":         {"run", "wag"},
	} {
		c, ok := controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		if c.Default == "" {
			t.Fatalf("controller %s has no default state", ctrl)
		}
		for _, state := range states {
			if _, ok := c.States[state]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, state)
			}
		}
	}

	var animators map[string]string
	readJSON(t, filepath.Join(root, "animators.json"), &animators)
	for path, ctrl := range map[string]string{
		"MovingObjects/airboarders/CPU1/neo_airboy":   "airboy",
		"MovingObjects/airboarders/CPU2/neo_airboy":   "airboy",
		"MovingObjects/airboarders/Player/neo_airboy": "airboy",
		"Environment/ArchFinePosition/Arch":           "arch",
		"Environment/WallFinePosition/Wall":           "Wall",
	} {
		if got := animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	for _, sound := range []string{
		"ready.wav", "crouch.wav", "crouchCharge.wav", "crouchvox.wav",
		"jump.wav", "jumpvox.wav", "barely.wav", "barelyvox.wav",
		"start1.wav", "start2.wav", "start3.wav", "miss1.wav", "miss15.wav", "missvox.wav",
	} {
		if _, err := os.Stat(filepath.Join(root, "sounds", sound)); err != nil {
			t.Fatalf("missing sound %s: %v", sound, err)
		}
	}
}

func TestAirboarderMeshRendererAssets(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "airboarder")
	var meshes kmdata.MeshData
	readJSON(t, filepath.Join(root, "meshes.json"), &meshes)
	if len(meshes.Bindings) != 1 {
		t.Fatalf("mesh bindings = %d, want 1", len(meshes.Bindings))
	}
	b := meshes.Bindings[0]
	if b.Path != "Sky/vsn_mesh_1_" || b.Renderer != "MeshRenderer" {
		t.Fatalf("sky mesh binding = %#v", b)
	}
	if b.Mesh.GUID == "" || b.Mesh.FileID == 0 {
		t.Fatalf("sky mesh should preserve imported FBX guid+fileID: %#v", b.Mesh)
	}
	if len(b.Materials) != 1 || b.Materials[0].Name != "sky" {
		t.Fatalf("sky material refs = %#v", b.Materials)
	}
	mat := meshes.Materials[b.Materials[0].GUID]
	if mat.Name != "sky" {
		t.Fatalf("sky material not exported: %#v", mat)
	}
	if tex := mat.Textures["_MainTex"].Texture; tex.GUID == "" || tex.FileID == 0 {
		t.Fatalf("sky _MainTex texture ref not preserved: %#v", tex)
	}
	if mat.Colors["_Color"] == ([4]float64{}) {
		t.Fatalf("sky material color missing: %#v", mat.Colors)
	}
	geoms := meshes.Geometries[b.Mesh.GUID]
	if len(geoms) != 1 {
		t.Fatalf("sky geometries = %d, want 1", len(geoms))
	}
	if geoms[0].Name != "mesh_1_" || geoms[0].FBXID == 0 {
		t.Fatalf("sky geometry identity = %#v", geoms[0])
	}
	if len(geoms[0].Vertices) != 32 || len(geoms[0].Indices) != 180 {
		t.Fatalf("sky geometry sizes = %d vertices, %d indices; want 32/180", len(geoms[0].Vertices), len(geoms[0].Indices))
	}
}

func TestAirboarderSynthesizesImportedModelAnimPaths(t *testing.T) {
	var scene kmdata.Rig
	readJSON(t, filepath.Join("..", "..", "assets", "airboarder", "scene.json"), &scene)
	nodes := map[string]bool{}
	for _, n := range scene.Nodes {
		nodes[n.Path] = true
	}
	for _, path := range []string{
		"MovingObjects/airboarders/Player/neo_airboy/airboy_model_skeleton/airboy/Skl_Root/Waist/Spine/Head",
		"MovingObjects/airboarders/CPU1/neo_airboy/airboy_model_skeleton/airboy/board_model/eft_fire_L",
		"Environment/WallFinePosition/Wall/wall_low_model_skeleton/wall_low_root/block04Pg",
		"Environment/ArchFinePosition/Arch/wall_high_model_skeleton/wall_root/block07Pg",
		"MovingObjects/dog/dog_model_skeleton/dog_root/hip/tail2",
	} {
		if !nodes[path] {
			t.Fatalf("missing synthesized imported-model path %q", path)
		}
	}
}

func TestAirboarderMaterialCurvesRemainExtracted(t *testing.T) {
	var anims map[string]*kmdata.Anim
	readJSON(t, filepath.Join("..", "..", "assets", "airboarder", "anims.json"), &anims)
	seenOpacity := false
	seenFresnel := false
	for _, anim := range anims {
		for _, attrs := range anim.Floats {
			if len(attrs["material._Opacity"]) > 0 {
				seenOpacity = true
			}
			if len(attrs["material._FresnelPower"]) > 0 {
				seenFresnel = true
			}
		}
	}
	if !seenOpacity {
		t.Fatal("airboarder obstacle fade clips should contain material._Opacity curves")
	}
	if !seenFresnel {
		t.Fatal("airboarder charge clips should contain material._FresnelPower curves")
	}
}

func TestAirboarderBopRegionSemantics(t *testing.T) {
	m := &Module{bops: []bopEvt{{beat: 4, auto: true}, {beat: 8, auto: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || m.autoBopAt(7.99) != true || m.autoBopAt(8) {
		t.Fatalf("auto bop did not match SetupBopRegion semantics")
	}
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
