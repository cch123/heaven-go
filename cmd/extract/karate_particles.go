package main

import (
	"fmt"
	"os"
	"sort"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

func exportKarateParticles(tables map[string]*spriteTable) {
	raw, err := os.ReadFile(gamePath("karateman.prefab"))
	must(err)
	docs, err := uy.Parse(raw)
	must(err)

	dt := &docTable{byID: map[int64]*docRef{}}
	names := map[int64]string{}
	tfByID := map[int64]map[string]any{}
	tfOwner := map[int64]int64{}
	for i := range docs {
		d := &docs[i]
		c := d.Content()
		dt.byID[d.FileID] = &docRef{classID: d.ClassID, content: c}
		switch d.ClassID {
		case 1:
			names[d.FileID] = uy.S(c["m_Name"])
		case 4:
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			tfByID[d.FileID] = c
			tfOwner[d.FileID] = gid
		}
	}

	paths := legacyKaratePaths(names, tfByID, tfOwner)
	materialAssets := scanAssetIndex(gamePath("Sprites"), ".mat")
	meshAssets := scanAssetIndex(gamePath("Sprites"), ".fbx", ".obj", ".asset")
	data := kmdata.ParticleData{
		Systems: collectParticleSystems(dt, paths, tables, materialAssets, meshAssets),
	}
	if len(data.Systems) == 0 {
		return
	}
	writeJSON("particles.json", data)
	fmt.Printf("particles: %d systems\n", len(data.Systems))
}

func legacyKaratePaths(names map[int64]string, tfByID map[int64]map[string]any, tfOwner map[int64]int64) map[int64]string {
	children := map[int64][]int64{}
	roots := []int64{}
	for tfID, tf := range tfByID {
		father := uy.I(uy.Get(tf, "m_Father", "fileID"))
		if father == 0 || tfByID[father] == nil {
			roots = append(roots, tfID)
			continue
		}
		children[father] = append(children[father], tfID)
	}
	sort.Slice(roots, func(i, j int) bool {
		return names[tfOwner[roots[i]]] < names[tfOwner[roots[j]]]
	})
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool {
			return names[tfOwner[children[parent][i]]] < names[tfOwner[children[parent][j]]]
		})
	}

	paths := map[int64]string{}
	var walk func(tfID int64, path string)
	walk = func(tfID int64, path string) {
		gid := tfOwner[tfID]
		name := names[gid]
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}
		paths[gid] = childPath
		for _, child := range children[tfID] {
			walk(child, childPath)
		}
	}
	for _, root := range roots {
		walk(root, "")
	}
	return paths
}
