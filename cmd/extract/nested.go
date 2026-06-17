// nested.go：嵌套 prefab（PrefabInstance，classID 1001）的展开。
//
// Unity 主 prefab 通过 PrefabInstance 引用子 prefab（marchingOrders 的
// 4 个 cadet 都是 Prefabs/Cadets.prefab 的实例）：
//   - 子 prefab 文档整体并入，fileID 重映射（避免多实例冲突）
//   - 主 prefab 里的 stripped 文档是实例内对象的"别名锚"——主侧引用
//     （父子链、脚本字段）都指向它，因此重映射时优先复用 stripped id
//   - 子 prefab 根 Transform 的 m_Father 接到 m_TransformParent
//   - m_Modifications 按 propertyPath 写回（位置/旋转/名字/激活/排序/
//     m_Sprite 等 objectReference 覆盖）
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	uy "hsdemo/unityyaml"
)

var nestedNextID int64 = 1 << 40 // 展开实例的 fileID 命名空间（远离普通 id）

const explicitArraySizePrefix = "__heaven_go_array_size__:"

type strippedDoc struct {
	id, inst, src int64
	classID       int
	script        string // classID 114 时的 m_Script guid
}

// expandPrefab 读取并展开 prefab（递归处理子 prefab 的嵌套）。
// prefabGUIDs：guid → prefab 路径（解析 m_SourcePrefab 用）。
func expandPrefab(path string, prefabGUIDs map[string]string) ([]uy.Doc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	docs, err := uy.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	// stripped 锚：(instanceID, 源 fileID) → stripped fileID
	strippedOf := map[[2]int64]int64{}
	var strippedAll []strippedDoc
	for i := range docs {
		if !docs[i].Stripped {
			continue
		}
		c := docs[i].Content()
		src := uy.I(uy.Get(uy.M(c["m_CorrespondingSourceObject"]), "fileID"))
		inst := uy.I(uy.Get(uy.M(c["m_PrefabInstance"]), "fileID"))
		if src != 0 && inst != 0 {
			strippedOf[[2]int64{inst, src}] = docs[i].FileID
			strippedAll = append(strippedAll, strippedDoc{
				id: docs[i].FileID, inst: inst, src: src, classID: docs[i].ClassID,
				script: uy.S(uy.Get(uy.M(c["m_Script"]), "guid")),
			})
		}
	}
	consumed := map[int64]bool{}  // 已被 remap 复用的 stripped id
	instDocs := map[int64][]int{} // 实例 id → out 下标（实例归属）

	var out []uy.Doc
	// 待回填的父子链（父 m_Children 不含实例根的情形——cheerReaders 的
	// pepSquad girls 仅靠 m_TransformParent 挂接）
	type pend struct{ child, parent int64 }
	var pends []pend
	for i := range docs {
		if docs[i].Stripped {
			continue // 由展开文档顶替其 fileID
		}
		if docs[i].ClassID != 1001 {
			out = append(out, docs[i])
			continue
		}
		// PrefabInstance
		instID := docs[i].FileID
		mod := uy.M(docs[i].Content()["m_Modification"])
		srcGUID := uy.S(uy.Get(uy.M(docs[i].Content()["m_SourcePrefab"]), "guid"))
		srcPath, ok := prefabGUIDs[srcGUID]
		if !ok {
			srcDocs, rootTF, rootOK := synthesizeModelPrefabInstance(instID, srcGUID, mod, strippedAll)
			if !rootOK {
				log.Printf("warn: 嵌套 prefab guid %s 未找到（实例 &%d 跳过）", srcGUID, instID)
				continue
			}
			for j := range srcDocs {
				instDocs[instID] = append(instDocs[instID], len(out))
				out = append(out, srcDocs[j])
			}
			if parentTF := uy.I(uy.Get(uy.M(mod["m_TransformParent"]), "fileID")); parentTF != 0 {
				pends = append(pends, pend{rootTF, parentTF})
			}
			continue
		}
		srcDocs, err := expandPrefab(srcPath, prefabGUIDs)
		if err != nil {
			return nil, err
		}
		// fileID 重映射（stripped 锚优先）
		remap := map[int64]int64{}
		for j := range srcDocs {
			old := srcDocs[j].FileID
			if sid, ok := strippedOf[[2]int64{instID, old}]; ok {
				remap[old] = sid
				consumed[sid] = true
			} else {
				remap[old] = nestedNextID
				nestedNextID++
			}
		}
		parentTF := uy.I(uy.Get(uy.M(mod["m_TransformParent"]), "fileID"))
		var rootTF, rootGO int64
		for j := range srcDocs {
			d := srcDocs[j]
			d.FileID = remap[d.FileID]
			remapRefs(d.Content(), remap)
			instDocs[instID] = append(instDocs[instID], len(out))
			if d.ClassID == 4 || d.ClassID == 224 {
				c := d.Content()
				fa := uy.I(uy.Get(uy.M(c["m_Father"]), "fileID"))
				if fa == 0 { // 子 prefab 根：接到实例的 TransformParent
					c["m_Father"] = map[string]any{"fileID": parentTF}
					pends = append(pends, pend{d.FileID, parentTF})
					rootTF = d.FileID
					rootGO = uy.I(uy.Get(uy.M(c["m_GameObject"]), "fileID"))
				}
			}
			out = append(out, d)
		}
		// 应用修改
		byID := map[int64]map[string]any{}
		for j := range out {
			byID[out[j].FileID] = out[j].Content()
		}
		unknownTargetProps := map[int64]map[string]bool{}
		for _, mv := range uy.L(mod["m_Modifications"]) {
			m := uy.M(mv)
			raw := uy.I(uy.Get(uy.M(m["target"]), "fileID"))
			if raw == 0 || remap[raw] != 0 {
				continue
			}
			if unknownTargetProps[raw] == nil {
				unknownTargetProps[raw] = map[string]bool{}
			}
			unknownTargetProps[raw][uy.S(m["propertyPath"])] = true
		}
		for _, mv := range uy.L(mod["m_Modifications"]) {
			m := uy.M(mv)
			rawTarget := uy.I(uy.Get(uy.M(m["target"]), "fileID"))
			tgt := remap[rawTarget]
			content := byID[tgt]
			pp := uy.S(m["propertyPath"])
			if content == nil {
				// 变体链的合成 fileID 在 remap 中不存在（cheerReaders 的
				// pepSquadN 位置覆盖）：只有整套实例 root placement 才回退
				// 到 root。子节点的单轴覆盖如果也写到 root，会把三排女孩
				// 的位置和比例混到一起。
				var fb int64
				switch {
				case strings.HasPrefix(pp, "m_Local") || strings.HasPrefix(pp, "m_RootOrder") ||
					strings.HasPrefix(pp, "m_Euler") || strings.HasPrefix(pp, "m_Anchored") ||
					strings.HasPrefix(pp, "m_SizeDelta"):
					if looksLikeRootPlacement(unknownTargetProps[rawTarget]) {
						fb = rootTF
					}
				case pp == "m_Name" || pp == "m_IsActive" || pp == "m_Layer":
					fb = rootGO
				}
				if fb != 0 {
					content = byID[fb]
				}
				if content == nil {
					continue
				}
			}
			if objRef := uy.M(m["objectReference"]); objRef != nil &&
				(uy.I(objRef["fileID"]) != 0 || uy.S(objRef["guid"]) != "") {
				setPropertyPath(content, pp, map[string]any{
					"fileID": uy.I(objRef["fileID"]), "guid": uy.S(objRef["guid"]),
				})
				continue
			}
			setPropertyPath(content, pp, m["value"])
		}
		clearExplicitArraySizeMarkers(out)
	}
	// 未消费的 stripped 锚（变体链的合成 fileID 在文本中不存在——
	// cheerReaders 的 pepSquadN 变体 RvlCharacter）：在所属实例的展开
	// 文档里按 classID（+ m_Script guid）唯一匹配建别名，重写全部引用。
	alias := map[int64]int64{}
	for _, sd := range strippedAll {
		if consumed[sd.id] {
			continue
		}
		var hit int64
		hits := 0
		for _, oi := range instDocs[sd.inst] {
			d := &out[oi]
			if d.ClassID != sd.classID {
				continue
			}
			if sd.classID == 114 &&
				uy.S(uy.Get(uy.M(d.Content()["m_Script"]), "guid")) != sd.script {
				continue
			}
			hit = d.FileID
			hits++
		}
		if hits == 1 {
			alias[sd.id] = hit
		} else if hits > 1 {
			log.Printf("warn: stripped &%d 在实例 &%d 中按脚本匹配到 %d 个候选，跳过", sd.id, sd.inst, hits)
		}
	}
	if len(alias) > 0 {
		for j := range out {
			remapRefs(out[j].Content(), alias)
		}
	}

	// 回填父 transform 的 m_Children（exportScene 按 m_Children 遍历）
	if len(pends) > 0 {
		byID := map[int64]map[string]any{}
		for j := range out {
			if out[j].ClassID == 4 || out[j].ClassID == 224 {
				byID[out[j].FileID] = out[j].Content()
			}
		}
		for _, p := range pends {
			pc, ok := byID[p.parent]
			if !ok {
				continue
			}
			kids := uy.L(pc["m_Children"])
			found := false
			for _, kv := range kids {
				if uy.I(uy.Get(uy.M(kv), "fileID")) == p.child {
					found = true
					break
				}
			}
			if !found {
				pc["m_Children"] = append(kids, map[string]any{"fileID": p.child})
			}
		}
	}
	return out, nil
}

