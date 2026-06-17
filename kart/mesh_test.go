package kart

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kmdata"
)

func TestSceneDrawsBuiltinMeshRendererBinding(t *testing.T) {
	as := meshTestAssets(nil)
	scene := NewScene(as)
	scene.Sample(0)

	node, tint, ok := scene.meshDrawable(0)
	if !ok {
		t.Fatal("MeshRenderer binding was not considered drawable")
	}
	if node != 0 {
		t.Fatalf("mesh node = %d, want 0", node)
	}
	if tint[0] <= tint[1] || tint[0] <= tint[2] || tint[3] != 1 {
		t.Fatalf("mesh material color not applied: %#v", tint)
	}
	dst := ebiten.NewImage(96, 96)
	scene.Draw(dst, Translate(48, 48).Mul(Scale(4, -4)))
}

func TestMeshRendererMaterialOpacityCurvesAffectTint(t *testing.T) {
	keys := []kmdata.Key{{T: 0, V: 0.25}, {T: 1, V: 0.75}}
	as := meshTestAssets(map[string]*kmdata.Anim{
		"Fade": {
			Duration: 1,
			Floats: map[string]map[string][]kmdata.Key{
				"": {
					"material._Opacity": keys,
					"material._Color.r": {kmdata.Key{T: 0, V: 0}, kmdata.Key{T: 1, V: 0}},
					"material._Color.g": {kmdata.Key{T: 0, V: 0}, kmdata.Key{T: 1, V: 1}},
					"material._Color.b": {kmdata.Key{T: 0, V: 0}, kmdata.Key{T: 1, V: 0}},
					"material._Color.a": {kmdata.Key{T: 0, V: 1}, kmdata.Key{T: 1, V: 1}},
				},
			},
		},
	})
	scene := NewScene(as)
	scene.Play("Plane", "Fade", 0, 1)
	scene.Sample(0.5)

	tint := scene.meshTint(0, &as.Meshes.Bindings[0])
	if math.Abs(tint[3]-0.5) > 1e-9 {
		t.Fatalf("mesh alpha = %v, want 0.5", tint[3])
	}
	if tint[0] != 0 || math.Abs(tint[1]-0.5) > 1e-9 || tint[2] != 0 {
		t.Fatalf("mesh animated material color = %#v, want green midpoint", tint)
	}
}

func TestMeshRendererSharedMaterialColorOverride(t *testing.T) {
	as := meshTestAssets(nil)
	scene := NewScene(as)
	scene.SetMaterialColorFor("GridPlane", [4]float64{0.2, 0.7, 0.4, 0.5})
	scene.Sample(0)

	tint := scene.meshTint(0, &as.Meshes.Bindings[0])
	if tint != [4]float64{0.2, 0.7, 0.4, 0.5} {
		t.Fatalf("mesh material override tint = %#v, want shared material color", tint)
	}
}

func TestImportedFBXMeshRendererNeedsGeometryData(t *testing.T) {
	as := meshTestAssets(nil)
	as.Meshes.Bindings[0].Mesh = kmdata.AssetRef{
		FileID: -1677165586242444022,
		GUID:   "669fa6d6b4096764387b9cf777fbf915",
		Name:   "scene",
	}
	scene := NewScene(as)
	scene.Sample(0)
	if _, _, ok := scene.meshDrawable(0); ok {
		t.Fatal("imported FBX mesh should wait for real vertex extraction")
	}
}

func TestImportedFBXMeshRendererUsesSingleGeometry(t *testing.T) {
	as := meshTestAssets(nil)
	as.Meshes.Bindings[0].Mesh = kmdata.AssetRef{
		FileID: -1677165586242444022,
		GUID:   "mesh-guid",
		Name:   "scene",
	}
	as.Meshes.Geometries = map[string][]kmdata.MeshGeometry{
		"mesh-guid": {{
			Name:     "mesh_1_",
			FBXID:    451390504,
			Vertices: [][3]float64{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}},
			Indices:  []int{0, 1, 2, 0, 2, 3},
		}},
	}
	scene := NewScene(as)
	scene.Sample(0)
	if _, _, ok := scene.meshDrawable(0); !ok {
		t.Fatal("imported FBX mesh with one extracted geometry should be drawable")
	}
	dst := ebiten.NewImage(96, 96)
	scene.Draw(dst, Translate(48, 48).Mul(Scale(4, -4)))
}

