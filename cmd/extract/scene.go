// scene.go：整游戏场景的泛化提取模式（-game rhythmSomen 等）。
// 与 KarateMan 的单骨架模式不同，这里导出 prefab 的完整节点树、
// 全部 AnimationClip、多张图集，以及游戏脚本字段 → 节点 path 的绑定表。
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

type sceneSpec struct {
	dir        string   // Assets/Bundled/Games/<dir>
	prefab     string   // prefab 文件名
	roleFields []string // 游戏 MonoBehaviour 中需要解析的 Animator/GameObject 引用字段

	refArrayFields []string // 引用数组字段（如对象模板表）
	strArrayFields []string // 字符串数组字段（如动画名表）
	curveFields    []string // BezierCurve3D 引用字段
	objMarkers     []string // 识别"对象模板组件"的字段集合（如 MobTrickObj）
	wantSequences  bool     // 提取 SoundSequences 组件
	commonSounds   []string // 需要的公共音效（Assets/Resources/Sfx/<name>）
}

var sceneSpecs = map[string]sceneSpec{
	"rhythmSomen": {
		dir:    "RhythmSomen",
		prefab: "rhythmSomen.prefab",
		roleFields: []string{
			"SomenPlayer", "FrontArm", "backArm", "EffectHit", "EffectSweat",
			"EffectExclam", "EffectShock", "CloseCrane", "FarCrane",
		},
	},
	"trickClass": {
		dir:            "TrickClass",
		prefab:         "trickClass.prefab",
		roleFields:     []string{"playerAnim", "girlAnim", "warnAnim", "objHolder"},
		refArrayFields: []string{"objPrefab", "objPrefabVariant"},
		strArrayFields: []string{"objWarnAnim", "objWarnAnimVariant", "objThrowAnim"},
		curveFields:    []string{"ballTossCurve", "ballMissCurve", "planeTossCurve", "planeMissCurve", "shockTossCurve"},
		objMarkers:     []string{"flyBeats", "dodgeBeats"},
		wantSequences:  true,
		commonSounds:   []string{"miss.wav"},
	},
}

func bundlePath(dir string, parts ...string) string {
	return filepath.Join(append([]string{*hsRoot, "Assets", "Bundled", "Games", dir}, parts...)...)
}

func extractScene(game string) {
	spec, ok := sceneSpecs[game]
	if !ok {
		log.Fatalf("unknown game %q (known: karateman, rhythmSomen)", game)
	}
	must(os.MkdirAll(filepath.Join(*outDir, "sounds"), 0o755))

	tables := scanSpriteMetas(bundlePath(spec.dir, "Sprites"))
	exportSheetMulti(tables)
	idx, docs := buildPrefabIndex(bundlePath(spec.dir, spec.prefab))
	paths := exportScene(idx, tables)
	exportRoles(spec, docs, idx, paths)
	exportExtra(spec, docs, idx, paths)
	exportAnimDir(bundlePath(spec.dir, "Sprites"), tables)
	copySounds(bundlePath(spec.dir, "Sounds"))
	for _, name := range spec.commonSounds {
		b, err := os.ReadFile(filepath.Join(*hsRoot, "Assets", "Resources", "Sfx", name))
		must(err)
		// 公共音效加 common_ 前缀避免与游戏音效重名
		must(os.WriteFile(filepath.Join(*outDir, "sounds", "common_"+name), b, 0o644))
	}
	fmt.Println("done.")
}

// ---------- 多图集 ----------

func exportSheetMulti(tables map[string]*spriteTable) {
	guids := make([]string, 0, len(tables))
	for g := range tables {
		guids = append(guids, g)
	}
	sort.Slice(guids, func(i, j int) bool { return tables[guids[i]].pngPath < tables[guids[j]].pngPath })

	sheet := &kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}}
	for _, g := range guids {
		t := tables[g]
		atlasIdx := len(sheet.Atlases)
		name := fmt.Sprintf("atlas%d.png", atlasIdx)
		raw, err := os.ReadFile(t.pngPath)
		must(err)
		must(os.WriteFile(filepath.Join(*outDir, name), raw, 0o644))
		sheet.Atlases = append(sheet.Atlases, name)

		if len(t.sheet) == 0 {
			// 单 sprite 贴图：整图作为一个切片
			base := strings.TrimSuffix(filepath.Base(t.pngPath), ".png")
			sheet.Sprites[base] = kmdata.SpriteInfo{
				X: 0, Y: 0, W: t.texW, H: t.texH,
				PivotX: 0.5, PivotY: 0.5, Atlas: atlasIdx, PPU: t.ppu,
			}
			continue
		}
		for sname, sp := range t.sheet {
			sp.Atlas = atlasIdx
			sp.PPU = t.ppu
			sheet.Sprites[sname] = sp
		}
	}
	writeJSON("sprites.json", sheet)
	fmt.Printf("sheet: %d atlases, %d sprites\n", len(sheet.Atlases), len(sheet.Sprites))
}

