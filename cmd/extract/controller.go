// controller.go：AnimatorController 状态机与 TMP 世界文本的提取。
//
// controller：HS 的 DoScaledAnimationAsync 按"状态名"播放，而状态名不一定等于
// 剪辑文件名（meatGrinder 的 TackMissDarkMeat → TackMissDark.anim），且 controller
// 携带运行时必需的转换语义（剪辑结束后按 bool 条件切到 idle/循环态）。
// 导出 controllers.json（状态机数据）+ animators.json（节点 path → controller 名）。
//
// TMP 文本：m_AtlasPopulationMode=1（动态填充）的字体资产 glyph 表为空，
// 因此导出源 OTF/TTF 字体文件 + 文本节点参数（texts.json），由运行时排版。
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

// scanGUIDs 递归扫描 root 下匹配后缀的 .meta，返回 guid → 资产文件路径（去掉 .meta）。
func scanGUIDs(root, suffix string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, suffix+".meta") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m, err := uy.ParseSingle(raw)
		if err == nil {
			if g := uy.S(m["guid"]); g != "" {
				out[g] = strings.TrimSuffix(p, ".meta")
				return nil
			}
		}
		// 上游仓库个别 .meta 残留 git 冲突标记（如 Kitties 的
		// CaughtFail.anim.meta），YAML 不可解析——按行扫第一个 guid。
		for _, line := range strings.Split(string(raw), "\n") {
			if s := strings.TrimSpace(line); strings.HasPrefix(s, "guid: ") {
				out[strings.TrimSpace(strings.TrimPrefix(s, "guid: "))] = strings.TrimSuffix(p, ".meta")
				break
			}
		}
		return nil
	})
	return out
}

func tmpAssetFamily(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "m_FamilyName:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "m_FamilyName:"))
		}
	}
	return ""
}

func findFontByFamily(root, family string) (string, bool) {
	if family == "" {
		return "", false
	}
	var found string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return err
		}
		if !strings.HasSuffix(p, ".otf.meta") && !strings.HasSuffix(p, ".ttf.meta") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		// Some TMP assets retain an old source-font GUID while Unity can still
		// resolve the installed font by family name. Keep the fallback narrow so
		// exact GUID matches remain the primary path.
		body := string(raw)
		if strings.Contains(body, "- "+family) || strings.Contains(body, "m_Name: "+family) {
			found = strings.TrimSuffix(p, ".meta")
		}
		return nil
	})
	return found, found != ""
}

// animNSKey 返回剪辑的命名空间 key（与 exportAnimDir 的约定一致：动画根相对路径）。
func animNSKey(animRoot, animPath string) string {
	rel, err := filepath.Rel(animRoot, animPath)
	if err != nil {
		rel = filepath.Base(animPath)
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), ".anim")
	return rel
}

func exportControllers(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string) {
	animDir := spec.animRoot()
	animGUIDs := scanGUIDs(animDir, ".anim")
	importedClips := scanImportedClips(bundlePath(spec.dir), spec.importedAnimFPS)
	ctrlGUIDs := scanGUIDs(animDir, ".controller")

	type ctrlFile struct {
		guid, base, nsKey, path string
	}
	var ctrlFiles []ctrlFile
	baseCount := map[string]int{}
	for g, p := range ctrlGUIDs {
		base := strings.TrimSuffix(filepath.Base(p), ".controller")
		ctrlFiles = append(ctrlFiles, ctrlFile{
			guid:  g,
			base:  base,
			nsKey: animNSKey(animDir, p),
			path:  p,
		})
		baseCount[base]++
	}

	// guid → controller key. Like .anim clips, controller basenames are not
	// globally unique in some official games (Super Samurai Slice has three
	// Demon/*/demon.controller files). Namespace duplicates so Animator
	// bindings cannot accidentally share the wrong state machine.
	ctrlName := map[string]string{}
	for _, c := range ctrlFiles {
		name := c.base
		if baseCount[c.base] > 1 {
			name = c.nsKey
		}
		ctrlName[c.guid] = name
	}

	ctrls := map[string]kmdata.Controller{}
	for _, c := range ctrlFiles {
		ctrls[ctrlName[c.guid]] = parseController(c.path, animDir, animGUIDs, importedClips)
	}
	writeJSON("controllers.json", ctrls)

	// prefab 内 Animator（classID 95）→ controller 绑定
	animators := kmdata.Animators{}
	for _, d := range dt.byID {
		if d.classID != 95 {
			continue
		}
		guid := uy.S(uy.Get(d.content, "m_Controller", "guid"))
		name, ok := ctrlName[guid]
		if !ok {
			if guid != "" {
				log.Printf("warn: Animator controller guid %s 不在游戏目录内，跳过", guid)
			}
			continue
		}
		gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
		p, ok := paths[gid]
		if !ok {
			log.Printf("warn: Animator GameObject &%d 不在场景树", gid)
			continue
		}
		animators[p] = name
	}
	writeJSON("animators.json", animators)
	fmt.Printf("controllers: %d 个，animator 绑定 %d 处\n", len(ctrls), len(animators))
}

