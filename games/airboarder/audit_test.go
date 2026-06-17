package airboarder

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

func TestAirboarderExtractedAssetCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "airboarder")
	for _, name := range []string{"scene.json", "sprites.json", "roles.json", "extra.json", "anims.json", "controllers.json", "animators.json", "materials.json", "meshes.json", "particles.json", "atlas0.png"} {
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

	var extra kmdata.Extra
	readJSON(t, filepath.Join(root, "extra.json"), &extra)
	game := extra.Components["game"]
	assertNear(t, game.Nums["cameraFOV"], 27)
	if game.Refs["cameraPos"] != "CameraPivot/CameraAngle" || game.Refs["cameraPivot"] != "CameraPivot" {
		t.Fatalf("camera refs = pos %q pivot %q", game.Refs["cameraPos"], game.Refs["cameraPivot"])
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
	if len(meshes.Bindings) != 10 {
		t.Fatalf("mesh bindings = %d, want 10 official model renderer bindings", len(meshes.Bindings))
	}
	for path, want := range map[string]struct {
		renderer string
		mesh     string
		material string
	}{
		"Environment/ArchFinePosition/Arch":           {"MeshRenderer", "wall_high", "wall_body"},
		"Environment/WallFinePosition/Wall":           {"MeshRenderer", "wall_low", "wall_body"},
		"MovingObjects/airboarders/CPU1/neo_airboy":   {"SkinnedMeshRenderer", "neo_airboy", "airboy_smile"},
		"MovingObjects/airboarders/CPU2/neo_airboy":   {"SkinnedMeshRenderer", "neo_airboy", "airboy_board"},
		"MovingObjects/airboarders/Player/neo_airboy": {"SkinnedMeshRenderer", "neo_airboy", "airboy_board"},
		"Sky/cloud_model":                             {"MeshRenderer", "cloud_model", "clouds"},
		"Sky/vsn_mesh_1_":                             {"MeshRenderer", "scene", "sky"},
	} {
		b := airboarderMeshBinding(t, meshes, path)
		if b.Renderer != want.renderer || b.Mesh.Name != want.mesh {
			t.Fatalf("%s binding = renderer %q mesh %#v, want %s/%s", path, b.Renderer, b.Mesh, want.renderer, want.mesh)
		}
		if !airboarderHasMaterial(b, want.material) {
			t.Fatalf("%s materials = %#v, want %s", path, b.Materials, want.material)
		}
		if len(meshes.Geometries[b.Mesh.GUID]) == 0 {
			t.Fatalf("%s mesh guid %s has no parsed FBX geometry", path, b.Mesh.GUID)
		}
	}
	b := airboarderMeshBinding(t, meshes, "Sky/vsn_mesh_1_")
	if b.Renderer != "MeshRenderer" {
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
	mainTex := mat.Textures["_MainTex"]
	if tex := mainTex.Texture; tex.GUID == "" || tex.FileID == 0 || tex.Name != "purplesky" {
		t.Fatalf("sky _MainTex texture ref not preserved: %#v", tex)
	}
	if mainTex.Image != "meshtex/5b82c9572d4fe7c41af997c06e051ea0.png" {
		t.Fatalf("sky _MainTex image = %q", mainTex.Image)
	}
	if _, err := os.Stat(filepath.Join(root, mainTex.Image)); err != nil {
		t.Fatalf("sky _MainTex image missing: %v", err)
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
	if len(geoms[0].UVs) != 52 || len(geoms[0].UVIndices) != len(geoms[0].Indices) {
		t.Fatalf("sky uv sizes = %d uvs, %d uv indices; want 52/%d", len(geoms[0].UVs), len(geoms[0].UVIndices), len(geoms[0].Indices))
	}
}

func airboarderMeshBinding(t *testing.T, meshes kmdata.MeshData, path string) kmdata.MeshBinding {
	t.Helper()
	for _, b := range meshes.Bindings {
		if b.Path == path {
			return b
		}
	}
	t.Fatalf("missing mesh binding %s", path)
	return kmdata.MeshBinding{}
}

func airboarderHasMaterial(b kmdata.MeshBinding, name string) bool {
	for _, m := range b.Materials {
		if m.Name == name {
			return true
		}
	}
	return false
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

func TestAirboarderFloorFallbackUsesExtractedMoveClip(t *testing.T) {
	var anims map[string]*kmdata.Anim
	readJSON(t, filepath.Join("..", "..", "assets", "airboarder", "anims.json"), &anims)
	delta := floorMoveDelta(anims["Animations/floor/move"])
	if math.Abs(delta-23.138) > 1e-3 {
		t.Fatalf("floor move delta = %.6f, want extracted clip displacement 23.138", delta)
	}
	m := &Module{floorLoopDelta: delta}
	const spacing = 128
	want := math.Mod(0.5*delta*54, spacing)
	if got := m.floorStripeOffset(2.5, spacing); math.Abs(got-want) > 1e-9 {
		t.Fatalf("mid-cycle floor stripe offset = %.6f, want %.6f from floor move clip", got, want)
	}
	if got0, got5 := m.floorStripeOffset(0, spacing), m.floorStripeOffset(5, spacing); math.Abs(got0-got5) > 1e-9 {
		t.Fatalf("floor stripe cycle should loop every 5 beats: beat0=%.6f beat5=%.6f", got0, got5)
	}
}

func TestAirboarderCameraEventsChainLikeUnity(t *testing.T) {
	m := &Module{cameraEvts: []cameraEvt{
		{beat: 0, length: 4, rotY: 90, rotX: 10, zoom: 2, x: 4, y: -2, additive: false, pivot: 1},
		{beat: 4, length: 4, rotY: 15, rotX: 20, zoom: 0.5, x: -4, y: 3, additive: true, pivot: 2},
	}}
	mid := m.cameraStateAt(2)
	assertNear(t, mid.rotY, 45)
	assertNear(t, mid.rotX, 5)
	assertNear(t, mid.zoom, 1.5)
	assertNear(t, mid.x, 2)
	assertNear(t, mid.y, -1)
	if mid.pivot != 1 {
		t.Fatalf("mid pivot = %d, want Practice", mid.pivot)
	}

	next := m.cameraStateAt(6)
	// Airboarder.ChangeCamera starts a new transition from the previous target,
	// not from the currently interpolated value. Additive Y rotation is applied
	// after wrapping that previous target with C#'s % 360 behavior.
	assertNear(t, next.rotY, 97.5)
	assertNear(t, next.rotX, 15)
	assertNear(t, next.zoom, 1.25)
	assertNear(t, next.x, 0)
	assertNear(t, next.y, 0.5)
	if next.pivot != 2 {
		t.Fatalf("next pivot = %d, want Legacy", next.pivot)
	}
}

func TestAirboarderCameraZoomAddUsesFoldedState(t *testing.T) {
	if got := cameraZoomAdd(2); math.Abs(got+2.5) > 1e-9 {
		t.Fatalf("zoom add for 2x = %.6f, want -2.5", got)
	}
	if got := cameraZoomAdd(0); got != 0 {
		t.Fatalf("non-positive zoom should not move camera: %v", got)
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

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.9f, want %.9f", got, want)
	}
}