func looksLikeRootPlacement(props map[string]bool) bool {
	if props == nil {
		return false
	}
	return props["m_RootOrder"] &&
		props["m_LocalPosition.x"] &&
		props["m_LocalPosition.y"]
}

// synthesizeModelPrefabInstance preserves the scene/reference surface for
// binary FBX model prefabs. Unity stores model prefab hierarchy inside the FBX
// importer, not as YAML docs, but the owning prefab still contains stripped
// anchors and override modifications. Building a minimal root GameObject /
// Transform plus referenced component stubs keeps script fields, Animator
// bindings, and parent-child placement auditable until a full FBX geometry
// importer is added.
func synthesizeModelPrefabInstance(instID int64, srcGUID string, mod map[string]any, stripped []strippedDoc) ([]uy.Doc, int64, bool) {
	var inst []strippedDoc
	bySrc := map[int64]strippedDoc{}
	for _, sd := range stripped {
		if sd.inst != instID {
			continue
		}
		inst = append(inst, sd)
		bySrc[sd.src] = sd
	}
	if len(inst) == 0 {
		return nil, 0, false
	}

	rootTFSrc := int64(0)
	rootGOSrc := int64(0)
	rootGOName := ""
	for _, mv := range uy.L(mod["m_Modifications"]) {
		m := uy.M(mv)
		src := uy.I(uy.Get(uy.M(m["target"]), "fileID"))
		pp := uy.S(m["propertyPath"])
		if pp == "m_RootOrder" {
			if sd, ok := bySrc[src]; ok && (sd.classID == 4 || sd.classID == 224) {
				rootTFSrc = src
			}
		}
		if pp == "m_Name" {
			if sd, ok := bySrc[src]; ok && sd.classID == 1 {
				rootGOSrc = src
			}
			if rootGOName == "" {
				rootGOName = uy.S(m["value"])
			}
		}
	}
	if rootTFSrc == 0 {
		for _, sd := range inst {
			if sd.classID == 4 || sd.classID == 224 {
				rootTFSrc = sd.src
				break
			}
		}
	}
	if rootGOSrc == 0 {
		for _, sd := range inst {
			if sd.classID == 1 {
				rootGOSrc = sd.src
				break
			}
		}
	}
	rootTF, okTF := bySrc[rootTFSrc]
	rootGO, okGO := bySrc[rootGOSrc]
	if !okTF {
		return nil, 0, false
	}
	if !okGO {
		// Some imported FBX prefab instances only expose stripped Transform and
		// Renderer anchors in the owning prefab. Create a synthetic GameObject so
		// those components still resolve to a stable scene path.
		rootGO = strippedDoc{id: nestedNextID, inst: instID, src: rootGOSrc, classID: 1}
		nestedNextID++
	}

	contents := map[int64]map[string]any{}
	classes := map[int64]int{}
	out := []uy.Doc{
		{
			ClassID: rootGO.classID,
			FileID:  rootGO.id,
			Root: map[string]any{"GameObject": map[string]any{
				"m_Name":     rootGOName,
				"m_IsActive": 1,
			}},
		},
		{
			ClassID: rootTF.classID,
			FileID:  rootTF.id,
			Root: map[string]any{unityClassName(rootTF.classID): map[string]any{
				"m_GameObject":    map[string]any{"fileID": rootGO.id},
				"m_LocalRotation": map[string]any{"x": 0, "y": 0, "z": 0, "w": 1},
				"m_LocalPosition": map[string]any{"x": 0, "y": 0, "z": 0},
				"m_LocalScale":    map[string]any{"x": 1, "y": 1, "z": 1},
				"m_Children":      []any{},
				"m_Father":        map[string]any{"fileID": uy.I(uy.Get(uy.M(mod["m_TransformParent"]), "fileID"))},
			}},
		},
	}
	if rootGOSrc != 0 {
		contents[rootGOSrc] = out[0].Content()
		classes[rootGOSrc] = out[0].ClassID
	}
	contents[rootTFSrc] = out[1].Content()
	classes[rootTFSrc] = out[1].ClassID

	for _, sd := range inst {
		if (rootGOSrc != 0 && sd.src == rootGOSrc) || sd.src == rootTFSrc || sd.classID == 1 || sd.classID == 4 || sd.classID == 224 {
			continue
		}
		kind := unityClassName(sd.classID)
		if kind == "" {
			continue
		}
		doc := uy.Doc{
			ClassID: sd.classID,
			FileID:  sd.id,
			Root: map[string]any{kind: map[string]any{
				"m_GameObject": map[string]any{"fileID": rootGO.id},
			}},
		}
		contents[sd.src] = doc.Content()
		classes[sd.src] = sd.classID
		out = append(out, doc)
	}
	out = synthesizeModelRendererDocs(out, contents, classes, rootGO.id, srcGUID, mod)

	for _, mv := range uy.L(mod["m_Modifications"]) {
		m := uy.M(mv)
		src := uy.I(uy.Get(uy.M(m["target"]), "fileID"))
		content := contents[src]
		if content == nil {
			continue
		}
		pp := uy.S(m["propertyPath"])
		if objRef := uy.M(m["objectReference"]); objRef != nil &&
			(uy.I(objRef["fileID"]) != 0 || uy.S(objRef["guid"]) != "") {
			setPropertyPath(content, pp, map[string]any{
				"fileID": uy.I(objRef["fileID"]), "guid": uy.S(objRef["guid"]),
			})
			continue
		}
		setPropertyPath(content, pp, m["value"])
	}
	clearExplicitArraySizeMarkers(out)

	return out, rootTF.id, true
}