// ---------- prefab 全树 ----------

type docTable struct {
	byID map[int64]*docRef
}
type docRef struct {
	classID int
	content map[string]any
}

func buildPrefabIndex(prefabPath string) (*prefabIndex, *docTable) {
	raw, err := os.ReadFile(prefabPath)
	must(err)
	docs, err := uy.Parse(raw)
	must(err)
	fmt.Printf("prefab: %d documents\n", len(docs))

	idx := &prefabIndex{
		goName: map[int64]string{}, tfByGO: map[int64]map[string]any{},
		tfByID: map[int64]map[string]any{}, tfOwner: map[int64]int64{},
		rendByGO: map[int64]map[string]any{}, goActive: map[int64]bool{},
		groupByGO: map[int64][]int{},
	}
	dt := &docTable{byID: map[int64]*docRef{}}
	for i := range docs {
		d := &docs[i]
		c := d.Content()
		dt.byID[d.FileID] = &docRef{classID: d.ClassID, content: c}
		switch d.ClassID {
		case 1: // GameObject
			idx.goName[d.FileID] = uy.S(c["m_Name"])
			idx.goActive[d.FileID] = uy.I(c["m_IsActive"]) != 0
		case 4: // Transform
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.tfByGO[gid] = c
			idx.tfByID[d.FileID] = c
			idx.tfOwner[d.FileID] = gid
		case 212: // SpriteRenderer
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.rendByGO[gid] = c
		case 210: // SortingGroup
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.groupByGO[gid] = []int{int(uy.I(c["m_SortingLayer"])), int(uy.I(c["m_SortingOrder"]))}
		}
	}
	return idx, dt
}

// exportScene 导出整棵节点树，返回 GameObject fileID → 节点 path（供 roles 解析）。
func exportScene(idx *prefabIndex, tables map[string]*spriteTable) map[int64]string {
	// 根 Transform：m_Father 不在本 prefab 内
	var rootTF map[string]any
	for tfID, tf := range idx.tfByID {
		father := uy.I(uy.Get(tf, "m_Father", "fileID"))
		if father == 0 || idx.tfByID[father] == nil {
			if rootTF != nil {
				log.Printf("warn: multiple root transforms, keeping first (extra &%d)", tfID)
				continue
			}
			rootTF = tf
		}
	}
	if rootTF == nil {
		log.Fatal("prefab root transform not found")
	}

	scene := &kmdata.Rig{}
	paths := map[int64]string{}
	var walk func(tf map[string]any, parent int, path string)
	walk = func(tf map[string]any, parent int, path string) {
		gid := uy.I(uy.Get(tf, "m_GameObject", "fileID"))
		paths[gid] = path
		n := kmdata.Node{
			Name:   idx.goName[gid],
			Path:   path,
			Parent: parent,
			Pos: [2]float64{
				uy.F(uy.Get(tf, "m_LocalPosition", "x")),
				uy.F(uy.Get(tf, "m_LocalPosition", "y")),
			},
			PosZ: uy.F(uy.Get(tf, "m_LocalPosition", "z")),
			RotZ: quatToZ(
				uy.F(uy.Get(tf, "m_LocalRotation", "z")),
				uy.F(uy.Get(tf, "m_LocalRotation", "w")),
			),
			Scale: [2]float64{
				uy.F(uy.Get(tf, "m_LocalScale", "x")),
				uy.F(uy.Get(tf, "m_LocalScale", "y")),
			},
			Inactive: !idx.goActive[gid],
		}
		n.SortGroup = idx.groupByGO[gid]
		if r := idx.rendByGO[gid]; r != nil {
			n.Sprite = resolveSprite(tables,
				uy.S(uy.Get(r, "m_Sprite", "guid")), uy.I(uy.Get(r, "m_Sprite", "fileID")))
			n.Order = int(uy.I(r["m_SortingOrder"]))
			n.Layer = int(uy.I(r["m_SortingLayer"]))
			n.Hidden = uy.I(r["m_Enabled"]) == 0
			n.FlipX = uy.I(r["m_FlipX"]) != 0
			n.FlipY = uy.I(r["m_FlipY"]) != 0
			n.DrawMode = int(uy.I(r["m_DrawMode"]))
			n.Size = [2]float64{uy.F(uy.Get(r, "m_Size", "x")), uy.F(uy.Get(r, "m_Size", "y"))}
			n.Color = [4]float64{
				uy.F(uy.Get(r, "m_Color", "r")), uy.F(uy.Get(r, "m_Color", "g")),
				uy.F(uy.Get(r, "m_Color", "b")), uy.F(uy.Get(r, "m_Color", "a")),
			}
		}
		self := len(scene.Nodes)
		scene.Nodes = append(scene.Nodes, n)

		for _, cv := range uy.L(tf["m_Children"]) {
			cid := uy.I(uy.Get(uy.M(cv), "fileID"))
			ct := idx.tfByID[cid]
			if ct == nil {
				continue
			}
			childName := idx.goName[idx.tfOwner[cid]]
			childPath := childName
			if path != "" {
				childPath = path + "/" + childName
			}
			walk(ct, self, childPath)
		}
	}
	walk(rootTF, -1, "")
	writeJSON("scene.json", scene)
	fmt.Printf("scene: %d nodes\n", len(scene.Nodes))
	return paths
}

