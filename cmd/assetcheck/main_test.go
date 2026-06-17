package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
)

func TestAuditGameChecksMeshBindings(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Bindings: []kmdata.MeshBinding{{
			Path:      "",
			Renderer:  "MeshRenderer",
			Mesh:      kmdata.AssetRef{FileID: 10209, GUID: "0"},
			Materials: []kmdata.AssetRef{{FileID: 2100000, GUID: "mat-guid"}},
			Enabled:   true,
		}},
		Materials: map[string]kmdata.Material{"mat-guid": {Name: "GridPlane", GUID: "mat-guid"}},
		Geometries: map[string][]kmdata.MeshGeometry{
			"mesh-guid": {{
				Name:      "mesh",
				Vertices:  [][3]float64{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}},
				UVs:       [][2]float64{{0, 0}, {1, 0}, {1, 1}},
				Indices:   []int{0, 1, 2},
				UVIndices: []int{0, 1, 2},
			}},
		},
	})
	r := auditGame(dir, "meshGame")
	if len(r.errs) != 0 {
		t.Fatalf("expected clean mesh audit, got %#v", r.errs)
	}
	if r.meshBindings != 1 {
		t.Fatalf("meshBindings = %d, want 1", r.meshBindings)
	}
}

func TestAuditGameAllowsBuiltinUnityMeshMaterial(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Bindings: []kmdata.MeshBinding{{
			Path:      "",
			Renderer:  "MeshRenderer",
			Mesh:      kmdata.AssetRef{FileID: 10210, GUID: "0"},
			Materials: []kmdata.AssetRef{{FileID: 10754, GUID: unityBuiltinGUID}},
			Enabled:   true,
		}},
	})

	r := auditGame(dir, "meshGame")
	if len(r.errs) != 0 {
		t.Fatalf("expected builtin Unity material to pass audit, got %#v", r.errs)
	}
}

func TestAuditGameRejectsInvalidMeshGeometryUVIndex(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Geometries: map[string][]kmdata.MeshGeometry{
			"mesh-guid": {{
				Name:      "mesh",
				Vertices:  [][3]float64{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}},
				UVs:       [][2]float64{{0, 0}},
				Indices:   []int{0, 1, 2},
				UVIndices: []int{0, 1, 2},
			}},
		},
	})
	r := auditGame(dir, "meshGame")
	found := false
	for _, err := range r.errs {
		if strings.Contains(err, "uv index 1 out of 1 uvs") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected invalid mesh geometry uv index error, got %#v", r.errs)
	}
}

func TestAuditGameRejectsMissingMeshTextureImage(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Materials: map[string]kmdata.Material{
			"mat-guid": {
				Name: "GridPlane",
				GUID: "mat-guid",
				Textures: map[string]kmdata.TextureEnv{
					"_MainTex": {Image: "meshtex/missing.png"},
				},
			},
		},
	})
	r := auditGame(dir, "meshGame")
	found := false
	for _, err := range r.errs {
		if strings.Contains(err, `texture _MainTex image "meshtex/missing.png" missing`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing mesh texture image error, got %#v", r.errs)
	}
}

func TestAuditGameRejectsInvalidMeshGeometryIndex(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Geometries: map[string][]kmdata.MeshGeometry{
			"mesh-guid": {{
				Name:     "mesh",
				Vertices: [][3]float64{{0, 0, 0}},
				Indices:  []int{0, 1, 2},
			}},
		},
	})
	r := auditGame(dir, "meshGame")
	found := false
	for _, err := range r.errs {
		if strings.Contains(err, "index 1 out of 1 vertices") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected invalid mesh geometry index error, got %#v", r.errs)
	}
}

func TestAuditGameRejectsMissingMeshBindingPath(t *testing.T) {
	dir := t.TempDir()
	writeAssetJSON(t, dir, "scene.json", kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "", Parent: -1}}})
	writeAssetJSON(t, dir, "roles.json", kmdata.Roles{})
	writeAssetJSON(t, dir, "anims.json", map[string]*kmdata.Anim{})
	writeAssetJSON(t, dir, "meshes.json", kmdata.MeshData{
		Bindings: []kmdata.MeshBinding{{Path: "Missing", Renderer: "MeshRenderer", Mesh: kmdata.AssetRef{FileID: 10209, GUID: "0"}}},
	})
	r := auditGame(dir, "meshGame")
	found := false
	for _, err := range r.errs {
		if strings.Contains(err, "mesh binding") && strings.Contains(err, "missing scene path") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing mesh binding path error, got %#v", r.errs)
	}
}

