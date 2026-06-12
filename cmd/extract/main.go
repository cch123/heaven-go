// extract 是迷你版 Unity 资产导出管线：从 Heaven Studio 工程中提取
// KarateMan 的图集切片、Joe 骨架（prefab 子树）、动画曲线与音效，
// 写成 kmdata 定义的 JSON + 原始 PNG/OGG，供 Go 运行时消费。
//
//	Unity 工程                          导出物 (assets/karateman/)
//	────────────────────────────       ──────────────────────────
//	karateman_main.png(.meta)    --->  atlas.png + sprites.json
//	karateman.prefab (Joe 子树)  --->  rig.json
//	anime/karateman/*.anim       --->  anims.json
//	Sounds/*.ogg|wav             --->  sounds/*
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

var (
	hsRoot = flag.String("hs", "/Users/xargin/Downloads/HeavenStudio-master", "Heaven Studio 工程根目录")
	outDir = flag.String("out", "", "输出目录（默认 assets/<game>）")
	game   = flag.String("game", "karateman", "要提取的 minigame（karateman / rhythmSomen）")

	animNames = []string{"Beat", "Jab", "Straight", "Prepare"}
	soundList = []string{
		"objectOut.ogg",      // 抛出
		"potHit.ogg",         // 命中（罐子）
		"punchKickHit1.ogg",  // 命中（偏差较大）
		"swingNoHit.wav",     // 空挥
		"karate_through.wav", // 漏拍飞过
	}
)

func gamePath(parts ...string) string {
	return filepath.Join(append([]string{*hsRoot, "Assets", "Bundled", "Games", "KarateMan"}, parts...)...)
}

func main() {
	flag.Parse()
	if *outDir == "" {
		*outDir = filepath.Join("assets", *game)
	}
	if *game == "common" {
		extractCommon()
		return
	}
	if *game != "karateman" {
		extractScene(*game)
		return
	}
	must(os.MkdirAll(filepath.Join(*outDir, "sounds"), 0o755))

	guidTable := scanSpriteMetas(gamePath("Sprites"))

	sheet := exportAtlas(guidTable)
	rig := exportRigAndStage(guidTable)
	exportAnims(guidTable)
	exportSounds()

	printBBox(rig, sheet)
	fmt.Println("done.")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func writeJSON(name string, v any) {
	b, err := json.MarshalIndent(v, "", " ")
	must(err)
	must(os.WriteFile(filepath.Join(*outDir, name), b, 0o644))
	fmt.Printf("wrote %s (%d bytes)\n", name, len(b))
}

// ---------- sprite metas ----------

// spriteTable: 一张贴图的 internalID → 切片名。
type spriteTable struct {
	pngPath    string
	byID       map[int64]string
	sheet      map[string]kmdata.SpriteInfo // 仅图集贴图填充
	ppu        float64
	texW, texH int
}

// scanSpriteMetas 扫描目录下所有 *.png.meta，建立 guid → 切片表。
func scanSpriteMetas(root string) map[string]*spriteTable {
	out := map[string]*spriteTable{}
	must(filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".png.meta") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m, err := uy.ParseSingle(data)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		guid := uy.S(m["guid"])
		ti := uy.M(m["TextureImporter"])
		if guid == "" || ti == nil {
			return nil
		}

		t := &spriteTable{
			pngPath: strings.TrimSuffix(p, ".meta"),
			byID:    map[int64]string{},
			sheet:   map[string]kmdata.SpriteInfo{},
			ppu:     uy.F(ti["spritePixelsToUnits"]),
		}
		if cfg, err := pngConfig(t.pngPath); err == nil {
			t.texW, t.texH = cfg.Width, cfg.Height
		}

		base := strings.TrimSuffix(filepath.Base(t.pngPath), ".png")
		if int(uy.I(ti["spriteMode"])) != 2 { // 单 sprite：固定 fileID 21300000
			t.byID[21300000] = base
		}
		for _, sv := range uy.L(uy.Get(ti, "spriteSheet", "sprites")) {
			sp := uy.M(sv)
			name := uy.S(sp["name"])
			id := uy.I(sp["internalID"])
			t.byID[id] = name

			rect := uy.M(sp["rect"])
			w := uy.F(rect["width"])
			h := uy.F(rect["height"])
			x := uy.F(uy.Get(rect, "x"))
			y := uy.F(uy.Get(rect, "y"))
			px, py := alignmentPivot(int(uy.I(sp["alignment"])), sp)
			t.sheet[name] = kmdata.SpriteInfo{
				X: int(math.Round(x)), Y: t.texH - int(math.Round(y+h)), // Unity 原点左下 → 图像左上
				W: int(math.Round(w)), H: int(math.Round(h)),
				PivotX: px, PivotY: py,
				Border: [4]float64{
					uy.F(uy.Get(sp, "border", "x")), uy.F(uy.Get(sp, "border", "y")),
					uy.F(uy.Get(sp, "border", "z")), uy.F(uy.Get(sp, "border", "w")),
				},
			}
		}
		out[guid] = t
		return nil
	}))
	fmt.Printf("scanned %d sprite metas\n", len(out))
	return out
}