func TestImportedFBXMeshRendererUsesTextureUVs(t *testing.T) {
	as := meshTestAssets(nil)
	as.Meshes.Bindings[0].Mesh = kmdata.AssetRef{GUID: "mesh-guid", Name: "scene"}
	as.Meshes.Materials["mat-guid"] = kmdata.Material{
		Name: "Sky",
		GUID: "mat-guid",
		Textures: map[string]kmdata.TextureEnv{
			"_MainTex": {Image: "meshtex/sky.png", Scale: [2]float64{1, 1}},
		},
		Colors: map[string][4]float64{"_Color": {1, 1, 1, 1}},
	}
	as.Meshes.Geometries = map[string][]kmdata.MeshGeometry{
		"mesh-guid": {{
			Name:      "mesh_1_",
			Vertices:  [][3]float64{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}},
			UVs:       [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			Indices:   []int{0, 1, 2, 0, 2, 3},
			UVIndices: []int{0, 1, 2, 0, 2, 3},
		}},
	}
	as.MeshTex = map[string]*ebiten.Image{"meshtex/sky.png": ebiten.NewImage(2, 2)}
	scene := NewScene(as)
	scene.Sample(0)
	if tex, _ := scene.meshTexture(&as.Meshes.Bindings[0]); tex == nil {
		t.Fatal("mesh texture was not resolved")
	}
	dst := ebiten.NewImage(96, 96)
	scene.Draw(dst, Translate(48, 48).Mul(Scale(4, -4)))
}

func TestBuiltinMeshRendererUsesMaterialTexture(t *testing.T) {
	as := meshTestAssets(nil)
	mat := as.Meshes.Materials["mat-guid"]
	mat.Colors["_Color"] = [4]float64{1, 1, 1, 1}
	mat.Textures = map[string]kmdata.TextureEnv{
		"_MainTex": {Image: "meshtex/grid.png", Scale: [2]float64{2, 1}},
	}
	as.Meshes.Materials["mat-guid"] = mat
	tex := ebiten.NewImage(2, 2)
	as.MeshTex = map[string]*ebiten.Image{"meshtex/grid.png": tex}

	scene := NewScene(as)
	scene.Sample(0)
	if tex, env := scene.meshTexture(&as.Meshes.Bindings[0]); tex == nil || env.Scale != [2]float64{2, 1} {
		t.Fatalf("builtin mesh texture = %v env=%#v, want tiled material texture", tex, env)
	}
	if got := repeatBreaks(0, 2); len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("tiling breaks = %v, want [0 1 2]", got)
	}
	dst := ebiten.NewImage(32, 32)
	scene.Draw(dst, Translate(16, 16).Mul(Scale(2, -2)))
}

func TestMeshRendererSharedTextureOffsetOverride(t *testing.T) {
	as := meshTestAssets(nil)
	mat := as.Meshes.Materials["mat-guid"]
	mat.Textures = map[string]kmdata.TextureEnv{
		"_MainTex": {Image: "meshtex/grid.png", Scale: [2]float64{1, 1}, Offset: [2]float64{0.25, 0.5}},
	}
	as.Meshes.Materials["mat-guid"] = mat
	as.MeshTex = map[string]*ebiten.Image{"meshtex/grid.png": ebiten.NewImage(2, 2)}

	scene := NewScene(as)
	scene.SetMaterialTextureOffsetFor("GridPlane", [2]float64{0, -0.75})
	if _, env := scene.meshTexture(&as.Meshes.Bindings[0]); env.Offset != [2]float64{0, -0.75} {
		t.Fatalf("texture offset = %#v, want runtime material offset", env.Offset)
	}
}

func TestSceneDrawsBuiltinSphereMeshRendererBinding(t *testing.T) {
	as := meshTestAssets(nil)
	as.Meshes.Bindings[0].Mesh = kmdata.AssetRef{FileID: 10207, GUID: "0"}
	scene := NewScene(as)
	scene.Sample(0)
	if _, _, ok := scene.meshDrawable(0); !ok {
		t.Fatal("built-in Sphere MeshRenderer should be drawable")
	}
	dst := ebiten.NewImage(32, 32)
	scene.Draw(dst, Translate(16, 16).Mul(Scale(20, -20)))
}

func meshTestAssets(anims map[string]*kmdata.Anim) *Assets {
	if anims == nil {
		anims = map[string]*kmdata.Anim{}
	}
	return &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{{
			Name:   "Plane",
			Path:   "Plane",
			Parent: -1,
			Scale:  [2]float64{1, 1},
		}}},
		Anims: anims,
		Meshes: kmdata.MeshData{
			Bindings: []kmdata.MeshBinding{{
				Path:      "Plane",
				Renderer:  "MeshRenderer",
				Mesh:      kmdata.AssetRef{FileID: 10209, GUID: "0"},
				Materials: []kmdata.AssetRef{{GUID: "mat-guid", Name: "GridPlane"}},
				Enabled:   true,
			}},
			Materials: map[string]kmdata.Material{
				"mat-guid": {
					Name: "GridPlane",
					GUID: "mat-guid",
					Colors: map[string][4]float64{
						"_Color": {1, 0.1, 0.1, 1},
					},
				},
			},
		},
	}
}
