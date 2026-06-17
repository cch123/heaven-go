package fanclub

import (
	"math"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/fanClub", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

var fanClubModuleDrivenClips = map[string]bool{
	arisaFacePrefix + "EyeMiddle":      true,
	arisaFacePrefix + "EyeEast":        true,
	arisaFacePrefix + "EyeWest":        true,
	arisaFacePrefix + "EyeNorth":       true,
	arisaFacePrefix + "EyeNorthRaised": true,
	arisaFacePrefix + "EyeSouth":       true,
}

func TestFanClubBindingsTemplatesAndSounds(t *testing.T) {
	as := loadAssets(t)
	nodeSet := nodeSet(as)
	for role, want := range map[string]string{
		"StageAnimator":   "Background",
		"Arisa":           "Idol_rootMotion/Idol",
		"ArisaRootMotion": "Idol_rootMotion",
		"ArisaShadow":     "idol_Shadow",
		"Blue":            "dancerR_rootMotion/Blue",
		"Orange":          "dancerL_rootMotion/Orange",
		"spectator":       "Fan",
		"spectatorAnchor": "fan_SpawnAnchor",
	} {
		if got := as.Roles[role]; got != want || !nodeSet[got] {
			t.Errorf("role %s = %q, want scene path %q", role, got, want)
		}
	}
	if tmpl := kart.NewTemplate(as, as.Roles["spectator"]); tmpl == nil {
		t.Fatalf("Fan spectator template not resolved")
	}
	for path, ctrl := range map[string]string{
		"Background":                "Background",
		"Idol_rootMotion/Idol":      "Arisa",
		"dancerR_rootMotion/Blue":   "Blue",
		"dancerL_rootMotion/Orange": "Orange",
		"Fan":                       "Fan",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Errorf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
	fan := as.Extra.Components["fan"]
	if fan.Path != "Fan" || fan.Refs["motionRoot"] != "Fan/root_motion" || fan.Refs["shadow"] != "Fan/fan_Shadow" {
		t.Fatalf("fan component refs drifted: %#v", fan)
	}
	arisa := as.Extra.Components["arisa"]
	if arisa.Path != as.Roles["Arisa"] || arisa.Refs["facePoser"] != as.Roles["Arisa"]+"/idol_head/FacePoser" {
		t.Fatalf("arisa component refs drifted: %#v", arisa)
	}
	for _, path := range []string{
		"Background/bgCol/Square",
		"Background/bgCol/Square (1)",
		"Background/bgFlash",
	} {
		n, ok := sceneNode(as, path)
		if !ok {
			t.Fatalf("missing Fan Club background node %s", path)
		}
		if n.Sprite != kart.UnitySquareSprite {
			t.Fatalf("background node %s sprite = %q, want %q", path, n.Sprite, kart.UnitySquareSprite)
		}
	}
	for _, snd := range []string{
		"play_clap", "crap_impact", "play_jump", "landing_impact", "crowd_big_ready",
		"jp/arisa_hai_1_jp", "jp/arisa_ka_jp", "jp/crowd_hai_jp", "jp/crowd_ne_jp",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestFanClubControllersAndNamespacedFaceposerClips(t *testing.T) {
	as := loadAssets(t)
	for ctrlName, states := range map[string][]string{
		"Arisa":      {"NoPose", "NoPoseArrange", "IdolBeat", "IdolBeatArrange", "IdolCall0", "IdolBigCall1Arrange", "IdolDab"},
		"Background": {"Bg", "Bg_Light", "Bg_Spot"},
		"Blue":       {"NoPose", "Beat", "Crap", "Jump", "WalkA", "WalkB", "Dab", "MouthA", "EyeLeft"},
		"Orange":     {"NoPose", "Beat", "Crap", "Jump", "WalkA", "WalkB", "Dab", "MouthA", "EyeRight"},
		"Fan":        {"NoPose", "FanBeat", "FanPrepare", "FanClap", "FanClapCharge", "FanJump", "FanBigReady", "FanFaceAngry"},
	} {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, st := range states {
			cs, ok := ctrl.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrlName, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrlName, st, cs.Clip)
			}
		}
	}
	for _, clip := range []string{
		arisaFacePrefix + "MouthA",
		arisaFacePrefix + "EyeLeft",
		backupFacePrefix + "MouthA",
		backupFacePrefix + "EyeRight",
		"Animations/Arisa/Long/IdolBeat",
		"Animations/Arisa/Arrange/IdolBeatArrange",
		"Animations/BackDancers/Blue/Dab",
		"Animations/BackDancers/Orange/Dab",
		"Animations/Fan/FanClap",
		"Animations/Fan/Head/FanFaceAngry",
		"Animations/Stage/Bg_Light",
	} {
		if as.Anims[clip] == nil {
			t.Errorf("missing clip %s", clip)
		}
	}
	if as.Anims["FacePoser/MouthA"] != nil {
		t.Fatalf("ambiguous short FacePoser/MouthA clip exported; Arisa and BackDancers must stay namespaced")
	}
}

func TestFanClubAllClipsAccountedAndPathsResolve(t *testing.T) {
	as := loadAssets(t)
	nodeSet := nodeSet(as)
	ctrlClips := map[string]bool{}
	for animPath, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			ctrlClips[st.Clip] = true
			checkAnimPaths(t, as.Anims[st.Clip], st.Clip, animPath, nodeSet)
			checkSupportedAttrs(t, as.Anims[st.Clip], st.Clip)
			if as.Anims[st.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrlName, stName, st.Clip)
			}
		}
	}
	for clip := range fanClubModuleDrivenClips {
		checkAnimPaths(t, as.Anims[clip], clip, as.Roles["Arisa"], nodeSet)
		checkSupportedAttrs(t, as.Anims[clip], clip)
	}
	for name := range as.Anims {
		if !strings.Contains(name, "/") {
			continue
		}
		if !ctrlClips[name] && !fanClubModuleDrivenClips[name] {
			t.Errorf("clip %q has no controller state or module driver", name)
		}
	}
}