// alignmentPivot 把 Unity SpriteAlignment 枚举换算为归一化枢轴。
func alignmentPivot(alignment int, sp map[string]any) (float64, float64) {
	switch alignment {
	case 0: // Center
		return 0.5, 0.5
	case 1: // TopLeft
		return 0, 1
	case 2: // TopCenter
		return 0.5, 1
	case 3: // TopRight
		return 1, 1
	case 4: // LeftCenter
		return 0, 0.5
	case 5: // RightCenter
		return 1, 0.5
	case 6: // BottomLeft
		return 0, 0
	case 7: // BottomCenter
		return 0.5, 0
	case 8: // BottomRight
		return 1, 0
	default: // 9 = Custom
		return uy.F(uy.Get(sp, "pivot", "x")), uy.F(uy.Get(sp, "pivot", "y"))
	}
}

func pngConfig(p string) (image.Config, error) {
	f, err := os.Open(p)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg, err
}

func resolveSprite(tables map[string]*spriteTable, guid string, fileID int64) string {
	if t, ok := tables[guid]; ok {
		return t.byID[fileID]
	}
	return ""
}

// ---------- atlas ----------

func exportAtlas(tables map[string]*spriteTable) *kmdata.Sheet {
	metaPath := gamePath("Sprites", "karateman_main.png.meta")
	raw, err := os.ReadFile(metaPath)
	must(err)
	m, err := uy.ParseSingle(raw)
	must(err)
	guid := uy.S(m["guid"])
	t := tables[guid]
	if t == nil {
		log.Fatalf("atlas meta guid %s not in table", guid)
	}

	png, err := os.ReadFile(t.pngPath)
	must(err)
	must(os.WriteFile(filepath.Join(*outDir, "atlas.png"), png, 0o644))

	sheet := &kmdata.Sheet{Atlas: "atlas.png", PPU: t.ppu, Sprites: t.sheet}
	writeJSON("sprites.json", sheet)
	fmt.Printf("atlas: %d sprites, ppu=%.2f\n", len(t.sheet), t.ppu)
	return sheet
}

// ---------- rig (prefab 子树) ----------

type prefabIndex struct {
	goName    map[int64]string         // GameObject fileID → 名字
	goActive  map[int64]bool           // GameObject fileID → m_IsActive
	tfByGO    map[int64]map[string]any // GameObject fileID → Transform 内容
	tfByID    map[int64]map[string]any // Transform fileID → 内容
	tfOwner   map[int64]int64          // Transform fileID → GameObject fileID
	rendByGO  map[int64]map[string]any // GameObject fileID → SpriteRenderer 内容
	groupByGO map[int64][]int          // GameObject fileID → SortingGroup [layer, order]

	mappedMats map[string]string // 调色板映射材质 guid → 文件主名（scene 模式填充）
}

// xAff 是提取器内部的 2D 仿射（与 kart.Aff 同布局）。
type xAff struct{ a, b, c, d, tx, ty float64 }

// apply 变换一个局部坐标点。
func (m xAff) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.tx, m.b*x + m.d*y + m.ty
}

