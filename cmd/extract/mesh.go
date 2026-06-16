package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

type assetIndex struct {
	refs map[string]kmdata.AssetRef
}

func scanAssetIndex(root string, suffixes ...string) assetIndex {
	out := assetIndex{refs: map[string]kmdata.AssetRef{}}
	for _, suffix := range suffixes {
		for guid, path := range scanGUIDs(root, suffix) {
			out.refs[guid] = kmdata.AssetRef{
				GUID: guid,
				Name: strings.TrimSuffix(filepath.Base(path), suffix),
				Path: unityRelPath(path),
			}
		}
	}
	return out
}

func unityRelPath(path string) string {
	assetsRoot := filepath.Join(*hsRoot, "Assets")
	if rel, err := filepath.Rel(assetsRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func exportMeshes(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string) {
	gameRoot := spec.gameRoot()
	meshAssets := scanAssetIndex(gameRoot, ".fbx", ".obj", ".asset")
	materialAssets := scanAssetIndex(gameRoot, ".mat")
	shaderAssets := scanAssetIndex(filepath.Join(*hsRoot, "Assets"), ".shader")
	textureAssets := scanAssetIndex(gameRoot, ".png", ".psd", ".tga", ".jpg", ".jpeg")

	bindings := collectMeshBindings(dt, paths, meshAssets, materialAssets)
	data := kmdata.MeshData{
		Bindings:   bindings,
		Materials:  collectMeshMaterials(collectMaterialGUIDs(dt, paths), materialAssets, shaderAssets, textureAssets),
		Geometries: collectMeshGeometries(collectMeshGUIDs(bindings), meshAssets),
	}
	if len(data.Bindings) == 0 && len(data.Materials) == 0 {
		return
	}
	writeJSON("meshes.json", data)
	fmt.Printf("meshes: %d bindings, %d materials, %d geometry assets\n", len(data.Bindings), len(data.Materials), len(data.Geometries))
	_ = idx // kept in signature symmetry with other scene exporters; mesh paths are resolved from paths.
}

func collectMeshGUIDs(bindings []kmdata.MeshBinding) map[string]bool {
	out := map[string]bool{}
	for _, b := range bindings {
		if b.Mesh.GUID != "" && b.Mesh.GUID != "0" {
			out[b.Mesh.GUID] = true
		}
	}
	return out
}

func collectMeshGeometries(wanted map[string]bool, meshAssets assetIndex) map[string][]kmdata.MeshGeometry {
	if len(wanted) == 0 {
		return nil
	}
	out := map[string][]kmdata.MeshGeometry{}
	for guid := range wanted {
		ref, ok := meshAssets.refs[guid]
		if !ok || ref.Path == "" || !strings.HasSuffix(strings.ToLower(ref.Path), ".fbx") {
			continue
		}
		path := filepath.Join(*hsRoot, "Assets", filepath.FromSlash(ref.Path))
		geoms, err := parseFBXGeometries(path)
		if err != nil || len(geoms) == 0 {
			continue
		}
		out[guid] = geoms
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type rendererDoc struct {
	id      int64
	classID int
	content map[string]any
}

func collectMeshBindings(dt *docTable, paths map[int64]string, meshAssets, materialAssets assetIndex) []kmdata.MeshBinding {
	filters := map[int64]map[string]any{}
	renderers := map[int64]rendererDoc{}
	for id, d := range dt.byID {
		gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
		switch d.classID {
		case 33: // MeshFilter
			filters[gid] = d.content
		case 23, 137: // MeshRenderer / SkinnedMeshRenderer
			renderers[gid] = rendererDoc{id: id, classID: d.classID, content: d.content}
		}
	}

	var out []kmdata.MeshBinding
	for gid, r := range renderers {
		path, ok := paths[gid]
		if !ok {
			continue
		}
		meshRef := map[string]any(nil)
		rendererName := "MeshRenderer"
		if r.classID == 137 {
			rendererName = "SkinnedMeshRenderer"
			meshRef = uy.M(r.content["m_Mesh"])
		} else if f := filters[gid]; f != nil {
			meshRef = uy.M(f["m_Mesh"])
		}
		mesh := assetRefFromYAML(meshRef, meshAssets)
		if mesh.GUID == "" && mesh.FileID == 0 {
			continue
		}
		out = append(out, kmdata.MeshBinding{
			Path:      path,
			Renderer:  rendererName,
			Mesh:      mesh,
			Materials: materialRefs(r.content, materialAssets),
			Enabled:   uy.I(r.content["m_Enabled"]) != 0,
			Layer:     int(uy.I(r.content["m_SortingLayer"])),
			Order:     int(uy.I(r.content["m_SortingOrder"])),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func collectMaterialGUIDs(dt *docTable, paths map[int64]string) map[string]bool {
	out := map[string]bool{}
	for _, d := range dt.byID {
		if d.classID != 23 && d.classID != 137 {
			continue
		}
		gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
		if _, ok := paths[gid]; !ok {
			continue
		}
		for _, mat := range materialRefs(d.content, assetIndex{}) {
			if mat.GUID != "" {
				out[mat.GUID] = true
			}
		}
	}
	return out
}

func materialRefs(renderer map[string]any, assets assetIndex) []kmdata.AssetRef {
	var out []kmdata.AssetRef
	for _, mv := range uy.L(renderer["m_Materials"]) {
		ref := assetRefFromYAML(uy.M(mv), assets)
		if ref.GUID != "" || ref.FileID != 0 {
			out = append(out, ref)
		}
	}
	return out
}

func assetRefFromYAML(ref map[string]any, assets assetIndex) kmdata.AssetRef {
	if ref == nil {
		return kmdata.AssetRef{}
	}
	fileID := uy.I(ref["fileID"])
	guid := uy.S(ref["guid"])
	out := kmdata.AssetRef{FileID: fileID, GUID: guid}
	if a, ok := assets.refs[guid]; ok {
		out.Name = a.Name
		out.Path = a.Path
	}
	return out
}

func collectMeshMaterials(wanted map[string]bool, materialAssets, shaderAssets, textureAssets assetIndex) map[string]kmdata.Material {
	if len(wanted) == 0 {
		return nil
	}
	out := map[string]kmdata.Material{}
	for guid := range wanted {
		ref, ok := materialAssets.refs[guid]
		if !ok || ref.Path == "" {
			continue
		}
		path := filepath.Join(*hsRoot, "Assets", filepath.FromSlash(ref.Path))
		mat, ok := parseMaterialAsset(path, guid, ref, shaderAssets, textureAssets)
		if ok {
			out[guid] = mat
		}
	}
	return out
}

func parseMaterialAsset(path, guid string, ref kmdata.AssetRef, shaderAssets, textureAssets assetIndex) (kmdata.Material, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return kmdata.Material{}, false
	}
	docs, err := uy.Parse(raw)
	if err != nil {
		return kmdata.Material{}, false
	}
	for i := range docs {
		if docs[i].ClassID != 21 {
			continue
		}
		c := docs[i].Content()
		mat := kmdata.Material{
			Name:   firstNonEmptyString(uy.S(c["m_Name"]), ref.Name),
			GUID:   guid,
			Path:   ref.Path,
			Shader: assetRefFromYAML(uy.M(c["m_Shader"]), shaderAssets),
		}
		props := uy.M(c["m_SavedProperties"])
		for _, tv := range uy.L(props["m_TexEnvs"]) {
			for name, val := range uy.M(tv) {
				env := uy.M(val)
				tex := assetRefFromYAML(uy.M(env["m_Texture"]), textureAssets)
				if tex.GUID == "" && tex.FileID == 0 {
					continue
				}
				if mat.Textures == nil {
					mat.Textures = map[string]kmdata.TextureEnv{}
				}
				mat.Textures[name] = kmdata.TextureEnv{
					Texture: tex,
					Image:   copyMeshTexture(tex),
					Scale:   [2]float64{uy.F(uy.Get(env, "m_Scale", "x")), uy.F(uy.Get(env, "m_Scale", "y"))},
					Offset:  [2]float64{uy.F(uy.Get(env, "m_Offset", "x")), uy.F(uy.Get(env, "m_Offset", "y"))},
				}
			}
		}
		for _, fv := range uy.L(props["m_Floats"]) {
			for name, val := range uy.M(fv) {
				if mat.Floats == nil {
					mat.Floats = map[string]float64{}
				}
				mat.Floats[name] = uy.F(val)
			}
		}
		for _, cv := range uy.L(props["m_Colors"]) {
			for name, val := range uy.M(cv) {
				cm := uy.M(val)
				if mat.Colors == nil {
					mat.Colors = map[string][4]float64{}
				}
				mat.Colors[name] = [4]float64{
					uy.F(cm["r"]), uy.F(cm["g"]), uy.F(cm["b"]), uy.F(cm["a"]),
				}
			}
		}
		return mat, true
	}
	return kmdata.Material{}, false
}

func copyMeshTexture(ref kmdata.AssetRef) string {
	if ref.Path == "" || ref.GUID == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(ref.Path))
	switch ext {
	case ".png", ".jpg", ".jpeg":
	default:
		return ""
	}
	src := filepath.Join(*hsRoot, "Assets", filepath.FromSlash(ref.Path))
	dstRel := filepath.ToSlash(filepath.Join("meshtex", ref.GUID+ext))
	dst := filepath.Join(*outDir, filepath.FromSlash(dstRel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return ""
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return ""
	}
	return dstRel
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