func TestFanClubLayeredFaceposerSamplingAndTimingConstants(t *testing.T) {
	as := loadAssets(t)
	sc := kart.NewScene(as)
	root := as.Roles["Arisa"]
	sc.PlayLayer("target", root, arisaFacePrefix+"EyeNorthRaised", 0, 1)
	sc.PlayLayerNormalized("shape", root, arisaFacePrefix+"EyeLeft", eyeNorm(6, 6))
	sc.Sample(0)
	if got, _, _ := sc.NodeSprite(root + "/idol_head/FacePoser/EyeL/EyeSprite"); got != "fanClub_IdolParts_19" {
		t.Fatalf("normalized eye shape sprite = %q, want fanClub_IdolParts_19", got)
	}
	if fanCount != 12 || radius != 1.5 || dancerAnimCount != 16 {
		t.Fatalf("layout constants drifted: fanCount=%d radius=%v dancerAnimCount=%d", fanCount, radius, dancerAnimCount)
	}
	if got := fanClubSeq("arisa_hai"); len(got) != 3 || got[0].beat != 0 || got[1].beat != 1 || got[2].beat != 2 {
		t.Fatalf("arisa_hai sequence drifted: %#v", got)
	}
	if got := fanClubSeq("arisa_kamone"); len(got) != 3 || got[1].name != "jp/arisa_mo_jp" || got[1].beat != 0.5 {
		t.Fatalf("arisa_kamone sequence drifted: %#v", got)
	}
	if got := fanClubSeq("crowd_big_ready"); len(got) != 1 || got[0].name != "crowd_big_ready" {
		t.Fatalf("crowd_big_ready sequence drifted: %#v", got)
	}
}