// worldAff 沿 m_Father 链合成 Transform 的世界仿射。
func (idx *prefabIndex) worldAff(tfID int64) xAff {
	acc := xAff{1, 0, 0, 1, 0, 0}
	for tfID != 0 {
		tf := idx.tfByID[tfID]
		if tf == nil {
			break
		}
		rot := quatToZ(uy.F(uy.Get(tf, "m_LocalRotation", "z")), uy.F(uy.Get(tf, "m_LocalRotation", "w")))
		sx, sy := uy.F(uy.Get(tf, "m_LocalScale", "x")), uy.F(uy.Get(tf, "m_LocalScale", "y"))
		px, py := uy.F(uy.Get(tf, "m_LocalPosition", "x")), uy.F(uy.Get(tf, "m_LocalPosition", "y"))
		sin, cos := math.Sin(rot), math.Cos(rot)
		local := xAff{cos * sx, sin * sx, -sin * sy, cos * sy, px, py}
		// acc = local ∘ acc（自下而上，左乘父变换）
		acc = xAff{
			local.a*acc.a + local.c*acc.b, local.b*acc.a + local.d*acc.b,
			local.a*acc.c + local.c*acc.d, local.b*acc.c + local.d*acc.d,
			local.a*acc.tx + local.c*acc.ty + local.tx, local.b*acc.tx + local.d*acc.ty + local.ty,
		}
		tfID = uy.I(uy.Get(tf, "m_Father", "fileID"))
	}
	return acc
}

// worldPos 取 Transform 的世界坐标。
func (idx *prefabIndex) worldPos(tfID int64) (float64, float64) {
	m := idx.worldAff(tfID)
	return m.tx, m.ty
}

// worldZ 沿 m_Father 链累加深度（父链按无旋转/单位 z 缩放近似）。
func (idx *prefabIndex) worldZ(tfID int64) float64 {
	z := 0.0
	for tfID != 0 {
		tf := idx.tfByID[tfID]
		if tf == nil {
			break
		}
		z += uy.F(uy.Get(tf, "m_LocalPosition", "z"))
		tfID = uy.I(uy.Get(tf, "m_Father", "fileID"))
	}
	return z
}

// quatRotate 用四元数 (x,y,z,w) 旋转向量：v' = v + 2*q.xyz×(q.xyz×v + w*v)。
func quatRotate(qx, qy, qz, qw float64, v [3]float64) [3]float64 {
	cx := qy*v[2] - qz*v[1] + qw*v[0]
	cy := qz*v[0] - qx*v[2] + qw*v[1]
	cz := qx*v[1] - qy*v[0] + qw*v[2]
	return [3]float64{
		v[0] + 2*(qy*cz-qz*cy),
		v[1] + 2*(qz*cx-qx*cz),
		v[2] + 2*(qx*cy-qy*cx),
	}
}

// transformPoint3D 等价 Unity Transform.TransformPoint：完整三维
// 缩放→旋转→平移沿 m_Father 链合成（曲线 Point 的父链带三维旋转，
// 2D 仿射 + z 直加会让 z 混入 x/y 的分量丢失）。
func (idx *prefabIndex) transformPoint3D(tfID int64, local [3]float64) [3]float64 {
	v := local
	for tfID != 0 {
		tf := idx.tfByID[tfID]
		if tf == nil {
			break
		}
		v = [3]float64{
			v[0] * uy.F(uy.Get(tf, "m_LocalScale", "x")),
			v[1] * uy.F(uy.Get(tf, "m_LocalScale", "y")),
			v[2] * uy.F(uy.Get(tf, "m_LocalScale", "z")),
		}
		v = quatRotate(
			uy.F(uy.Get(tf, "m_LocalRotation", "x")), uy.F(uy.Get(tf, "m_LocalRotation", "y")),
			uy.F(uy.Get(tf, "m_LocalRotation", "z")), uy.F(uy.Get(tf, "m_LocalRotation", "w")),
			v)
		v = [3]float64{
			v[0] + uy.F(uy.Get(tf, "m_LocalPosition", "x")),
			v[1] + uy.F(uy.Get(tf, "m_LocalPosition", "y")),
			v[2] + uy.F(uy.Get(tf, "m_LocalPosition", "z")),
		}
		tfID = uy.I(uy.Get(tf, "m_Father", "fileID"))
	}
	return v
}