func TestAuditGameChecksLegacyRigLayout(t *testing.T) {
	dir := t.TempDir()
	writeLegacyKarateFixture(t, dir, map[string]*kmdata.Anim{
		"karateman/UpperCut": {
			Pos: map[string]kmdata.XYCurve{
				"Arm":             {},
				"Body/BoxingBody": {}, // stale Unity binding absent from karateman.prefab
			},
			Sprites: map[string][]kmdata.SwapKey{
				"Arm": {{T: 0, Name: "arm_0"}},
			},
		},
	})
	r := auditGame(dir, "karateman")
	if len(r.errs) != 0 {
		t.Fatalf("expected clean legacy rig audit, got %#v", r.errs)
	}
	if r.nodes != 2 || r.checkedPaths != 2 {
		t.Fatalf("legacy audit counters nodes=%d checkedPaths=%d, want 2/2", r.nodes, r.checkedPaths)
	}
}

func TestAuditGameAllowsBuiltinUnitySquareSprite(t *testing.T) {
	dir := t.TempDir()
	writeLegacyKarateFixture(t, dir, map[string]*kmdata.Anim{
		"karateman/Flash": {
			Sprites: map[string][]kmdata.SwapKey{
				"Arm": {{T: 0, Name: unitySquareSpriteName}},
			},
		},
	})
	writeAssetJSON(t, dir, "rig.json", kmdata.Rig{Nodes: []kmdata.Node{
		{Name: "KarateMan", Path: "", Parent: -1},
		{Name: "Arm", Path: "Arm", Parent: 0, Sprite: unitySquareSpriteName},
	}})

	r := auditGame(dir, "karateman")
	if len(r.errs) != 0 {
		t.Fatalf("expected builtin square sprite to pass audit, got %#v", r.errs)
	}
}

func TestAuditGameRejectsLegacyMissingAnimationPath(t *testing.T) {
	dir := t.TempDir()
	writeLegacyKarateFixture(t, dir, map[string]*kmdata.Anim{
		"karateman/Jab": {
			Pos: map[string]kmdata.XYCurve{"Missing": {}},
		},
	})
	r := auditGame(dir, "karateman")
	found := false
	for _, err := range r.errs {
		if strings.Contains(err, `animation karateman/Jab path "Missing" missing from legacy rig`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing legacy animation path error, got %#v", r.errs)
	}
}

func writeAssetJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLegacyKarateFixture(t *testing.T, dir string, anims map[string]*kmdata.Anim) {
	t.Helper()
	writeAssetJSON(t, dir, "rig.json", kmdata.Rig{Nodes: []kmdata.Node{
		{Name: "KarateMan", Path: "", Parent: -1},
		{Name: "Arm", Path: "Arm", Parent: 0, Sprite: "arm_0"},
	}})
	writeAssetJSON(t, dir, "sprites.json", kmdata.Sheet{
		PPU:     100,
		Sprites: map[string]kmdata.SpriteInfo{"arm_0": {W: 16, H: 16}},
	})
	writeAssetJSON(t, dir, "stage.json", kmdata.Stage{
		HitPositions: [][2]float64{{0, 0}},
		ItemCurves: []kmdata.Curve{{Points: []kmdata.CurvePoint{
			{P: [3]float64{0, 0, 0}},
			{P: [3]float64{1, 0, 0}},
		}}},
	})
	writeAssetJSON(t, dir, "anims.json", anims)
	writeAssetJSON(t, dir, "particles.json", kmdata.ParticleData{
		Systems: []kmdata.ParticleSystem{{
			Path:         "karateman/Effect/Snow",
			Enabled:      true,
			Renderer:     kmdata.ParticleRenderer{Enabled: true},
			MaxParticles: 16,
		}},
	})
}
