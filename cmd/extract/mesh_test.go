package main

import (
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

func kmdataRef(guid, name, path string) map[string]kmdata.AssetRef {
	return map[string]kmdata.AssetRef{guid: {GUID: guid, Name: name, Path: path}}
}