func exportRigAndStage(tables map[string]*spriteTable) *kmdata.Rig {
	ctrlMeta, err := os.ReadFile(gamePath("Sprites", "anime", "karateman", "KarateMan.controller.meta"))
	must(err)
	cm, err := uy.ParseSingle(ctrlMeta)
	must(err)
	ctrlGUID := uy.S(cm["guid"])

	raw, err := os.ReadFile(gamePath("karateman.prefab"))
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
	var rootGO int64
	goTF := map[int64]int64{}       // GameObject fileID → Transform fileID
	var potBehaviour map[string]any // KarateManPot 序列化字段（含轨迹参数）
	for i := range docs {
		d := &docs[i]
		c := d.Content()
		switch d.ClassID {
		case 1: // GameObject
			idx.goName[d.FileID] = uy.S(c["m_Name"])
		case 4: // Transform
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.tfByGO[gid] = c
			idx.tfByID[d.FileID] = c
			idx.tfOwner[d.FileID] = gid
			goTF[gid] = d.FileID
		case 212: // SpriteRenderer
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.rendByGO[gid] = c
		case 95: // Animator：用 controller guid 定位 Joe 根
			if uy.S(uy.Get(c, "m_Controller", "guid")) == ctrlGUID {
				rootGO = uy.I(uy.Get(c, "m_GameObject", "fileID"))
			}
		case 114: // MonoBehaviour：按字段特征识别 KarateManPot
			if potBehaviour == nil && c["HitPositionOffset"] != nil && c["ItemSlipRt"] != nil {
				potBehaviour = c
			}
		}
	}
	if rootGO == 0 {
		log.Fatal("rig root not found (no Animator references KarateMan.controller)")
	}
	fmt.Printf("rig root: %q (GameObject &%d)\n", idx.goName[rootGO], rootGO)

	rig := &kmdata.Rig{}
	var walk func(tf map[string]any, parent int, path string)
	walk = func(tf map[string]any, parent int, path string) {
		gid := uy.I(uy.Get(tf, "m_GameObject", "fileID"))
		n := kmdata.Node{
			Name:   idx.goName[gid],
			Path:   path,
			Parent: parent,
			Pos: [2]float64{
				uy.F(uy.Get(tf, "m_LocalPosition", "x")),
				uy.F(uy.Get(tf, "m_LocalPosition", "y")),
			},
			RotZ: quatToZ(
				uy.F(uy.Get(tf, "m_LocalRotation", "z")),
				uy.F(uy.Get(tf, "m_LocalRotation", "w")),
			),
			Scale: [2]float64{
				uy.F(uy.Get(tf, "m_LocalScale", "x")),
				uy.F(uy.Get(tf, "m_LocalScale", "y")),
			},
		}
		if r := idx.rendByGO[gid]; r != nil {
			n.Sprite = resolveSprite(tables,
				uy.S(uy.Get(r, "m_Sprite", "guid")), uy.I(uy.Get(r, "m_Sprite", "fileID")))
			n.Order = int(uy.I(r["m_SortingOrder"]))
			n.Hidden = uy.I(r["m_Enabled"]) == 0
			n.FlipX = uy.I(r["m_FlipX"]) != 0
		}
		self := len(rig.Nodes)
		rig.Nodes = append(rig.Nodes, n)

		for _, cv := range uy.L(tf["m_Children"]) {
			cid := uy.I(uy.Get(uy.M(cv), "fileID"))
			ct := idx.tfByID[cid]
			if ct == nil {
				continue // stripped / 外部引用
			}
			childName := idx.goName[idx.tfOwner[cid]]
			childPath := childName
			if path != "" {
				childPath = path + "/" + childName
			}
			walk(ct, self, childPath)
		}
	}
	walk(idx.tfByGO[rootGO], -1, "")
	writeJSON("rig.json", rig)
	fmt.Printf("rig: %d nodes\n", len(rig.Nodes))

	exportStage(idx, potBehaviour, goTF[rootGO])
	return rig
}

