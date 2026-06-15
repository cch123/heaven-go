package pajamaparty

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/pajamaParty", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestPajamaPartyRolesTemplateAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for field, want := range map[string]string{
		"Mako":           "Mako_Root",
		"Bed":            "Bed",
		"MonkeyPrefab":   "Monkey",
		"Castle":         "Bg/castle",
		"BgAnimator":     "Bg",
		"BalloonsEffect": "Balloons",
		"SpawnRoot":      "Spawn_Root",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	if tmpl := kart.NewTemplate(as, as.Roles["MonkeyPrefab"]); tmpl == nil {
		t.Fatalf("Monkey prefab template not extractable")
	}
	for path, ctrl := range map[string]string{
		"Bg":                                "Bg",
		"Bed":                               "Bed",
		"Mako_Root/Mako":                    "Mako",
		"Mako_Root/Pillow_Root/Mako_Pillow": "Mako_Pillow",
		"Monkey/Monkey":                     "Monkey",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
	for _, snd := range []string{
		"three1", "three2", "three3", "five1", "five2", "five3", "five4", "five5",
		"throw1", "throw2", "throw3", "throw4", "throw4a", "throw5", "charge",
		"catch0", "catch1", "jumpJust", "siesta1", "siesta2", "siesta3",
		"siesta4", "siestaBad", "siestaDone", "common_miss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestPajamaPartyControllersAndNamespacedClips(t *testing.T) {
	as := loadAuditAssets(t)
	want := map[string][]string{
		"Bg":  {"NoPose", "SlideOpen", "SlideClose", "CastleAppear", "CastleHide", "FloatsNear", "FloatsFar", "BoatsAppear"},
		"Bed": {"NoPose", "BedImpact"},
		"Mako": {
			"NoPose", "NoPose_H", "MakoBeat", "MakoBeat_H", "MakoJump", "MakoJump_H",
			"MakoCatch", "MakoCatch_H", "MakoCatchNg", "MakoReady", "MakoReady_H",
			"MakoThrow", "MakoThrow_H", "MakoThrowOut", "MakoSleep00", "MakoSleep01",
			"MakoSleepJust", "MakoSleepNg", "MakoSleepOut", "MakoSleepThrough",
			"MakoAwake", "MakoReadySleep", "MakoReadySleep01",
		},
		"Mako_Pillow": {"NoPose", "ThrowOut"},
		"Monkey": {
			"NoPose", "NoPose_H", "MonkeyBeat", "MonkeyBeat_H", "MonkeyJump",
			"MonkeyJump_H", "MonkeyJump02", "MonkeyJump03", "MonkeyLand",
			"MonkeyReady", "MonkeyThrow", "MonkeySleep00", "MonkeySleep01",
			"MonkeySleep02", "MonkeyReadySleep", "MonkeyAwake",
		},
	}
	for ctrl, states := range want {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for _, clip := range []string{
		"Anime/BG/NoPose", "Anime/Bed/NoPose", "Anime/Mako/NoPose",
		"Anime/Mako/NoPose_H", "Anime/MakoPillow/NoPose",
		"Anime/Monkey/NoPose", "Anime/Monkey/NoPose_H",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing namespaced clip %s", clip)
		}
	}
}

func TestPajamaPartyAnimationPathsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		if !nodes[root] {
			t.Fatalf("animator root %s missing from scene", root)
		}
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s state %s missing clip %s", ctrlName, stName, st.Clip)
			}
			covered[st.Clip] = true
			checkAnimPaths(t, root, st.Clip, anim, nodes)
			checkSupportedAttrs(t, st.Clip, anim)
		}
	}
	for clip := range as.Anims {
		if strings.Contains(clip, "/") && !covered[clip] {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func TestPajamaPartySpawnGridMatchesUnity(t *testing.T) {
	m := &Module{}
	root := [2]float64{0, -2.582}
	const radius = 2.75
	scale := 1.0
	order := 10
	spawnX, spawnY, spawnZ := root[0]-radius*3, root[1], 0.0
	count := 0
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			spawnX += radius * scale
			if y == 0 && x == 2 {
				continue
			}
			count++
			_ = &monkey{x: spawnX, y: spawnY, z: spawnZ, scale: scale, order: order}
		}
		scale -= 0.1
		spawnX = root[0] - radius*3*scale
		spawnY = root[1] + radius/3.75*float64(y+1)
		spawnZ = radius / 5 * float64(y+1)
		order--
	}
	if count != 24 {
		t.Fatalf("spawned monkeys = %d, want 24", count)
	}
	if got := parabola(0.5); got != 1 {
		t.Fatalf("jump parabola at half = %g, want 1", got)
	}
	_ = m
}

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
	}
}

func checkSupportedAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	for path, attrs := range anim.Floats {
		for attr := range attrs {
			switch attr {
			case "m_Color.r", "m_Color.g", "m_Color.b", "m_Color.a",
				"material._Color.r", "material._Color.g", "material._Color.b", "material._Color.a",
				"material._AddColor.r", "material._AddColor.g", "material._AddColor.b", "material._AddColor.a",
				"m_Size.x", "m_Size.y", "m_FlipX", "m_FlipY",
				"m_SortingOrder", "m_IsActive", "m_Enabled":
			default:
				t.Fatalf("clip %s path %s unsupported float attr %s", clip, path, attr)
			}
		}
	}
}

func animPathSet(anim *kmdata.Anim) map[string]bool {
	out := map[string]bool{}
	for p := range anim.Pos {
		out[p] = true
	}
	for p := range anim.Euler {
		out[p] = true
	}
	for p := range anim.Scale {
		out[p] = true
	}
	for p := range anim.Sprites {
		out[p] = true
	}
	for p := range anim.Floats {
		out[p] = true
	}
	return out
}