func parseController(path, animRoot string, animGUIDs map[string]string, importedClips map[string]map[int64]importedClip) kmdata.Controller {
	raw, err := os.ReadFile(path)
	must(err)
	docs, err := uy.Parse(raw)
	must(err)

	type docRef struct {
		classID int
		content map[string]any
	}
	byID := map[int64]docRef{}
	for i := range docs {
		byID[docs[i].FileID] = docRef{docs[i].ClassID, docs[i].Content()}
	}

	out := kmdata.Controller{Params: map[string]bool{}, States: map[string]kmdata.CtrlState{}}

	// 状态名表（转换的 DstState 解析用）
	stateName := map[int64]string{}
	for id, d := range byID {
		if d.classID == 1102 {
			stateName[id] = uy.S(d.content["m_Name"])
		}
	}

	for _, d := range byID {
		switch d.classID {
		case 91: // AnimatorController：参数表
			for _, pv := range uy.L(d.content["m_AnimatorParameters"]) {
				p := uy.M(pv)
				if uy.I(p["m_Type"]) != 4 { // 4 = Bool；HS 游戏只用 bool
					log.Printf("warn: %s 参数 %s 类型 %d 非 bool，未支持",
						filepath.Base(path), uy.S(p["m_Name"]), uy.I(p["m_Type"]))
					continue
				}
				out.Params[uy.S(p["m_Name"])] = uy.I(p["m_DefaultBool"]) != 0
			}
		case 1107: // AnimatorStateMachine：默认状态
			if dflt := uy.I(uy.Get(uy.M(d.content["m_DefaultState"]), "fileID")); dflt != 0 {
				out.Default = stateName[dflt]
			}
			if l := uy.L(d.content["m_AnyStateTransitions"]); len(l) > 0 {
				log.Printf("warn: %s 含 %d 条 AnyState 转换，未支持", filepath.Base(path), len(l))
			}
		case 1102: // AnimatorState
			name := uy.S(d.content["m_Name"])
			st := kmdata.CtrlState{Speed: uy.F(d.content["m_Speed"])}
			motion := uy.M(d.content["m_Motion"])
			if g := uy.S(uy.Get(motion, "guid")); g != "" {
				if ap, ok := animGUIDs[g]; ok {
					st.Clip = animNSKey(animRoot, ap)
				} else if c, ok := importedClips[g][uy.I(motion["fileID"])]; ok {
					st.Clip = c.Key
				} else {
					log.Printf("warn: 状态 %s 的 motion guid %s 无对应 .anim", name, g)
				}
			}
			for _, tv := range uy.L(d.content["m_Transitions"]) {
				tid := uy.I(uy.Get(uy.M(tv), "fileID"))
				td, ok := byID[tid]
				if !ok || td.classID != 1101 {
					continue
				}
				tr := kmdata.CtrlTransition{
					Dst:      stateName[uy.I(uy.Get(uy.M(td.content["m_DstState"]), "fileID"))],
					ExitTime: uy.F(td.content["m_ExitTime"]),
					Duration: uy.F(td.content["m_TransitionDuration"]),
				}
				if uy.I(td.content["m_HasExitTime"]) != 0 && tr.ExitTime <= 0 {
					// hasExitTime=1 且 exitTime=0（Kitties 等）：作者意图为
					// "剪辑播完即转 idle"，归一为剪辑末端。
					tr.ExitTime = 1
				}
				if uy.I(td.content["m_HasExitTime"]) == 0 {
					// 无退出时间：条件满足即转换（运行时 gate=0 → 逐帧评估）
					tr.ExitTime = 0
					if len(uy.L(td.content["m_Conditions"])) == 0 {
						log.Printf("warn: %s 状态 %s 有无退出时间且无条件的转换（会立即触发）",
							filepath.Base(path), name)
					}
				}
				for _, cv := range uy.L(td.content["m_Conditions"]) {
					c := uy.M(cv)
					mode := "if"
					switch uy.I(c["m_ConditionMode"]) {
					case 1:
						mode = "if"
					case 2:
						mode = "ifnot"
					default:
						log.Printf("warn: 条件模式 %d 未支持（%s.%s）", uy.I(c["m_ConditionMode"]),
							filepath.Base(path), name)
						continue
					}
					tr.Conds = append(tr.Conds, kmdata.CtrlCond{Mode: mode, Param: uy.S(c["m_ConditionEvent"])})
				}
				st.Transitions = append(st.Transitions, tr)
			}
			out.States[name] = st
		}
	}
	return out
}