// exportStage 提取普通罐子（path=1）的轨迹参数，坐标换算为相对 Joe 根的单位空间。
func exportStage(idx *prefabIndex, pot map[string]any, joeTF int64) {
	if pot == nil {
		log.Fatal("KarateManPot behaviour not found in prefab (no doc with HitPositionOffset+ItemSlipRt)")
	}
	const path = 1 // 普通罐子

	hitRefs := uy.L(pot["HitPosition"])
	if len(hitRefs) <= path {
		log.Fatalf("HitPosition has %d entries, need > %d", len(hitRefs), path)
	}
	joeX, joeY := idx.worldPos(joeTF)
	hitTFID := uy.I(uy.Get(uy.M(hitRefs[path]), "fileID"))
	floorTFID := uy.I(uy.Get(uy.M(hitRefs[0]), "fileID"))
	hitX, hitY := idx.worldPos(hitTFID)
	_, floorY := idx.worldPos(floorTFID)

	offs := uy.L(pot["HitPositionOffset"])
	starts := uy.L(pot["StartPositionOffset"])
	slips := uy.L(pot["ItemSlipRt"])

	stage := &kmdata.Stage{
		HitPos: [2]float64{hitX - joeX, hitY - joeY},
		FloorY: floorY - joeY,
		StartOffset: [2]float64{
			uy.F(uy.Get(uy.M(starts[path]), "x")),
			uy.F(uy.Get(uy.M(starts[path]), "y")),
		},
		StartOffsetZ: uy.F(uy.Get(uy.M(starts[path]), "z")),
		HitOffset:    uy.F(offs[path]),
		Slip:         uy.F(slips[path]),
	}
	writeJSON("stage.json", stage)
	fmt.Printf("stage: hit=(%.2f, %.2f) floorY=%.2f startOff=(%.2f, %.2f) hitOff=%.2f slip=%.2f\n",
		stage.HitPos[0], stage.HitPos[1], stage.FloorY,
		stage.StartOffset[0], stage.StartOffset[1], stage.HitOffset, stage.Slip)
}

// quatToZ 取纯 Z 旋转四元数的角度（弧度）。
func quatToZ(z, w float64) float64 { return 2 * math.Atan2(z, w) }

// ---------- anims ----------

func exportAnims(tables map[string]*spriteTable) {
	anims := map[string]*kmdata.Anim{}
	for _, name := range animNames {
		p := gamePath("Sprites", "anime", "karateman", name+".anim")
		raw, err := os.ReadFile(p)
		if err != nil {
			log.Printf("skip %s: %v", name, err)
			continue
		}
		docs, err := uy.Parse(raw)
		must(err)
		var clip map[string]any
		for i := range docs {
			if docs[i].ClassID == 74 {
				clip = docs[i].Content()
				break
			}
		}
		if clip == nil {
			log.Fatalf("%s: no AnimationClip doc", name)
		}
		anims[name] = convertClip(clip, tables)
		fmt.Printf("anim %-9s dur=%.3fs loop=%v\n", name, anims[name].Duration, anims[name].Loop)
	}
	writeJSON("anims.json", anims)
}

