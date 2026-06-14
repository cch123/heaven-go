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

	refArrayFields  []string // 引用数组字段（如对象模板表）
	strArrayFields  []string // 字符串数组字段（如动画名表）
	curveFields     []string // BezierCurve3D 引用字段
	objMarkers      []string // 识别"对象模板组件"的字段集合（如 MobTrickObj）
	objRefFields    []string // 模板组件上的单引用字段（→ 节点 path，如 Meat.startPosition）
	objSpriteFields []string // 模板组件上的 sprite 引用数组字段（→ 切片名，如 Meat.meats）
	wantSequences   bool     // 提取 SoundSequences 组件
	commonSounds    []string // 需要的公共音效（Assets/Resources/Sfx/<name>）
	wantControllers bool     // 提取 AnimatorController 状态机（controllers.json + animators.json）
	wantTexts       bool     // 提取 TMP 世界文本（texts.json + fonts/）

	components []componentSpec // 通用组件 dump（Extra.Components）
}

// componentSpec 按字段特征（可选限定 GameObject path）识别一个 MonoBehaviour，
// 全字段通用 dump 到 Extra.Components[name]。
type componentSpec struct {
	name    string
	markers []string // 必须同时存在的字段
	atPath  string   // 非空时限定组件所在 GameObject 的 path（同字段集脚本如 TCTotem/TCDragon）
	multi   bool     // 匹配多个组件：导出为 name0、name1…（按 path 排序）
}

