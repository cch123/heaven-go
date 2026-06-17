package main

import (
	"testing"

	uy "hsdemo/unityyaml"
)

func TestSynthesizeModelPrefabInstanceAddsMeshFilterForStrippedMeshRenderer(t *testing.T) {
	restoreNestedNextID := nestedNextID
	nestedNextID = 1 << 40
	t.Cleanup(func() { nestedNextID = restoreNestedNextID })

	mod := map[string]any{
		"m_TransformParent": map[string]any{"fileID": 0},
		"m_Modifications": []any{
			prefabModValue(100, "m_RootOrder", 0),
			prefabModValue(10, "m_Name", "scene_model"),
			prefabModValue(23, "m_Materials.Array.size", 1),
			prefabModObject(23, "m_Materials.Array.data[0]", 2100000, "material-guid"),
			prefabModObject(23, "m_Materials.Array.data[3]", 2100000, "stale-material-guid"),
			prefabModValue(23, "m_SortingOrder", 7),
		},
	}
	stripped := []strippedDoc{
		{id: 10, inst: 1, src: 10, classID: 1},
		{id: 100, inst: 1, src: 100, classID: 4},
		{id: 23, inst: 1, src: 23, classID: 23},
	}

	docs, rootTF, ok := synthesizeModelPrefabInstance(1, "model-guid", mod, stripped)
	if !ok || rootTF != 100 {
		t.Fatalf("synthesize root = (%d, %v), want (100, true)", rootTF, ok)
	}
	renderer := findDocContent(t, docs, 23)
	if got := uy.I(renderer["m_SortingOrder"]); got != 7 {
		t.Fatalf("renderer sorting order = %d, want 7", got)
	}
	mats := uy.L(renderer["m_Materials"])
	if len(mats) != 1 || uy.S(uy.M(mats[0])["guid"]) != "material-guid" {
		t.Fatalf("renderer materials = %#v, want material-guid override", mats)
	}
	if _, leaked := renderer[explicitArraySizePrefix+"m_Materials"]; leaked {
		t.Fatalf("explicit array size marker leaked into synthesized renderer: %#v", renderer)
	}
	filter := findDocContent(t, docs, 33)
	if got := uy.I(uy.Get(filter, "m_GameObject", "fileID")); got != 10 {
		t.Fatalf("mesh filter GameObject = %d, want synthetic root GameObject 10", got)
	}
	if got := uy.I(uy.Get(filter, "m_Mesh", "fileID")); got != 23 {
		t.Fatalf("mesh filter mesh fileID = %d, want renderer source fileID 23", got)
	}
	if got := uy.S(uy.Get(filter, "m_Mesh", "guid")); got != "model-guid" {
		t.Fatalf("mesh filter mesh guid = %q, want model-guid", got)
	}
}

func TestSynthesizeModelPrefabInstanceAddsMeshToSkinnedRenderer(t *testing.T) {
	restoreNestedNextID := nestedNextID
	nestedNextID = 1 << 40
	t.Cleanup(func() { nestedNextID = restoreNestedNextID })

	mod := map[string]any{
		"m_TransformParent": map[string]any{"fileID": 0},
		"m_Modifications": []any{
			prefabModValue(200, "m_RootOrder", 0),
			prefabModValue(20, "m_Name", "paddler_model"),
			prefabModValue(137, "m_Materials.Array.size", 1),
			prefabModObject(137, "m_Materials.Array.data[0]", 2100000, "skin-material"),
			prefabModValue(137, "m_RootBone", 200),
		},
	}
	stripped := []strippedDoc{
		{id: 20, inst: 2, src: 20, classID: 1},
		{id: 200, inst: 2, src: 200, classID: 4},
		{id: 137, inst: 2, src: 137, classID: 137},
	}

	docs, _, ok := synthesizeModelPrefabInstance(2, "skinned-guid", mod, stripped)
	if !ok {
		t.Fatal("synthesize failed")
	}
	renderer := findDocContent(t, docs, 137)
	if got := uy.I(uy.Get(renderer, "m_Mesh", "fileID")); got != 137 {
		t.Fatalf("skinned mesh fileID = %d, want renderer source fileID 137", got)
	}
	if got := uy.S(uy.Get(renderer, "m_Mesh", "guid")); got != "skinned-guid" {
		t.Fatalf("skinned mesh guid = %q, want skinned-guid", got)
	}
	if got := uy.I(uy.Get(renderer, "m_RootBone")); got != 200 {
		t.Fatalf("root bone override = %d, want 200", got)
	}
	for _, d := range docs {
		if d.ClassID == 33 {
			t.Fatalf("skinned renderer should not synthesize MeshFilter: %#v", d)
		}
	}
}

func findDocContent(t *testing.T, docs []uy.Doc, classID int) map[string]any {
	t.Helper()
	for _, d := range docs {
		if d.ClassID == classID {
			return d.Content()
		}
	}
	t.Fatalf("missing classID %d in docs", classID)
	return nil
}

func prefabModValue(target int64, path string, value any) map[string]any {
	return map[string]any{
		"target":       map[string]any{"fileID": target},
		"propertyPath": path,
		"value":        value,
	}
}

func prefabModObject(target int64, path string, fileID int64, guid string) map[string]any {
	return map[string]any{
		"target":          map[string]any{"fileID": target},
		"propertyPath":    path,
		"objectReference": map[string]any{"fileID": fileID, "guid": guid},
	}
}