func synthesizeModelRendererDocs(out []uy.Doc, contents map[int64]map[string]any, classes map[int64]int, rootGO int64, srcGUID string, mod map[string]any) []uy.Doc {
	if srcGUID == "" {
		return out
	}
	for _, src := range rendererOverrideTargets(mod) {
		if c := classes[src]; c == 23 || c == 137 {
			ensureModelRendererMesh(contents[src], c, src, srcGUID)
			if c == 23 {
				out = ensureModelMeshFilter(out, rootGO, src, srcGUID)
			}
			continue
		}
		classID := modelRendererClass(mod, src)
		rendererID := nestedNextID
		nestedNextID++
		content := map[string]any{
			"m_GameObject": map[string]any{"fileID": rootGO},
			"m_Enabled":    1,
		}
		ensureModelRendererMesh(content, classID, src, srcGUID)
		out = append(out, uy.Doc{
			ClassID: classID,
			FileID:  rendererID,
			Root:    map[string]any{unityClassName(classID): content},
		})
		contents[src] = content
		classes[src] = classID
		if classID == 23 {
			out = ensureModelMeshFilter(out, rootGO, src, srcGUID)
		}
	}
	return out
}

func ensureModelMeshFilter(out []uy.Doc, rootGO, meshFileID int64, guid string) []uy.Doc {
	for i := range out {
		if out[i].ClassID != 33 {
			continue
		}
		content := out[i].Content()
		if uy.I(uy.Get(content, "m_GameObject", "fileID")) != rootGO {
			continue
		}
		if ref := uy.M(content["m_Mesh"]); ref == nil || (uy.I(ref["fileID"]) == 0 && uy.S(ref["guid"]) == "") {
			content["m_Mesh"] = map[string]any{"fileID": meshFileID, "guid": guid}
		}
		return out
	}
	filterID := nestedNextID
	nestedNextID++
	filter := map[string]any{
		"m_GameObject": map[string]any{"fileID": rootGO},
		"m_Mesh":       map[string]any{"fileID": meshFileID, "guid": guid},
	}
	return append(out, uy.Doc{
		ClassID: 33,
		FileID:  filterID,
		Root:    map[string]any{"MeshFilter": filter},
	})
}