var sceneSpecs = map[string]sceneSpec{
	"bouncyRoad": {
		dir:    "BouncyRoad",
		prefab: "bouncyRoad.prefab",
		roleFields: []string{
			"baseBall", "baseBounceCurve", "CurveHolder", "ThingsTrans",
			"PosCurve", "BGGradient", "BGHigh", "BGLow",
		},
		curveFields:     []string{"baseBounceCurve", "PosCurve"},
		wantControllers: true,
	},
	"catchyTune": {
		dir:    "CatchyTune",
		prefab: "catchyTune.prefab",
		roleFields: []string{
			"plalinAnim", "alalinAnim", "orangeBase", "pineappleBase",
			"fruitHolder", "heartMessage", "bg2",
		},
		wantControllers: true,
	},
	"coinToss": {
		dir:    "CoinToss",
		prefab: "coinToss.prefab",
		roleFields: []string{
			"fg", "bg", "imageBG", "handAnimator", "manHand",
			"handHolder", "manHolder", "imageAnim",
		},
		wantControllers: true,
		commonSounds:    []string{"applause.ogg", "audienceSad.ogg"},
	},
	"wizardsWaltz": {
		dir:             "WizardsWaltz",
		prefab:          "wizardsWaltz.prefab",
		roleFields:      []string{"wizard", "girl", "plantHolder", "plantBase"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"wizard", "girl", "plantHolder", "plantBase"}},
			{name: "wizard", markers: []string{"animator", "shadow"}, atPath: "Wizard"},
			{name: "girl", markers: []string{"animator", "flowers"}, atPath: "Girl"},
			{name: "plant", markers: []string{"animator", "spriteRenderer", "createBeat"}, atPath: "Prefabs/Plant"},
		},
	},
	"basketballGirls": {
		dir:             "BasketballGirls",
		prefab:          "basketballGirls.prefab",
		roleFields:      []string{"baseBall", "girlLeftAnim", "girlRightAnim", "goalAnim", "BGPlane"},
		refArrayFields:  []string{"CameraPosition"},
		wantControllers: true,
	},
	"drummingPractice": {
		dir:    "DrummingPractice",
		prefab: "drummingPractice.prefab",
		roleFields: []string{
			"background", "backgroundGradient", "player", "leftDrummer",
			"rightDrummer", "hitPrefab", "NPCDrummers",
		},
		wantControllers: true,
		commonSounds:    []string{"applause.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"background", "backgroundGradient", "streaks", "player", "leftDrummer", "rightDrummer"}},
			{name: "drummer", markers: []string{"animator", "miiFaces", "face"}, multi: true},
		},
	},
	"clappyTrio": {
		dir:             "ClappyTrio",
		prefab:          "clappyTrio.prefab",
		roleFields:      []string{"customText", "signAnim", "textTrioTiming", "textCustom"},
		wantControllers: true,
		wantTexts:       true,
		components: []componentSpec{
			{name: "game", markers: []string{"Lion", "faces", "signAnim", "textTrioTiming", "textCustom"}},
		},
	},
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
	"totemClimb": {
		dir:    "TotemClimb",
		prefab: "totemClimb.prefab",
		roleFields: []string{
			"_cameraTransform", "_scrollTransform", "_jumper", "_totemManager",
			"_birdManager", "_groundHolder", "_fakeTotemHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"_scrollSpeedX", "_scrollTransform"}},
			{name: "jumper", markers: []string{"_jumpHeight", "_initialPoint"}},
			{name: "totemManager", markers: []string{"_xDistance", "_totemTransform"}},
			{name: "birdManager", markers: []string{"_birdRef", "_speedX"}},
			{name: "groundManager", markers: []string{"_groundFirst"}},
			{name: "pillarManager", markers: []string{"_pillarFirst"}},
			{name: "backgroundManager", markers: []string{"_objectsParent"}},
			{name: "totem", markers: []string{"_anim", "_jumperPoint"}, atPath: "Game/Scrollable/Totems/Totem"},
			{name: "dragon", markers: []string{"_anim", "_jumperPoint"}, atPath: "Game/Scrollable/Totems/Dragon"},
			{name: "frog", markers: []string{"_animLeft", "_jumperPointLeft"}},
		},
	},
	"seeSaw": {
		dir:    "SeeSaw",
		prefab: "seeSaw.prefab",
		roleFields: []string{
			"seeSawAnim", "see", "saw", "leftWhiteOrbs", "rightBlackOrbs",
			"gradient", "bgLow", "bgHigh",
		},
		refArrayFields:  []string{"recolors"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"jumpPaths", "seeSawAnim"}},
			{name: "see", markers: []string{"landOutTrans", "deathParticle"}, atPath: "Game/Guys/SeeHolder"},
			{name: "saw", markers: []string{"landOutTrans", "deathParticle"}, atPath: "Game/Guys/SawHolder"},
		},
	},
	"blueBear": {
		dir:    "BlueBear",
		prefab: "blueBear.prefab",
		roleFields: []string{
			"headAndBodyAnim", "bagsAnim", "donutBagAnim", "cakeBagAnim", "windAnim",
			"leftCrumb", "rightCrumb", "_storyAnim", "donutBase", "cakeBase",
			"crumbsBase", "foodHolder", "crumbsHolder", "individualBagHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"_treatCurves", "donutGradient"}},
		},
	},
	"cheerReaders": {
		dir:    "CheerReaders",
		prefab: "cheerReaders.prefab",
		roleFields: []string{
			"playerMask", "missPoster", "topPoster", "middlePoster", "bottomPoster",
			"player", "whiteYayParticle", "blackYayParticle",
		},
		refArrayFields: []string{
			"firstRow", "secondRow", "thirdRow", "topMasks", "middleMasks", "bottomMasks",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"posters", "topMasks"}},
			{name: "girl", markers: []string{"faceAnim", "blushLeft"}, multi: true},
		},
	},
	"tapTrial": {
		dir:    "TapTrial",
		prefab: "tapTrial.prefab",
		roleFields: []string{
			"player", "monkeyL", "monkeyR", "giraffe",
			"rootPlayer", "rootMonkeyL", "rootMonkeyR", "flash",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"tambourine": {
		dir:    "Tambourine",
		prefab: "tambourine.prefab",
		roleFields: []string{
			"handsAnimator", "bg", "monkeyAnimator", "flowerParticles",
			"happyFace", "sadFace", "sweatAnimator", "frogAnimator",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"sneakySpirits": {
		dir:    "SneakySpirits",
		prefab: "sneakySpirits.prefab",
		roleFields: []string{
			"bowAnim", "bowHolderAnim", "doorAnim", "arrowMissPrefab", "ghostMissPrefab",
			"deathGhostPrefab", "normalRain", "slowRain", "normalTree", "slowTree",
		},
		refArrayFields:  []string{"ghostPositions"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"airRally": {
		dir:    "AirRally",
		prefab: "airRally.prefab",
		roleFields: []string{
			"Baxter", "Forthington", "Shuttlecock", "objHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"spaceDance": {
		dir:    "SpaceDance",
		prefab: "spaceDance.prefab",
		roleFields: []string{
			"bg", "shootingStarAnim", "DancerP", "Dancer1", "Dancer2", "Dancer3",
			"Gramps", "Hit", "Player",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"kitties": {
		dir:    "Kitties",
		prefab: "kitties.prefab",
		roleFields: []string{
			"player", "Fish", "background",
		},
		refArrayFields:  []string{"kitties", "Cats"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "playerScript", markers: []string{"Player", "fish"}},
		},
	},
	"lockstep": {
		dir:    "Lockstep",
		prefab: "lockstep.prefab",
		roleFields: []string{
			"stepswitcherPlayer", "stepswitcherLeft", "stepswitcherRight", "bach",
			"masterStepperAnim", "masterStepperSprite", "background",
		},
		refArrayFields:  []string{"slaveSteppers"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"marchingOrders": {
		dir:    "MarchingOrders",
		prefab: "marchingOrders.prefab",
		roleFields: []string{
			"Sarge", "Steam", "CadetPlayer", "CadetHeadPlayer",
		},
		refArrayFields:  []string{"Cadets", "CadetHeads", "BackgroundRecolorable", "RecolorMats", "ConveyorGo"},
		wantControllers: true,
		wantSequences:   true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"munchyMonk": {
		dir:    "MunchyMonk",
		prefab: "munchyMonk.prefab",
		roleFields: []string{
			"Baby", "BrowHolder", "StacheHolder", "DumplingObj", "CloudMonkey",
			"OneGiverAnim", "TwoGiverAnim", "ThreeGiverAnim", "BrowAnim",
			"StacheAnim", "MonkHolderAnim", "MonkAnim", "MonkArmsAnim",
		},
		wantControllers: true,
		wantSequences:   true,
		components: []componentSpec{
			{name: "game", markers: []string{"dumplingSprites", "MonkAnim"}},
			{name: "scroll", markers: []string{"XSpeed", "PositiveBounds"}, multi: true},
		},
	},
	"meatGrinder": {
		dir:    "MeatGrinder",
		prefab: "meatGrinder.prefab",
		roleFields: []string{
			"GrinderText", "MeatBase", "MeatSplash",
			"BossAnim", "TackAnim", "CartGuyParentAnim", "CartGuyAnim",
		},
		refArrayFields:  []string{"Gears"},
		objMarkers:      []string{"meatFlyHeight", "meatFlyHeightAlt"}, // Meat.cs
		objRefFields:    []string{"startPosition", "startPositionAlt", "hitPosition", "missPosition"},
		objSpriteFields: []string{"meats"},
		wantControllers: true,
		wantTexts:       true,
	},
}

func bundlePath(dir string, parts ...string) string {
	return filepath.Join(append([]string{*hsRoot, "Assets", "Bundled", "Games", dir}, parts...)...)
}

// mappingShaderGUIDs 是调色板映射 shader（CellAnime_MappedInvert /
// CellAnime_Mapped）的 guid：贴图 RGB 通道为掩码权重，
// out = ColorAlpha·r + ColorBravo·g + ColorDelta·b。
var mappingShaderGUIDs = []string{
	"d6702951943fe3f48b9e437dd725e76f", // CellAnime_MappedInvert
	"ff54fed5718ccc543808dec1f266d1c8", // CellAnime_Mapped
}

// scanMappedMats 扫描游戏目录下使用映射 shader 的材质，guid → 文件主名。
func scanMappedMats(root string) map[string]string {
	out := map[string]string{}
	for guid, p := range scanGUIDs(root, ".mat") {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, sg := range mappingShaderGUIDs {
			if strings.Contains(string(raw), sg) {
				out[guid] = strings.TrimSuffix(filepath.Base(p), ".mat")
				break
			}
		}
	}
	if len(out) > 0 {
		fmt.Printf("mapped materials: %d\n", len(out))
	}
	return out
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
	idx.mappedMats = scanMappedMats(bundlePath(spec.dir))
	paths, nodeIdx := exportScene(idx, tables)
	exportRoles(spec, docs, idx, paths)
	exportExtra(spec, docs, idx, paths, nodeIdx, tables)
	exportAnimDir(bundlePath(spec.dir, "Sprites"), tables)
	if spec.wantControllers {
		exportControllers(spec, docs, idx, paths)
	}
	if spec.wantTexts {
		exportTexts(docs, paths)
	}
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
		base := strings.TrimSuffix(filepath.Base(t.pngPath), ".png")
		for sname, sp := range t.sheet {
			sp.Atlas = atlasIdx
			sp.PPU = t.ppu
			key := sname
			if _, dup := sheet.Sprites[key]; dup {
				// 跨贴图同名切片（cheerReaders 14 张海报均为
				// TopPart/MiddlePart/BottomPart/Miss）：按文件名命名空间。
				key = base + "/" + sname
				for id, n := range t.byID {
					if n == sname {
						t.byID[id] = key
					}
				}
			}
			sheet.Sprites[key] = sp
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
	// 展开嵌套 prefab（子 prefab 在游戏目录与共享 Prefabs 下扫描）
	prefabGUIDs := scanGUIDs(filepath.Dir(prefabPath), ".prefab")
	docs, err := expandPrefab(prefabPath, prefabGUIDs)
	must(err)
	fmt.Printf("prefab: %d documents（含嵌套展开）\n", len(docs))

	idx := &prefabIndex{
		goName: map[int64]string{}, tfByGO: map[int64]map[string]any{},
		tfByID: map[int64]map[string]any{}, tfOwner: map[int64]int64{},
		rendByGO: map[int64]map[string]any{}, goActive: map[int64]bool{},
		groupByGO: map[int64][]int{},
		maskByGO:  map[int64]map[string]any{},
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
		case 4, 224: // Transform / RectTransform（TMP 文本节点）
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
		case 331: // SpriteMask
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.maskByGO[gid] = c
		}
	}
	return idx, dt
}

// exportScene 导出整棵节点树，返回 GameObject fileID → 节点 path（供 roles 解析）
// 与 GameObject fileID → 节点下标（path 重名时按下标寻址）。
func exportScene(idx *prefabIndex, tables map[string]*spriteTable) (map[int64]string, map[int64]int) {
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
	nodeIdx := map[int64]int{}
	var walk func(tf map[string]any, parent int, path string)
	walk = func(tf map[string]any, parent int, path string) {
		gid := uy.I(uy.Get(tf, "m_GameObject", "fileID"))
		paths[gid] = path
		pos := [2]float64{
			uy.F(uy.Get(tf, "m_LocalPosition", "x")),
			uy.F(uy.Get(tf, "m_LocalPosition", "y")),
		}
		// RectTransform：本地位置由 m_AnchoredPosition 驱动（点锚 + 非 Rect 父
		// 节点时即等于 localPosition.xy；m_LocalPosition 是序列化残留）
		if ap, ok := tf["m_AnchoredPosition"]; ok {
			pos = [2]float64{uy.F(uy.Get(uy.M(ap), "x")), uy.F(uy.Get(uy.M(ap), "y"))}
		}
		n := kmdata.Node{
			Name:   idx.goName[gid],
			Path:   path,
			Parent: parent,
			Pos:    pos,
			PosZ:   uy.F(uy.Get(tf, "m_LocalPosition", "z")),
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
			for _, mv := range uy.L(r["m_Materials"]) {
				if name, ok := idx.mappedMats[uy.S(uy.Get(uy.M(mv), "guid"))]; ok {
					n.Mapped = true
					n.Mat = name
				}
			}
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
			n.MaskIn = int(uy.I(r["m_MaskInteraction"]))
		}
		if mk := idx.maskByGO[gid]; mk != nil {
			n.Mask = true
			n.Sprite = resolveSprite(tables,
				uy.S(uy.Get(mk, "m_Sprite", "guid")), uy.I(uy.Get(mk, "m_Sprite", "fileID")))
			n.Hidden = uy.I(mk["m_Enabled"]) == 0
		}
		self := len(scene.Nodes)
		nodeIdx[gid] = self
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
	return paths, nodeIdx
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

func exportExtra(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string, nodeIdx map[int64]int, tables map[string]*spriteTable) {
	if len(spec.refArrayFields)+len(spec.strArrayFields)+len(spec.curveFields)+len(spec.objMarkers)+len(spec.components) == 0 && !spec.wantSequences {
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
		RefArrays:   map[string][]string{},
		Strings:     map[string][]string{},
		Curves:      map[string]kmdata.Curve{},
		ObjNums:     map[string]map[string]float64{},
		ObjStrs:     map[string]map[string]string{},
		Sequences:   map[string][]kmdata.SeqClip{},
		RefArrayIdx: map[string][]int{},
		ObjRefs:     map[string]map[string]string{},
		ObjSprites:  map[string]map[string][]string{},
	}

	// goIdxOf 把组件或 GameObject 引用解析为场景节点下标。
	goIdxOf := func(fid int64) (int, bool) {
		ref := dt.byID[fid]
		if ref == nil {
			return -1, false
		}
		gid := fid
		if ref.classID != 1 {
			gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
		}
		i, ok := nodeIdx[gid]
		return i, ok
	}

	for _, f := range spec.refArrayFields {
		for _, rv := range uy.L(script[f]) {
			fid := uy.I(uy.Get(uy.M(rv), "fileID"))
			if mname, isMat := idx.mappedMats[uy.S(uy.Get(uy.M(rv), "guid"))]; isMat {
				extra.RefArrays[f] = append(extra.RefArrays[f], mname)
				extra.RefArrayIdx[f] = append(extra.RefArrayIdx[f], -1)
				continue
			}
			p, ok := goPathOf(dt, paths, fid)
			if !ok {
				log.Printf("warn: refArray %s -> &%d not in scene", f, fid)
			}
			extra.RefArrays[f] = append(extra.RefArrays[f], p)
			i, ok := goIdxOf(fid)
			if !ok {
				i = -1
			}
			extra.RefArrayIdx[f] = append(extra.RefArrayIdx[f], i)
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
		if curveDoc.classID == 1 {
			// Some Heaven Studio prefabs serialize a BezierCurve3D field as the
			// owning GameObject. Resolve that to the MonoBehaviour carrying
			// keyPoints so runtime ports still use Unity-authored path data.
			for _, d := range dt.byID {
				if d.classID != 114 {
					continue
				}
				if uy.I(uy.Get(d.content, "m_GameObject", "fileID")) != fid {
					continue
				}
				if uy.L(d.content["keyPoints"]) != nil || uy.L(d.content["KeyPoints"]) != nil {
					curveDoc = d
					break
				}
			}
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
			// 单引用字段（Transform/GameObject → 节点 path）
			for _, f := range spec.objRefFields {
				rv, ok := d.content[f]
				if !ok {
					continue
				}
				fid := uy.I(uy.Get(uy.M(rv), "fileID"))
				rp, ok := goPathOf(dt, paths, fid)
				if !ok {
					log.Printf("warn: objRef %s.%s -> &%d not in scene", p, f, fid)
					continue
				}
				if extra.ObjRefs[p] == nil {
					extra.ObjRefs[p] = map[string]string{}
				}
				extra.ObjRefs[p][f] = rp
			}
			// sprite 引用数组字段（→ 图集切片名）
			for _, f := range spec.objSpriteFields {
				rv, ok := d.content[f]
				if !ok {
					continue
				}
				var names []string
				for _, sv := range uy.L(rv) {
					s := uy.M(sv)
					name := resolveSprite(tables, uy.S(s["guid"]), uy.I(s["fileID"]))
					if name == "" {
						log.Printf("warn: objSprite %s.%s 切片解析失败 guid=%s fileID=%d",
							p, f, uy.S(s["guid"]), uy.I(s["fileID"]))
					}
					names = append(names, name)
				}
				if extra.ObjSprites[p] == nil {
					extra.ObjSprites[p] = map[string][]string{}
				}
				extra.ObjSprites[p][f] = names
			}
		}
	}

	// 通用组件 dump
	if len(spec.components) > 0 {
		extra.Components = map[string]kmdata.Component{}
		for _, cs := range spec.components {
			type hit struct {
				p       string
				content map[string]any
			}
			var hits []hit
			for _, d := range dt.byID {
				if d.classID != 114 {
					continue
				}
				ok := true
				for _, mk := range cs.markers {
					if _, has := d.content[mk]; !has {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
				p, inScene := paths[gid]
				if !inScene {
					continue
				}
				if cs.atPath != "" && p != cs.atPath {
					continue
				}
				hits = append(hits, hit{p, d.content})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].p < hits[j].p })
			switch {
			case len(hits) == 0:
				log.Fatalf("组件 %s（markers %v）未在 prefab 中找到", cs.name, cs.markers)
			case cs.multi:
				for i, h := range hits {
					extra.Components[fmt.Sprintf("%s%d", cs.name, i)] = dumpComponent(dt, paths, tables, idx.mappedMats, h.p, h.content)
				}
			default:
				if len(hits) > 1 {
					log.Printf("warn: 组件 %s 匹配 %d 个，保留 path 最小者 %q（用 atPath/multi 限定）", cs.name, len(hits), hits[0].p)
				}
				extra.Components[cs.name] = dumpComponent(dt, paths, tables, idx.mappedMats, hits[0].p, hits[0].content)
			}
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

// dumpComponent 通用 dump 一个 MonoBehaviour 的全部序列化字段：
// 数值/字符串直存；{fileID} 引用 → 节点 path；{fileID, guid} → 图集切片名
// （解析失败回退节点 path）；x/y/z 向量按分量展开；结构体数组逐项解析。
func dumpComponent(dt *docTable, paths map[int64]string, tables map[string]*spriteTable,
	mats map[string]string, p string, content map[string]any) kmdata.Component {
	c := kmdata.Component{
		Path: p,
		Nums: map[string]float64{}, Strs: map[string]string{},
		Refs: map[string]string{}, Sprites: map[string]string{},
		RefArrays: map[string][]string{}, SpriteArrays: map[string][]string{},
		Lists: map[string][]kmdata.ComponentItem{},
	}
	resolveRef := func(field string, m map[string]any) (string, bool) {
		fid := uy.I(m["fileID"])
		if fid == 0 {
			return "", false
		}
		if g := uy.S(m["guid"]); g != "" {
			if name := resolveSprite(tables, g, fid); name != "" {
				return name, true // sprite
			}
			if name, ok := mats[g]; ok {
				return name, false // 映射材质 → 文件主名
			}
		}
		rp, ok := goPathOf(dt, paths, fid)
		if !ok {
			log.Printf("warn: 组件字段 %s.%s 引用 &%d 无法解析", p, field, fid)
			return "", false
		}
		return rp, false
	}
	for k, v := range content {
		if strings.HasPrefix(k, "m_") || k == "SoundSequences" {
			continue
		}
		switch tv := v.(type) {
		case int, int64, uint64, float64:
			c.Nums[k] = uy.F(v)
		case string:
			c.Strs[k] = tv
		case map[string]any:
			if _, hasID := tv["fileID"]; hasID {
				val, isSprite := resolveRef(k, tv)
				if val == "" {
					continue
				}
				if isSprite {
					c.Sprites[k] = val
				} else {
					c.Refs[k] = val
				}
			} else if _, hasX := tv["x"]; hasX {
				for _, axis := range []string{"x", "y", "z", "w"} {
					if av, ok := tv[axis]; ok {
						c.Nums[k+"."+axis] = uy.F(av)
					}
				}
			} else if _, hasKey0 := tv["key0"]; hasKey0 {
				// Unity Gradient：key0..7 颜色 + ctime0..7（0..65535 归一化时刻）
				nkeys := int(uy.F(tv["m_NumColorKeys"]))
				for ki := 0; ki < nkeys && ki < 8; ki++ {
					kv := uy.M(tv[fmt.Sprintf("key%d", ki)])
					item := kmdata.ComponentItem{Nums: map[string]float64{
						"r": uy.F(kv["r"]), "g": uy.F(kv["g"]), "b": uy.F(kv["b"]), "a": uy.F(kv["a"]),
						"t": uy.F(tv[fmt.Sprintf("ctime%d", ki)]) / 65535,
					}}
					c.Lists[k] = append(c.Lists[k], item)
				}
			}
		case []any:
			for _, iv := range tv {
				im := uy.M(iv)
				if im == nil {
					continue
				}
				if _, hasID := im["fileID"]; hasID && len(im) <= 3 { // 纯引用数组（fileID[+guid+type]）
					val, isSprite := resolveRef(k, im)
					if isSprite {
						c.SpriteArrays[k] = append(c.SpriteArrays[k], val)
					} else {
						c.RefArrays[k] = append(c.RefArrays[k], val)
					}
					continue
				}
				c.Lists[k] = append(c.Lists[k], dumpItem(k, im, resolveRef, true))
			}
		}
	}
	return c
}

// dumpItem 解析结构体数组的一项；nest=true 时再下钻一层嵌套结构数组
// （SuperCurveObject.Path 的 positions 等）。
func dumpItem(field string, im map[string]any,
	resolveRef func(string, map[string]any) (string, bool), nest bool) kmdata.ComponentItem {
	item := kmdata.ComponentItem{
		Nums: map[string]float64{}, Strs: map[string]string{}, Refs: map[string]string{},
	}
	for ik, ivv := range im {
		switch itv := ivv.(type) {
		case int, int64, uint64, float64:
			item.Nums[ik] = uy.F(ivv)
		case string:
			item.Strs[ik] = itv
		case map[string]any:
			if _, hasID := itv["fileID"]; hasID {
				if val, isSprite := resolveRef(field+"."+ik, itv); val != "" && !isSprite {
					item.Refs[ik] = val
				}
			} else if _, hasX := itv["x"]; hasX {
				for _, axis := range []string{"x", "y", "z", "w"} {
					if av, ok := itv[axis]; ok {
						item.Nums[ik+"."+axis] = uy.F(av)
					}
				}
			}
		case []any:
			if !nest {
				continue
			}
			for _, nv := range itv {
				nm := uy.M(nv)
				if nm == nil {
					continue
				}
				if item.Items == nil {
					item.Items = map[string][]kmdata.ComponentItem{}
				}
				item.Items[ik] = append(item.Items[ik], dumpItem(field+"."+ik, nm, resolveRef, false))
			}
		}
	}
	return item
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
	n := 0
	must(filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(d.Name(), ".meta") {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		// 子目录音效（cheerReaders 的 Solo/Girls/All）保留相对路径作 key
		dst := filepath.Join(*outDir, "sounds", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		must(err)
		must(os.WriteFile(dst, b, 0o644))
		n++
		return nil
	}))
	fmt.Printf("sounds: %d copied\n", n)
}
