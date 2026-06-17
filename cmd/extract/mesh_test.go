package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

func TestCollectMeshBindingsExportsMeshRendererMaterials(t *testing.T) {
	const (
		meshGUID = "mesh-guid"
		matGUID  = "mat-guid"
	)
	dt := &docTable{byID: map[int64]*docRef{
		33: {classID: 33, content: map[string]any{
			"m_GameObject": map[string]any{"fileID": float64(1)},
			"m_Mesh":       map[string]any{"fileID": float64(4300000), "guid": meshGUID},
		}},
		23: {classID: 23, content: map[string]any{
			"m_GameObject":     map[string]any{"fileID": float64(1)},
			"m_Enabled":        float64(1),
			"m_SortingLayer":   float64(2),
			"m_SortingOrder":   float64(3),
			"m_Materials":      []any{map[string]any{"fileID": float64(2100000), "guid": matGUID}},
			"m_ReceiveShadows": float64(1),
		}},
	}}
	bindings := collectMeshBindings(dt, map[int64]string{1: "Game/Models/Airboy"}, assetIndex{
		refs: kmdataRef(meshGUID, "scene", "Models/scene.fbx"),
	}, assetIndex{
		refs: kmdataRef(matGUID, "airboy_body", "Models/Materials/airboy_body.mat"),
	})
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	got := bindings[0]
	if got.Path != "Game/Models/Airboy" || got.Renderer != "MeshRenderer" {
		t.Fatalf("binding identity wrong: %#v", got)
	}
	if got.Mesh.GUID != meshGUID || got.Mesh.FileID != 4300000 || got.Mesh.Name != "scene" {
		t.Fatalf("mesh ref wrong: %#v", got.Mesh)
	}
	if len(got.Materials) != 1 || got.Materials[0].GUID != matGUID || got.Materials[0].Name != "airboy_body" {
		t.Fatalf("material refs wrong: %#v", got.Materials)
	}
	if !got.Enabled || got.Layer != 2 || got.Order != 3 {
		t.Fatalf("renderer flags wrong: %#v", got)
	}
}

func TestCollectMeshBindingsExportsSkinnedMeshRenderer(t *testing.T) {
	const meshGUID = "skinned-mesh-guid"
	dt := &docTable{byID: map[int64]*docRef{
		137: {classID: 137, content: map[string]any{
			"m_GameObject": map[string]any{"fileID": float64(7)},
			"m_Enabled":    float64(1),
			"m_Mesh":       map[string]any{"fileID": float64(7400000), "guid": meshGUID},
		}},
	}}
	bindings := collectMeshBindings(dt, map[int64]string{7: "Game/Models/Dog"}, assetIndex{
		refs: kmdataRef(meshGUID, "dog", "Models/dog.fbx"),
	}, assetIndex{})
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	if bindings[0].Renderer != "SkinnedMeshRenderer" || bindings[0].Mesh.GUID != meshGUID {
		t.Fatalf("skinned binding wrong: %#v", bindings[0])
	}
}

func TestCollectMeshBindingsKeepsBuiltinMeshFileID(t *testing.T) {
	dt := &docTable{byID: map[int64]*docRef{
		33: {classID: 33, content: map[string]any{
			"m_GameObject": map[string]any{"fileID": float64(3)},
			"m_Mesh":       map[string]any{"fileID": float64(10209), "guid": float64(0)},
		}},
		23: {classID: 23, content: map[string]any{
			"m_GameObject": map[string]any{"fileID": float64(3)},
			"m_Enabled":    float64(1),
		}},
	}}
	bindings := collectMeshBindings(dt, map[int64]string{3: "Game/Models/Plane"}, assetIndex{}, assetIndex{})
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	if bindings[0].Mesh.FileID != 10209 || bindings[0].Mesh.GUID != "0" {
		t.Fatalf("builtin mesh ref should preserve fileID and guid sentinel: %#v", bindings[0].Mesh)
	}
}

func TestParseAirboarderSceneFBXGeometry(t *testing.T) {
	path := filepath.Join(*hsRoot, "Assets", "Bundled", "Games", "Airboarder", "Models", "scene.fbx")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("Heaven Studio source asset unavailable: %v", err)
	}
	geoms, err := parseFBXGeometries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(geoms) != 1 {
		t.Fatalf("geometries = %d, want 1", len(geoms))
	}
	g := geoms[0]
	if g.Name != "mesh_1_" || g.FBXID == 0 {
		t.Fatalf("geometry identity = %#v", g)
	}
	if len(g.Vertices) != 32 || len(g.Indices) != 180 {
		t.Fatalf("geometry sizes = %d vertices, %d indices; want 32/180", len(g.Vertices), len(g.Indices))
	}
	if len(g.UVs) != 52 || len(g.UVIndices) != len(g.Indices) {
		t.Fatalf("uv sizes = %d uvs, %d uv indices; want 52/%d", len(g.UVs), len(g.UVIndices), len(g.Indices))
	}
	minX, maxX := g.Vertices[0][0], g.Vertices[0][0]
	for _, v := range g.Vertices {
		minX = math.Min(minX, v[0])
		maxX = math.Max(maxX, v[0])
	}
	if math.Abs(minX+181.85501098632812) > 0.001 || math.Abs(maxX-181.85501098632812) > 0.001 {
		t.Fatalf("x bounds = %.6f..%.6f", minX, maxX)
	}
}

func TestRepairBrokenAirboarderSkyTexture(t *testing.T) {
	prevGame, prevHS := *game, *hsRoot
	t.Cleanup(func() {
		*game = prevGame
		*hsRoot = prevHS
	})
	*game = "airboarder"
	*hsRoot = t.TempDir()
	matDir := filepath.Join(*hsRoot, "Assets", "Bundled", "Games", "Airboarder", "Models", "Materials")
	if err := os.MkdirAll(matDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(matDir, "purplesky.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := repairBrokenMaterialTexture(filepath.Join(matDir, "sky.mat"), "sky", "_MainTex", kmdata.AssetRef{
		FileID: 2800000,
		GUID:   "5b82c9572d4fe7c41af997c06e051ea0",
	})
	if ref.Name != "purplesky" || ref.Path != "Bundled/Games/Airboarder/Models/Materials/purplesky.png" {
		t.Fatalf("repair ref = %#v", ref)
	}
	other := repairBrokenMaterialTexture(filepath.Join(matDir, "other.mat"), "other", "_MainTex", kmdata.AssetRef{
		FileID: 2800000,
		GUID:   "5b82c9572d4fe7c41af997c06e051ea0",
	})
	if other.Path != "" || other.Name != "" {
		t.Fatalf("non-sky material should not be repaired: %#v", other)
	}
}

func kmdataRef(guid, name, path string) map[string]kmdata.AssetRef {
	return map[string]kmdata.AssetRef{guid: {GUID: guid, Name: name, Path: path}}
}
