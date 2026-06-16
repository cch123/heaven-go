package valiantvolley

import (
	"math"
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/valiantVolley", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestValiantVolleyBindingsCurvesAndSounds(t *testing.T) {
	as := loadAssets(t)
	nodes := nodeSet(as)
	game := as.Extra.Components["game"]
	if got := game.Refs["volleyObject"]; got != "ObjectHolder" {
		t.Fatalf("volleyObject = %q", got)
	}
	wantAnts := []string{"Ants/AntLeft", "Ants/AntMiddle", "Ants/AntPlayer"}
	if got := game.RefArrays["ants"]; len(got) != len(wantAnts) {
		t.Fatalf("ants refs = %v", got)
	} else {
		for i, want := range wantAnts {
			if got[i] != want || !nodes[got[i]] {
				t.Fatalf("ants[%d] = %q, want %q", i, got[i], want)
			}
		}
	}
	obj := as.Extra.Components["object"]
	if obj.Path != "ObjectHolder" || obj.Refs["objectSprite"] != "ObjectHolder/Object" ||
		obj.Refs["missImpact"] != "ObjectHolder/Object/missImpact" || obj.Sprites["fruitSprite"] != "fruit" {
		t.Fatalf("object component drifted: %#v", obj)
	}
	for _, curve := range []string{
		"object.enterCurve", "object.bounceCurve1", "object.bounceCurve2", "object.hitCurve", "object.barelyCurve",
	} {
		c := as.Extra.Curves[curve]
		if c.Sampling != 25 || len(c.Points) != 2 {
			t.Fatalf("curve %s = sampling %d points %d", curve, c.Sampling, len(c.Points))
		}
	}
	for _, snd := range []string{"dirtHit", "dirtMiss", "fruitHit", "fruitMiss", "woosh", "common_nearMiss"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestValiantVolleyControllersClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"AntAnim":    {"Idle", "AntBop", "AntHappy", "AntAngry", "AntOops", "AntPrepare", "Volley"},
		"ObjectAnim": {"Neutral", "ObjectHit", "ObjectJuggle", "ObjectBarely"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for path, ctrl := range map[string]string{
		"Ants/AntLeft":   "AntAnim",
		"Ants/AntMiddle": "AntAnim",
		"Ants/AntPlayer": "AntAnim",
		"ObjectHolder":   "ObjectAnim",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Errorf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
	nodes := nodeSet(as)
	ctrlClips := map[string]bool{}
	for root, ctrlName := range as.Animators {
		for stName, st := range as.Controllers[ctrlName].States {
			if st.Clip == "" {
				continue
			}
			ctrlClips[st.Clip] = true
			a := as.Anims[st.Clip]
			if a == nil {
				t.Errorf("%s/%s missing clip %s", ctrlName, stName, st.Clip)
				continue
			}
			checkAnimPaths(t, a, st.Clip, root, nodes)
			checkSupportedAttrs(t, a, st.Clip)
		}
	}
	for name := range as.Anims {
		if strings.Contains(name, "/") && !ctrlClips[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
}

func TestValiantVolleyTimingAndMultiSpawnSemantics(t *testing.T) {
	ev := hitEvt{beat: 12, length: 2, typ: objDirt}
	plan := objectPlan{start: ev.beat - ev.length, distance: ev.length, typ: ev.typ}
	o := &volleyObject{plan: plan, hitBeat: math.Inf(1)}
	if got, want := o.targetBeat(), 16.0; got != want {
		t.Fatalf("target beat = %v, want %v", got, want)
	}
	m := &Module{}
	iv := intervalEvt{beat: 20, length: 4}
	multi := m.multiSpawn([]hitEvt{
		{beat: 20, length: 1, typ: objFruit},
		{beat: 21, length: 1, typ: objFruit},
		{beat: 23, length: 1, typ: objFruit},
	}, iv)
	if multi.start != 16 || multi.distance != 4 || !multi.juggle {
		t.Fatalf("multi plan = %#v", multi)
	}
	if len(multi.inputs) != 2 || multi.inputs[0] != 21 || multi.inputs[1] != 23 {
		t.Fatalf("multi inputs = %v", multi.inputs)
	}
	if multi.lastJuggle != 23 || multi.lastJuggleLength != 4 {
		t.Fatalf("last juggle = beat %v len %v", multi.lastJuggle, multi.lastJuggleLength)
	}
	if actionFruit != 3 {
		t.Fatalf("fruit action channel = %d, want 3", actionFruit)
	}
	if got, want := passTurnJustHitClearBeat(20, 4), 31.5; got != want {
		t.Fatalf("PassTurn justHit clear beat = %v, want %v", got, want)
	}
	if got, want := passTurnJustHitClearBeat(ev.beat, ev.length), 17.5; got != want {
		t.Fatalf("standalone hit justHit clear beat = %v, want %v", got, want)
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	out := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		out[n.Path] = true
	}
	return out
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip, root string, nodes map[string]bool) {
	t.Helper()
	for p := range animPaths(anim) {
		full := root
		if p != "" {
			full = root + "/" + p
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
	for _, attrs := range anim.Floats {
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Errorf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	default:
		return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
	}
}
