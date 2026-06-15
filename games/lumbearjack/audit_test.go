package lumbearjack

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/lumbearjack", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestLumbearjackExtractedBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"_bear", "_baby", "_smallObjectPrefab", "_bigObjectPrefab", "_hugeObjectPrefab",
		"_catRight", "_catLeft", "_particleHitPoint", "_particleCutPoint", "_snowParticle",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"SmallObject", "BigObject", "HugeObject"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
	game := as.Extra.Components["game"]
	for _, field := range []string{"_catRightObjectsSmall", "_catLeftObjectsSmall", "_bgCats"} {
		if len(game.RefArrays[field]) == 0 {
			t.Fatalf("missing ref array %s", field)
		}
	}
	if len(game.RefArrays["_catRightObjectsSmall"]) != 6 || len(game.RefArrays["_catRightObjectsBig"]) != 2 || len(game.RefArrays["_catRightObjectsHuge"]) != 3 {
		t.Fatalf("cat object arrays do not match Unity enum sizes")
	}
}

func TestLumbearjackControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"Beast": {"BeastIdle", "BeastBop", "BeastCut", "BeastCutMid", "BeastCutMidNoImpact", "BeastHalfCut", "BeastHuhL", "BeastHuhR", "BeastReady", "BeastRest", "BeastWhiff"},
		"Cat":   {"CatIdle", "CatBop", "CatDance", "CatGrab"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, state := range states {
			st, ok := c.States[state]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, state)
			}
			if st.Clip != "" && as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, state, st.Clip)
			}
		}
	}
	for _, sound := range []string{
		"readyVoice", "smallLogPut", "smallLogCut", "canPut", "canCut", "bigLogHit",
		"bigLogCut", "hugeLogHit1", "hugeLogHit2", "hugeLogHit3", "freezerCut",
		"peachCut", "huh", "swing", "sighA", "sighB", "common_miss",
	} {
		if len(as.Sounds[sound]) == 0 {
			t.Fatalf("missing sound %s", sound)
		}
	}
}

func TestLumbearjackBareCatHolderIsNotScriptDriven(t *testing.T) {
	as := loadAssets(t)
	if _, ok := as.NodeIndex("CatHolder/Arms"); ok {
		t.Fatal("bare CatHolder unexpectedly grew full cat Arms branch; re-audit CatDance")
	}
	if as.Roles["_catRight"] == "CatHolder" || as.Roles["_catLeft"] == "CatHolder" {
		t.Fatalf("bare CatHolder must not be used as a main cat role: %#v", as.Roles)
	}
	for _, bg := range as.Extra.Components["game"].RefArrays["_bgCats"] {
		if bg == "CatHolder" {
			t.Fatal("bare CatHolder must not be used as a background cat")
		}
	}
}

func TestLumbearjackCatSideSemantics(t *testing.T) {
	m := &Module{
		cats: []catPresenceEvt{
			{beat: 0, main: mainCatBoth},
			{beat: 8, main: mainCatLeft},
		},
		catPuts: []objectEvt{
			{beat: 2, cat: catAlternate},
			{beat: 4, cat: catAlternate},
			{beat: 9, cat: catAlternate},
		},
	}
	if !m.shouldBeRight(2, catAlternate) {
		t.Fatalf("first alternate should use right cat when both are present")
	}
	if m.shouldBeRight(4, catAlternate) {
		t.Fatalf("second alternate should flip to left cat")
	}
	if m.shouldBeRight(9, catAlternate) {
		t.Fatalf("left-only presence should force alternate to left")
	}
}