func ensureModelRendererMesh(content map[string]any, classID int, meshFileID int64, guid string) {
	if content == nil {
		return
	}
	if classID == 137 {
		if ref := uy.M(content["m_Mesh"]); ref == nil || (uy.I(ref["fileID"]) == 0 && uy.S(ref["guid"]) == "") {
			content["m_Mesh"] = map[string]any{"fileID": meshFileID, "guid": guid}
		}
	}
}

func rendererOverrideTargets(mod map[string]any) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, mv := range uy.L(mod["m_Modifications"]) {
		m := uy.M(mv)
		pp := uy.S(m["propertyPath"])
		if !looksLikeRendererProperty(pp) {
			continue
		}
		src := uy.I(uy.Get(uy.M(m["target"]), "fileID"))
		if src == 0 || seen[src] {
			continue
		}
		seen[src] = true
		out = append(out, src)
	}
	return out
}

func looksLikeRendererProperty(path string) bool {
	return strings.HasPrefix(path, "m_Materials.") ||
		strings.HasPrefix(path, "m_CastShadows") ||
		strings.HasPrefix(path, "m_ReceiveShadows") ||
		strings.HasPrefix(path, "m_DynamicOccludee") ||
		strings.HasPrefix(path, "m_LightProbeUsage") ||
		strings.HasPrefix(path, "m_ReflectionProbeUsage") ||
		strings.HasPrefix(path, "m_SortingLayer") ||
		strings.HasPrefix(path, "m_SortingOrder") ||
		strings.HasPrefix(path, "m_RootBone") ||
		strings.HasPrefix(path, "m_UpdateWhenOffscreen") ||
		strings.HasPrefix(path, "m_SkinnedMotionVectors") ||
		strings.HasPrefix(path, "m_Bones") ||
		strings.HasPrefix(path, "m_BlendShape")
}