// ---------- TMP 文本 ----------

func exportTexts(dt *docTable, paths map[int64]string) {
	fontsRoot := filepath.Join(*hsRoot, "Assets", "Resources", "Fonts")
	assetGUIDs := scanGUIDs(fontsRoot, ".asset")
	otfGUIDs := scanGUIDs(fontsRoot, ".otf")
	ttfGUIDs := scanGUIDs(fontsRoot, ".ttf")

	// MeshRenderer（classID 23）按 GameObject 索引（TMP 文本的排序来自它）
	meshRend := map[int64]map[string]any{}
	for _, d := range dt.byID {
		if d.classID == 23 {
			meshRend[uy.I(uy.Get(d.content, "m_GameObject", "fileID"))] = d.content
		}
	}
	// RectTransform sizeDelta 按 GameObject 索引
	sizeDelta := map[int64][2]float64{}
	for _, d := range dt.byID {
		if sd, ok := d.content["m_SizeDelta"]; ok {
			gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
			sizeDelta[gid] = [2]float64{uy.F(uy.Get(uy.M(sd), "x")), uy.F(uy.Get(uy.M(sd), "y"))}
		}
	}

	var texts []kmdata.TextNode
	for _, d := range dt.byID {
		if d.classID != 114 {
			continue
		}
		txt, ok := d.content["m_text"]
		if !ok {
			continue
		}
		gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
		p, inScene := paths[gid]
		if !inScene {
			log.Printf("warn: TMP 文本 GameObject &%d 不在场景树", gid)
			continue
		}
		tn := kmdata.TextNode{
			Path: p,
			Text: uy.S(txt),
			Size: uy.F(d.content["m_fontSize"]),
			Color: [4]float64{
				uy.F(uy.Get(uy.M(d.content["m_fontColor"]), "r")),
				uy.F(uy.Get(uy.M(d.content["m_fontColor"]), "g")),
				uy.F(uy.Get(uy.M(d.content["m_fontColor"]), "b")),
				uy.F(uy.Get(uy.M(d.content["m_fontColor"]), "a")),
			},
			Rect:   sizeDelta[gid],
			HAlign: int(uy.I(d.content["m_HorizontalAlignment"])),
			VAlign: int(uy.I(d.content["m_VerticalAlignment"])),
		}
		if r := meshRend[gid]; r != nil {
			tn.Order = int(uy.I(r["m_SortingOrder"]))
			tn.Layer = int(uy.I(r["m_SortingLayer"]))
		}
		// 字体链：m_fontAsset guid → TMP .asset → m_SourceFontFileGUID → OTF/TTF
		fontGUID := uy.S(uy.Get(uy.M(d.content["m_fontAsset"]), "guid"))
		assetPath, ok := assetGUIDs[fontGUID]
		if !ok {
			log.Fatalf("TMP 字体资产 guid %s 未在 %s 找到", fontGUID, fontsRoot)
		}
		raw, err := os.ReadFile(assetPath)
		must(err)
		adocs, err := uy.Parse(raw)
		must(err)
		var srcGUID string
		for i := range adocs {
			if g := uy.S(adocs[i].Content()["m_SourceFontFileGUID"]); g != "" {
				srcGUID = g
				break
			}
		}
		fontPath, ok := otfGUIDs[srcGUID]
		if !ok {
			fontPath, ok = ttfGUIDs[srcGUID]
		}
		if !ok {
			fontPath, ok = findFontByFamily(fontsRoot, tmpAssetFamily(raw))
		}
		if !ok {
			log.Fatalf("TMP 字体 %s 的源字体文件 guid %s 未找到（动态字体必须有源文件）",
				filepath.Base(assetPath), srcGUID)
		}
		must(os.MkdirAll(filepath.Join(*outDir, "fonts"), 0o755))
		b, err := os.ReadFile(fontPath)
		must(err)
		tn.Font = filepath.Base(fontPath)
		must(os.WriteFile(filepath.Join(*outDir, "fonts", tn.Font), b, 0o644))
		texts = append(texts, tn)
	}
	writeJSON("texts.json", texts)
	for _, t := range texts {
		fmt.Printf("text %-20q @ %s size=%.1f font=%s order=%d\n", t.Text, t.Path, t.Size, t.Font, t.Order)
	}
}