func TestFanClubFanTemplateUsesUnitySortingGroupOrder(t *testing.T) {
	as := loadAssets(t)
	tmpl := kart.NewTemplate(as, as.Roles["spectator"])
	if tmpl == nil {
		t.Fatal("Fan spectator template not resolved")
	}
	inst := tmpl.NewInstance()
	inst.SetGroupOrder(2)
	f := &fan{inst: inst, groupOrder: 2, groupKey: -1}
	sc := kart.NewScene(as)

	f.queue(sc, 0)

	if f.groupKey < 0 {
		t.Fatal("fan queue did not assign a dynamic group key")
	}
	if len(sc.QueuedSpritesForTest()) == 0 {
		t.Fatal("fan queue produced no sprites")
	}
	for _, q := range sc.QueuedSpritesForTest() {
		if !q.HasGroup || q.GroupOrder != 2 {
			t.Fatalf("fan sprite sorting group = %#v, want group order 2", q)
		}
		if q.Order >= 100 {
			t.Fatalf("fan local renderer order was flattened into global order: %#v", q)
		}
	}
}

func TestFanClubFanClapEffectUsesPrefabEmitterOutsideFanSortingGroup(t *testing.T) {
	as := loadAssets(t)
	tmpl := kart.NewTemplate(as, as.Roles["spectator"])
	if tmpl == nil {
		t.Fatal("Fan spectator template not resolved")
	}
	inst := tmpl.NewInstance()
	f := &fan{inst: inst, groupOrder: 2, groupKey: 77}
	sc := kart.NewScene(as)
	m := &Module{ctx: &engine.Ctx{Assets: as, Scene: sc}}
	fx := fanClubEffects{bursts: []fanClubEffectBurst{{
		beat: 0, secPerBeat: 0.5, lifetime: fanClubClapLifetimeSec,
		kind: fanClubEffectFanClap, fan: f, relPath: "Effect_FanCrap",
		scale: 0.24, order: 32, tint: [4]float64{1, 1, 1, 1},
	}}}

	fx.queue(m, 0)

	qs := sc.QueuedSpritesForTest()
	if len(qs) != fanClubFanParticleCount {
		t.Fatalf("queued effect sprites = %d, want %d", len(qs), fanClubFanParticleCount)
	}
	emitter, ok := inst.NodeWorldAt("Effect_FanCrap", kart.Identity(), 0)
	if !ok {
		t.Fatal("missing Fan/Effect_FanCrap emitter")
	}
	for _, q := range qs {
		if q.HasGroup {
			t.Fatalf("fan clap effect should not be inside root_motion SortingGroup: %#v", q)
		}
		if q.Order != 32 {
			t.Fatalf("fan clap effect order = %d, want ParticleSystemRenderer order 32", q.Order)
		}
		if q.Sprite != fanClubEffectSprite {
			t.Fatalf("fan clap effect sprite = %q, want %q", q.Sprite, fanClubEffectSprite)
		}
		dist := math.Hypot(q.World.Tx-emitter.Tx, q.World.Ty-emitter.Ty)
		if dist <= 0.05 || dist > 0.35 {
			t.Fatalf("fan clap effect should start as a small spread from Effect_FanCrap, dist=%v q=(%v,%v), emitter=(%v,%v)", dist, q.World.Tx, q.World.Ty, emitter.Tx, emitter.Ty)
		}
	}
}

