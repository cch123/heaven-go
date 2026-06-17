package fanclub

import "testing"

func TestFanClubEffectRefsResolve(t *testing.T) {
	as := loadAssets(t)
	if _, ok := as.Sheet.Sprites[fanClubEffectSprite]; !ok {
		t.Fatalf("missing shared Fan Club effect sprite %q", fanClubEffectSprite)
	}
	nodes := nodeSet(as)
	for comp, refs := range map[string][]string{
		"arisa": {"idolClapEffect", "idolWinkEffect", "idolKissEffect", "idolWinkArrEffect"},
		"amie1": {"clapEffect", "winkEffect"},
		"fan":   {"fanClapEffect"},
	} {
		c := as.Extra.Components[comp]
		if c.Path == "" {
			t.Fatalf("missing component %s", comp)
		}
		for _, ref := range refs {
			path := c.Refs[ref]
			if path == "" {
				t.Fatalf("component %s missing effect ref %s", comp, ref)
			}
			if !nodes[path] {
				t.Fatalf("component %s effect ref %s points to missing node %q", comp, ref, path)
			}
		}
	}
	for _, path := range []string{
		"dancerL_rootMotion/Orange/Effect_IdolCrap",
		"dancerL_rootMotion/Orange/Effect_IdolWinkArr",
	} {
		if !nodes[path] {
			t.Fatalf("left dancer fallback effect node missing: %s", path)
		}
	}
}

func TestFanClubEffectBurstLifecycle(t *testing.T) {
	burst := fanClubEffectBurst{
		beat: 10, secPerBeat: 0.5, lifetime: 0.45,
		scale: 1, tint: [4]float64{1, 0.5, 0.25, 0.8},
	}
	if _, keep, draw := burst.sample(9.99); !keep || draw {
		t.Fatalf("future burst keep/draw = %v/%v, want true/false", keep, draw)
	}
	start, keep, draw := burst.sample(10)
	if !keep || !draw || start.tint[3] <= 0 || start.scale <= 0 {
		t.Fatalf("start sample = %#v keep=%v draw=%v", start, keep, draw)
	}
	mid, keep, draw := burst.sample(10.45)
	if !keep || !draw {
		t.Fatalf("mid sample keep/draw = %v/%v, want true/true", keep, draw)
	}
	if mid.tint[3] >= start.tint[3] {
		t.Fatalf("effect alpha did not fade: start=%v mid=%v", start.tint[3], mid.tint[3])
	}
	if _, keep, draw := burst.sample(10.9); keep || draw {
		t.Fatalf("expired burst keep/draw = %v/%v, want false/false", keep, draw)
	}
}