func convertClip(clip map[string]any, tables map[string]*spriteTable) *kmdata.Anim {
	a := &kmdata.Anim{
		Duration: uy.F(uy.Get(clip, "m_AnimationClipSettings", "m_StopTime")),
		Loop:     uy.I(uy.Get(clip, "m_AnimationClipSettings", "m_LoopTime")) != 0,
		Pos:      map[string]kmdata.XYCurve{},
		Euler:    map[string][]kmdata.Key{},
		Scale:    map[string]kmdata.XYCurve{},
		Sprites:  map[string][]kmdata.SwapKey{},
		Floats:   map[string]map[string][]kmdata.Key{},
	}

	vecCurves := func(field string, dst map[string]kmdata.XYCurve) {
		for _, cv := range uy.L(clip[field]) {
			c := uy.M(cv)
			path := uy.S(c["path"])
			var xc, yc []kmdata.Key
			for _, kv := range uy.L(uy.Get(c, "curve", "m_Curve")) {
				k := uy.M(kv)
				t := uy.F(k["time"])
				xc = append(xc, key(t, uy.Get(k, "value", "x"), uy.Get(k, "inSlope", "x"), uy.Get(k, "outSlope", "x")))
				yc = append(yc, key(t, uy.Get(k, "value", "y"), uy.Get(k, "inSlope", "y"), uy.Get(k, "outSlope", "y")))
			}
			dst[path] = kmdata.XYCurve{X: xc, Y: yc}
		}
	}
	vecCurves("m_PositionCurves", a.Pos)
	vecCurves("m_ScaleCurves", a.Scale)

	for _, cv := range uy.L(clip["m_EulerCurves"]) {
		c := uy.M(cv)
		path := uy.S(c["path"])
		var keys []kmdata.Key
		for _, kv := range uy.L(uy.Get(c, "curve", "m_Curve")) {
			k := uy.M(kv)
			keys = append(keys, key(uy.F(k["time"]), uy.Get(k, "value", "z"), uy.Get(k, "inSlope", "z"), uy.Get(k, "outSlope", "z")))
		}
		a.Euler[path] = keys
	}

	for _, cv := range uy.L(clip["m_FloatCurves"]) {
		c := uy.M(cv)
		path, attr := uy.S(c["path"]), uy.S(c["attribute"])
		var keys []kmdata.Key
		for _, kv := range uy.L(uy.Get(c, "curve", "m_Curve")) {
			k := uy.M(kv)
			keys = append(keys, key(uy.F(k["time"]), k["value"], k["inSlope"], k["outSlope"]))
		}
		if a.Floats[path] == nil {
			a.Floats[path] = map[string][]kmdata.Key{}
		}
		a.Floats[path][attr] = keys
	}

	for _, cv := range uy.L(clip["m_PPtrCurves"]) {
		c := uy.M(cv)
		if uy.S(c["attribute"]) != "m_Sprite" {
			continue
		}
		path := uy.S(c["path"])
		var keys []kmdata.SwapKey
		for _, kv := range uy.L(c["curve"]) {
			k := uy.M(kv)
			name := resolveSprite(tables,
				uy.S(uy.Get(k, "value", "guid")), uy.I(uy.Get(k, "value", "fileID")))
			keys = append(keys, kmdata.SwapKey{T: uy.F(k["time"]), Name: name})
		}
		a.Sprites[path] = keys
	}
	return a
}

// key 构造关键帧，±Inf 斜率编码为 ±1e30（kmdata.StepSlope 哨兵之上）。
func key(t float64, v, in, out any) kmdata.Key {
	return kmdata.Key{T: t, V: uy.F(v), I: capInf(uy.F(in)), O: capInf(uy.F(out))}
}

func capInf(f float64) float64 {
	if math.IsInf(f, 1) {
		return 1e30
	}
	if math.IsInf(f, -1) {
		return -1e30
	}
	return f
}

// ---------- 公共音效（assets/common） ----------