// ---------- roles ----------

func exportRoles(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string) {
	// 找到包含全部 role 字段的 MonoBehaviour（即游戏主脚本）
	var script map[string]any
	for _, d := range dt.byID {
		if d.classID != 114 {
			continue
		}
		hits := 0
		for _, f := range spec.roleFields {
			if _, ok := d.content[f]; ok {
				hits++
			}
		}
		if hits == len(spec.roleFields) {
			script = d.content
			break
		}
	}
	if script == nil {
		log.Fatalf("game script with fields %v not found in prefab", spec.roleFields)
	}

	roles := kmdata.Roles{}
	for _, f := range spec.roleFields {
		fid := uy.I(uy.Get(uy.M(script[f]), "fileID"))
		ref := dt.byID[fid]
		if ref == nil {
			log.Printf("warn: role %s -> &%d not found", f, fid)
			continue
		}
		gid := fid
		if ref.classID != 1 { // 组件引用（如 Animator）→ 其 GameObject
			gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
		}
		p, ok := paths[gid]
		if !ok {
			log.Printf("warn: role %s GameObject &%d not in scene tree", f, gid)
			continue
		}
		roles[f] = p
	}
	writeJSON("roles.json", roles)
	for _, f := range spec.roleFields {
		fmt.Printf("role %-13s -> %q\n", f, roles[f])
	}
}

// ---------- extra（数组引用 / 字符串表 / 曲线 / 对象模板 / 音效序列） ----------

// goPathOf 把组件或 GameObject 引用解析为场景节点 path。
func goPathOf(dt *docTable, paths map[int64]string, fid int64) (string, bool) {
	ref := dt.byID[fid]
	if ref == nil {
		return "", false
	}
	gid := fid
	if ref.classID != 1 {
		gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
	}
	p, ok := paths[gid]
	return p, ok
}

