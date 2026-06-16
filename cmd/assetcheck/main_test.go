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
	})
	r := auditGame(dir, "meshGame")
	if len(r.errs) != 0 {
		t.Fatalf("expected clean mesh audit, got %#v", r.errs)
	}
	if r.meshBindings != 1 {
		t.Fatalf("meshBindings = %d, want 1", r.meshBindings)
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