// extractCommon 导出引擎级公共音效：countIn 计数音（count-ins/ 根目录全部，
// gba/dsmale/dsfemale 变体目录暂不需要）与通用判定音。engine 启动时加载
// assets/common，countIn/* 事件由 engine 直接播放。
func extractCommon() {
	outSounds := filepath.Join(*outDir, "sounds")
	must(os.MkdirAll(outSounds, 0o755))
	sfxRoot := filepath.Join(*hsRoot, "Assets", "Resources", "Sfx")
	n := 0
	copyOne := func(rel string) {
		b, err := os.ReadFile(filepath.Join(sfxRoot, rel))
		if err != nil {
			log.Fatalf("公共音效 %s: %v", rel, err)
		}
		must(os.WriteFile(filepath.Join(outSounds, filepath.Base(rel)), b, 0o644))
		n++
	}
	entries, err := os.ReadDir(filepath.Join(sfxRoot, "count-ins"))
	must(err)
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".meta") {
			continue
		}
		copyOne(filepath.Join("count-ins", e.Name()))
	}
	for _, name := range []string{"miss.wav", "nearMiss.ogg", "skillStar.ogg", "metronome.wav"} {
		copyOne(name)
	}
	fmt.Printf("common sounds: %d copied -> %s\n", n, outSounds)

	// vfx/filter 的 AmplifyColor LUT（Resources/Filters/*.png，55 张）
	lutDir := filepath.Join(*outDir, "filters")
	must(os.MkdirAll(lutDir, 0o755))
	luts, err := os.ReadDir(filepath.Join(*hsRoot, "Assets", "Resources", "Filters"))
	must(err)
	ln := 0
	for _, e := range luts {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".meta") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*hsRoot, "Assets", "Resources", "Filters", e.Name()))
		must(err)
		must(os.WriteFile(filepath.Join(lutDir, e.Name()), b, 0o644))
		ln++
	}
	fmt.Printf("filter LUTs: %d copied\n", ln)

	// vfx/display textbox 的框体贴图 + 字体（KurokaneStd，近似原版样式）
	tb, err := os.ReadFile(filepath.Join(*hsRoot, "Assets", "Resources", "Sprites", "UI", "Common", "Textbox", "textboxSDF.png"))
	must(err)
	must(os.WriteFile(filepath.Join(*outDir, "textbox.png"), tb, 0o644))
	fnt, err := os.ReadFile(filepath.Join(*hsRoot, "Assets", "Resources", "Fonts", "kurokane", "FOT-KurokaneStd_Megamix_Modified-EB.otf"))
	must(err)
	must(os.WriteFile(filepath.Join(*outDir, "textbox_font.otf"), fnt, 0o644))
	fmt.Println("textbox assets copied")
}

// ---------- sounds ----------

func exportSounds() {
	for _, name := range soundList {
		b, err := os.ReadFile(gamePath("Sounds", name))
		if err != nil {
			log.Printf("skip sound %s: %v", name, err)
			continue
		}
		must(os.WriteFile(filepath.Join(*outDir, "sounds", name), b, 0o644))
	}
	fmt.Printf("sounds: %d copied\n", len(soundList))
}

// ---------- 标定输出 ----------

// printBBox 计算默认姿态下骨架的世界包围盒（unit），供运行时镜头标定参考。
func printBBox(rig *kmdata.Rig, sheet *kmdata.Sheet) {
	type aff struct{ a, b, c, d, tx, ty float64 }
	id := aff{1, 0, 0, 1, 0, 0}
	mul := func(m, n aff) aff {
		return aff{
			m.a*n.a + m.c*n.b, m.b*n.a + m.d*n.b,
			m.a*n.c + m.c*n.d, m.b*n.c + m.d*n.d,
			m.a*n.tx + m.c*n.ty + m.tx, m.b*n.tx + m.d*n.ty + m.ty,
		}
	}
	world := make([]aff, len(rig.Nodes))
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, n := range rig.Nodes {
		sin, cos := math.Sin(n.RotZ), math.Cos(n.RotZ)
		local := aff{cos * n.Scale[0], sin * n.Scale[0], -sin * n.Scale[1], cos * n.Scale[1], n.Pos[0], n.Pos[1]}
		if n.Parent < 0 {
			world[i] = local
		} else {
			world[i] = mul(world[n.Parent], local)
		}
		sp, ok := sheet.Sprites[n.Sprite]
		if !ok || n.Hidden {
			continue
		}
		w, h := float64(sp.W)/sheet.PPU, float64(sp.H)/sheet.PPU
		for _, corner := range [][2]float64{
			{-sp.PivotX * w, -sp.PivotY * h}, {(1 - sp.PivotX) * w, -sp.PivotY * h},
			{-sp.PivotX * w, (1 - sp.PivotY) * h}, {(1 - sp.PivotX) * w, (1 - sp.PivotY) * h},
		} {
			m := world[i]
			x := m.a*corner[0] + m.c*corner[1] + m.tx
			y := m.b*corner[0] + m.d*corner[1] + m.ty
			minX, maxX = math.Min(minX, x), math.Max(maxX, x)
			minY, maxY = math.Min(minY, y), math.Max(maxY, y)
		}
	}
	_ = id
	fmt.Printf("rig bbox (units): x[%.2f, %.2f] y[%.2f, %.2f] size %.2fx%.2f\n",
		minX, maxX, minY, maxY, maxX-minX, maxY-minY)
}