func modelRendererClass(mod map[string]any, src int64) int {
	for _, mv := range uy.L(mod["m_Modifications"]) {
		m := uy.M(mv)
		if uy.I(uy.Get(uy.M(m["target"]), "fileID")) != src {
			continue
		}
		switch pp := uy.S(m["propertyPath"]); {
		case strings.HasPrefix(pp, "m_RootBone"),
			strings.HasPrefix(pp, "m_UpdateWhenOffscreen"),
			strings.HasPrefix(pp, "m_SkinnedMotionVectors"),
			strings.HasPrefix(pp, "m_Bones"),
			strings.HasPrefix(pp, "m_BlendShape"):
			return 137
		}
	}
	return 23
}

func unityClassName(classID int) string {
	switch classID {
	case 1:
		return "GameObject"
	case 4:
		return "Transform"
	case 23:
		return "MeshRenderer"
	case 33:
		return "MeshFilter"
	case 95:
		return "Animator"
	case 137:
		return "SkinnedMeshRenderer"
	case 224:
		return "RectTransform"
	default:
		return ""
	}
}

// remapRefs 重写文档内部引用（无 guid 的 {fileID: N}）。
func remapRefs(v any, remap map[int64]int64) {
	switch tv := v.(type) {
	case map[string]any:
		if fid, ok := tv["fileID"]; ok {
			if _, hasGUID := tv["guid"]; !hasGUID {
				if nid, ok2 := remap[uy.I(fid)]; ok2 {
					tv["fileID"] = nid
				}
			}
			return
		}
		for _, vv := range tv {
			remapRefs(vv, remap)
		}
	case []any:
		for _, vv := range tv {
			remapRefs(vv, remap)
		}
	}
}

