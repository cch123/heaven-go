package figurefighter

import (
	"path"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "figureFighter"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsRolesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"dollAnim":        "Doll",
		"crowdAnim":       "Background/Crowds",
		"bagAnim":         "Bag",
		"bagCopyAnim":     "BagCopy",
		"buttonAnim":      "Button",
		"lightsAnim":      "Background/Lights",
		"topAnim":         "TopsSpots",
		"barsAnim":        "StickyBars/Bars",
		"fartAnim":        "Fart",
		"bagObject":       "Bag",
		"chainParticles1": "ChainParticle/Chain",
		"chainParticles2": "ChainParticle/Chain (1)",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}
	if got := as.Extra.Components["game"].Refs["StickyLayer"]; got != "StickyBars" {
		t.Fatalf("StickyLayer ref = %q, want StickyBars", got)
	}
	for _, snd := range []string{
		"and", "and2", "applause", "barely", "crowdEnd", "crowdGo1", "crowdGo2", "crowdGo3",
		"crowdJab", "crowdOne", "crowdOneFast", "crowdStart", "crowdTwo", "crowdTwoFast",
		"failHit", "fastOneTwo1", "fastOneTwo2", "go1", "go2", "go3", "goHit1", "goHit2",
		"goLastHit", "hit1Cheer", "hit2Cheer", "hit3Cheer", "jab", "jabHit", "oneTwo1",
		"oneTwo2", "oneTwoHit1", "oneTwoHit2", "oneTwoHit2Fast", "powerHit", "pump",
		"regularHit", "whiffFart", "whiffPress",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
	for i := 1; i <= 8; i++ {
		snd := "break" + string(rune('0'+i))
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	wantStates := map[string][]string{
		"BagCopy":     {"Blow", "idle"},
		"Bars":        {"CloseIn1", "CloseIn2", "CloseIn3", "Idle"},
		"Fart":        {"gas_L", "gas_S", "idle"},
		"FeverBag":    {"BagBarely", "BagBlow", "BagHit1", "BagHit2", "BagIdle", "BagSummon", "BagThrough"},
		"FeverButton": {"ButtonIdle", "ButtonPress", "ButtonPress2"},
		"FeverCrowd":  {"CrowdBop0", "CrowdBop1", "CrowdBop2", "CrowdIdle"},
		"FeverFigure": {
			"FigureBarely1", "FigureBarely2", "FigureBarely3", "FigureBarely4",
			"FigureBop", "FigureBop2", "FigureBop3", "FigureBop4",
			"FigureDeflate", "FigureDeflateBarely", "FigureFinisher", "FigureIdle", "FigureInflate",
			"FigureJab", "FigureJab2", "FigurePrep1", "FigurePrep2", "FigurePrep3", "FigurePrep4",
			"FigureWhiff1", "FigureWhiff2",
		},
		"FeverLights": {
			"BackFade1", "BackFade2", "BackFade3", "BackFade4", "BackFade5", "BackFade6",
			"BackFadeO1", "BackFadeO2", "BackFadeO3", "BackFadeO4", "BackFadeO5", "BackFadeO6",
			"BackFlash1", "BackFlash2", "BackFlash3", "BackFlash4", "BackFlash5", "BackFlash6",
			"LightAmbient", "LightsOff",
		},
		"FeverSpots": {"SpotsFadeIn", "SpotsFadeOut", "SpotsOff"},
	}
	for ctrlName, states := range wantStates {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, state := range states {
			st, ok := ctrl.States[state]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, state)
			}
			if st.Clip != "" && as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, state, st.Clip)
			}
		}
	}
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stateName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			checkAnimPaths(t, root, ctrlName, stateName, st.Clip, as.Anims[st.Clip], nodes)
		}
	}
}

func TestAllClipsAccountedAndSpriteSwapsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
				if alias := path.Base(st.Clip); alias != st.Clip {
					ctrlClips[alias] = true
				}
			}
		}
	}
	for name := range as.Anims {
		if !ctrlClips[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("clip %s path %q sprite %q missing from atlas", name, path, k.Name)
				}
			}
		}
	}
}

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	ev := figureBopEvt(&riq.Entity{
		Beat: 4, Length: 2,
		Data: map[string]any{"bop": true, "auto": false, "crowd": true, "crowdAuto": true},
	})
	if ev.dollManual || !ev.dollPulse || !ev.crowdManual || !ev.crowdPulse {
		t.Fatalf("bop parameter mapping changed: %#v", ev)
	}
	m := &Module{bops: []bopEvt{{beat: 4, dollManual: true}, {beat: 8, dollManual: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || m.autoBopAt(8) {
		t.Fatalf("autoBopAt did not follow SetupBopRegion semantics")
	}
	if crowdBopState(-1) != "CrowdBop0" || crowdBopState(1) != "CrowdBop1" || crowdBopState(4) != "CrowdBop2" {
		t.Fatalf("crowdBopState did not clamp to controller states")
	}
	if !isNG(-1) || !isNG(1) || isNG(0.99) || isNG(-0.99) {
		t.Fatalf("isNG must match state >= 1 || state <= -1")
	}
}

func checkAnimPaths(t *testing.T, root, ctrl, state, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	if anim == nil {
		t.Fatalf("clip %s missing", clip)
	}
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
	for p, attrs := range anim.Floats {
		paths[p] = true
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Fatalf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
	for p := range paths {
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] {
			t.Fatalf("controller %s state %s clip %s path %q missing under %q", ctrl, state, clip, p, root)
		}
	}
}