func exportExtra(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string) {
	if len(spec.refArrayFields)+len(spec.strArrayFields)+len(spec.curveFields)+len(spec.objMarkers) == 0 && !spec.wantSequences {
		return
	}
	// 游戏主脚本（与 exportRoles 相同的定位方式）
	var script map[string]any
	for _, d := range dt.byID {
		if d.classID != 114 {
			continue
		}
		hits := 0
		for _, f := range spec.roleFields {
			if _, ok := d.content[f]; ok {
				hits++
			}
		}
		if hits == len(spec.roleFields) && len(spec.roleFields) > 0 {
			script = d.content
			break
		}
	}
	if script == nil {
		log.Fatal("game script not found for extra extraction")
	}

	extra := &kmdata.Extra{
		RefArrays: map[string][]string{},
		Strings:   map[string][]string{},
		Curves:    map[string]kmdata.Curve{},
		ObjNums:   map[string]map[string]float64{},
		ObjStrs:   map[string]map[string]string{},
		Sequences: map[string][]kmdata.SeqClip{},
	}

	for _, f := range spec.refArrayFields {
		for _, rv := range uy.L(script[f]) {
			fid := uy.I(uy.Get(uy.M(rv), "fileID"))
			p, ok := goPathOf(dt, paths, fid)
			if !ok {
				log.Printf("warn: refArray %s -> &%d not in scene", f, fid)
			}
			extra.RefArrays[f] = append(extra.RefArrays[f], p)
		}
	}
	for _, f := range spec.strArrayFields {
		for _, sv := range uy.L(script[f]) {
			extra.Strings[f] = append(extra.Strings[f], uy.S(sv))
		}
	}

	for _, f := range spec.curveFields {
		fid := uy.I(uy.Get(uy.M(script[f]), "fileID"))
		curveDoc := dt.byID[fid]
		if curveDoc == nil {
			log.Printf("warn: curve %s -> &%d missing", f, fid)
			continue
		}
		kps := uy.L(curveDoc.content["keyPoints"])
		if kps == nil {
			kps = uy.L(curveDoc.content["KeyPoints"])
		}
		curve := kmdata.Curve{Sampling: int(uy.I(curveDoc.content["sampling"]))}
		for _, kv := range kps {
			pid := uy.I(uy.Get(uy.M(kv), "fileID"))
			pd := dt.byID[pid]
			if pd == nil {
				continue
			}
			gid := uy.I(uy.Get(pd.content, "m_GameObject", "fileID"))
			var tfID int64
			for id, owner := range idx.tfOwner {
				if owner == gid {
					tfID = id
					break
				}
			}
			lhl := [3]float64{
				uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "x")),
				uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "y")),
				uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "z")),
			}
			rhl := [3]float64{
				uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "x")),
				uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "y")),
				uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "z")),
			}
			curve.Points = append(curve.Points, kmdata.CurvePoint{
				P:  idx.transformPoint3D(tfID, [3]float64{}),
				LH: idx.transformPoint3D(tfID, lhl),
				RH: idx.transformPoint3D(tfID, rhl),
			})
		}
		extra.Curves[f] = curve
	}

	// 对象模板组件（按字段特征识别）
	if len(spec.objMarkers) > 0 {
		for _, d := range dt.byID {
			if d.classID != 114 {
				continue
			}
			all := true
			for _, k := range spec.objMarkers {
				if _, ok := d.content[k]; !ok {
					all = false
					break
				}
			}
			if !all {
				continue
			}
			gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
			p, ok := paths[gid]
			if !ok {
				continue
			}
			nums, strs := map[string]float64{}, map[string]string{}
			for k, v := range d.content {
				if strings.HasPrefix(k, "m_") {
					continue
				}
				switch tv := v.(type) {
				case int, int64, uint64, float64:
					nums[k] = uy.F(v)
				case string:
					strs[k] = tv
				}
			}
			extra.ObjNums[p] = nums
			extra.ObjStrs[p] = strs
		}
	}

	if spec.wantSequences {
		for _, d := range dt.byID {
			if d.classID != 114 {
				continue
			}
			seqs := uy.L(d.content["SoundSequences"])
			if seqs == nil {
				continue
			}
			for _, sv := range seqs {
				s := uy.M(sv)
				name := uy.S(s["name"])
				for _, cv := range uy.L(uy.Get(s, "sequence", "clips")) {
					c := uy.M(cv)
					clip := uy.S(c["clip"])
					if i := strings.LastIndexByte(clip, '/'); i >= 0 {
						clip = clip[i+1:]
					}
					vol := uy.F(c["volume"])
					if vol == 0 {
						vol = 1
					}
					extra.Sequences[name] = append(extra.Sequences[name], kmdata.SeqClip{
						Clip: clip, Beat: uy.F(c["beat"]), Volume: vol,
					})
				}
			}
		}
	}

	writeJSON("extra.json", extra)
	fmt.Printf("extra: %d refArrays, %d curves, %d obj templates, %d sequences\n",
		len(extra.RefArrays), len(extra.Curves), len(extra.ObjNums), len(extra.Sequences))
}

// ---------- anims / sounds ----------

// exportAnimDir 导出全部剪辑。同名 .anim 可能分属不同 Animator（如
// Girl/Bop 与 Player/Bop），因此每个剪辑都以"末级目录/文件名"为命名空间 key；
// 文件名全局唯一时再额外写裸名 key（向后兼容只有单 Animator 的游戏）。
func exportAnimDir(dir string, tables map[string]*spriteTable) {
	type clipFile struct {
		base, nsKey string
		clip        *kmdata.Anim
	}
	var clips []clipFile
	baseCount := map[string]int{}
	must(filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".anim") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		docs, err := uy.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		for i := range docs {
			if docs[i].ClassID == 74 {
				base := strings.TrimSuffix(filepath.Base(p), ".anim")
				ns := filepath.Base(filepath.Dir(p)) + "/" + base
				clips = append(clips, clipFile{base, ns, convertClip(docs[i].Content(), tables)})
				baseCount[base]++
				break
			}
		}
		return nil
	}))
	anims := map[string]*kmdata.Anim{}
	for _, c := range clips {
		anims[c.nsKey] = c.clip
		if baseCount[c.base] == 1 {
			anims[c.base] = c.clip
		} else {
			fmt.Printf("anim %q 有 %d 个同名文件，仅按命名空间 key 导出（如 %q）\n", c.base, baseCount[c.base], c.nsKey)
		}
	}
	writeJSON("anims.json", anims)
	fmt.Printf("anims: %d clip files\n", len(clips))
}

func copySounds(dir string) {
	entries, err := os.ReadDir(dir)
	must(err)
	n := 0
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".meta") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		must(err)
		must(os.WriteFile(filepath.Join(*outDir, "sounds", e.Name()), b, 0o644))
		n++
	}
	fmt.Printf("sounds: %d copied\n", n)
}