// setPropertyPath 按 Unity propertyPath 写值（"m_LocalPosition.x"、"m_Name"、
// "m_Materials.Array.data[0]" 等；数组路径与未知层级按跳过处理并打日志）。
func setPropertyPath(content map[string]any, path string, value any) {
	if setTopLevelArrayPath(content, path, value) {
		return
	}
	parts := strings.Split(path, ".")
	cur := content
	for i, p := range parts {
		last := i == len(parts)-1
		if strings.Contains(p, "[") || p == "Array" {
			// 数组修改（极少见，出现时记录）
			if p == "Array" {
				continue
			}
			log.Printf("warn: propertyPath %q 含数组下标，未支持", path)
			return
		}
		if last {
			cur[p] = normalizeModValue(value)
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

// setTopLevelArrayPath handles Unity prefab overrides like
// "m_Materials.Array.data[0]". Renderer materials are stored as YAML arrays;
// treating this as a nested map drops mapped-material overrides from nested
// prefab instances such as Moai Doo-Wop's male/female Moai prefabs.
func setTopLevelArrayPath(content map[string]any, path string, value any) bool {
	const sizeSuffix = ".Array.size"
	if strings.HasSuffix(path, sizeSuffix) {
		field := strings.TrimSuffix(path, sizeSuffix)
		size := int(uy.I(normalizeModValue(value)))
		if field == "" || size < 0 {
			return false
		}
		arr := uy.L(content[field])
		for len(arr) < size {
			arr = append(arr, map[string]any{"fileID": 0})
		}
		if len(arr) > size {
			arr = arr[:size]
		}
		content[field] = arr
		content[explicitArraySizePrefix+field] = size
		return true
	}

	const marker = ".Array.data["
	i := strings.Index(path, marker)
	if i < 0 || !strings.HasSuffix(path, "]") {
		return false
	}
	field := path[:i]
	idxText := strings.TrimSuffix(path[i+len(marker):], "]")
	idx, err := strconv.Atoi(idxText)
	if err != nil || field == "" || idx < 0 {
		return false
	}
	if size, ok := content[explicitArraySizePrefix+field]; ok && idx >= int(uy.I(size)) {
		return true
	}
	arr := uy.L(content[field])
	for len(arr) <= idx {
		arr = append(arr, map[string]any{"fileID": 0})
	}
	arr[idx] = normalizeModValue(value)
	content[field] = arr
	return true
}

func clearExplicitArraySizeMarkers(docs []uy.Doc) {
	for i := range docs {
		clearExplicitArraySizeMarkersIn(docs[i].Content())
	}
}

func clearExplicitArraySizeMarkersIn(v any) {
	switch tv := v.(type) {
	case map[string]any:
		for k, vv := range tv {
			if strings.HasPrefix(k, explicitArraySizePrefix) {
				delete(tv, k)
				continue
			}
			clearExplicitArraySizeMarkersIn(vv)
		}
	case []any:
		for _, vv := range tv {
			clearExplicitArraySizeMarkersIn(vv)
		}
	}
}

// normalizeModValue 把修改值转为数值（YAML 解析可能给 string）。
func normalizeModValue(v any) any {
	if s, ok := v.(string); ok {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return v
}