func TestFanClubFanClapKeepsHeadVisibleAndOffsetsParticles(t *testing.T) {
	as := loadAssets(t)
	tmpl := kart.NewTemplate(as, as.Roles["spectator"])
	if tmpl == nil {
		t.Fatal("Fan spectator template not resolved")
	}
	inst := tmpl.NewInstance()
	inst.PlayState("", "FanClap", 0, 0.5)
	sc := kart.NewScene(as)
	inst.Queue(sc, 0, kart.Identity(), 0)
	if !queuedSpritePrefix(sc, "fan_Face") {
		t.Fatal("FanClap should keep the monkey head sprite visible")
	}

	f := &fan{inst: inst, groupOrder: 2, groupKey: 77}
	fx := fanClubEffects{bursts: []fanClubEffectBurst{{
		beat: 0, secPerBeat: 0.5, lifetime: fanClubClapLifetimeSec,
		kind: fanClubEffectFanClap, fan: f, relPath: "Effect_FanCrap",
		scale: 0.24, order: 32, tint: [4]float64{1, 1, 1, 1},
	}}}
	sc = kart.NewScene(as)
	m := &Module{ctx: &engine.Ctx{Assets: as, Scene: sc}}
	fx.queue(m, 0.45)

	anchor, ok := inst.NodeWorldAt("Effect_FanCrap", kart.Identity(), 0.45)
	if !ok {
		t.Fatal("missing sampled Effect_FanCrap emitter")
	}
	moved := false
	for _, q := range sc.QueuedSpritesForTest() {
		if math.Hypot(q.World.Tx-anchor.Tx, q.World.Ty-anchor.Ty) > 0.05 {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatal("fan clap particles should travel away from the authored emitter instead of sitting on the monkey head")
	}
}

func TestFanClubIdolClapUsesUnityParticleOrderAndHandAnchor(t *testing.T) {
	as := loadAssets(t)
	sc := kart.NewScene(as)
	root := as.Roles["Arisa"]
	sc.PlayState(root, "IdolCrap", 0, 1)
	sc.Sample(0.05)
	m := &Module{ctx: &engine.Ctx{Assets: as, Scene: sc}, arisa: root}
	fx := fanClubEffects{bursts: []fanClubEffectBurst{{
		beat: 0, secPerBeat: 0.5, lifetime: fanClubClapLifetimeSec,
		kind: fanClubEffectClap, path: "Effect_IdolCrap",
		scale: 0.62, order: 0, tint: [4]float64{1, 1, 1, 1},
	}}}

	fx.queue(m, 0.05)

	qs := sc.QueuedSpritesForTest()
	if len(qs) != 5 {
		t.Fatalf("idol clap particles = %d, want 5 from Unity burst", len(qs))
	}
	hand, ok := fx.bursts[0].actorHandWorld(m)
	if !ok {
		t.Fatal("missing idol hand midpoint")
	}
	for _, q := range qs {
		if q.Order != 0 {
			t.Fatalf("idol clap particle order = %d, want Unity ParticleSystemRenderer order 0", q.Order)
		}
		if math.Hypot(q.World.Tx-hand.Tx, q.World.Ty-hand.Ty) > 0.25 {
			t.Fatalf("idol clap particle at (%v,%v) is not anchored to hands (%v,%v)", q.World.Tx, q.World.Ty, hand.Tx, hand.Ty)
		}
	}
}

func queuedSpritePrefix(sc *kart.SceneInst, prefix string) bool {
	for _, q := range sc.QueuedSpritesForTest() {
		if strings.HasPrefix(q.Sprite, prefix) {
			return true
		}
	}
	return false
}

func nodeSet(as *kart.Assets) map[string]bool {
	out := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		out[n.Path] = true
	}
	return out
}

func sceneNode(as *kart.Assets, path string) (kmdata.Node, bool) {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n, true
		}
	}
	return kmdata.Node{}, false
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip, root string, nodes map[string]bool) {
	t.Helper()
	if anim == nil {
		t.Errorf("clip %s missing", clip)
		return
	}
	for p := range animPaths(anim) {
		full := p
		if root != "" && p != "" {
			full = root + "/" + p
		} else if p == "" {
			full = root
		}
		if !nodes[full] {
			t.Errorf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
	}
}

func animPaths(anim *kmdata.Anim) map[string]bool {
	paths := map[string]bool{}
	for p := range anim.Pos {
		paths[p] = true
	}
	for p := range anim.Euler {
		paths[p] = true
	}
	for p := range anim.Scale {
		paths[p] = true
	}
	for p := range anim.Sprites {
		paths[p] = true
	}
	for p := range anim.Floats {
		paths[p] = true
	}
	return paths
}

func checkSupportedAttrs(t *testing.T, anim *kmdata.Anim, clip string) {
	t.Helper()
	if anim == nil {
		return
	}
	for _, attrs := range anim.Floats {
		for attr := range attrs {
			if !fanClubSupportedFloatAttr(attr) {
				t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
			}
		}
	}
}

func fanClubSupportedFloatAttr(attr string) bool {
	switch attr {
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	default:
		return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
	}
}
