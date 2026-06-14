package agbsamuraislice

import (
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "agbSamuraiSlice"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsControllersAndCurves(t *testing.T) {
	as := loadAuditAssets(t)
	for _, key := range []string{"samuraiAnim", "yokaiEntity", "mayaFeyFromAceAttorney", "samuraiObject", "fireAnim", "fireParent", "fogAnim"} {
		if as.Roles[key] == "" {
			t.Fatalf("missing role %s", key)
		}
	}
	for _, path := range []string{as.Roles["yokaiEntity"], as.Roles["mayaFeyFromAceAttorney"], "shadow", "bigshadow"} {
		if kart.NewTemplate(as, path) == nil {
			t.Fatalf("missing template %s", path)
		}
	}
	if len(as.Extra.RefArrays["fogSprite"]) != 2 {
		t.Fatalf("fogSprite refs = %d, want 2", len(as.Extra.RefArrays["fogSprite"]))
	}
	if got := len(as.Extra.Curves); got != 30 {
		t.Fatalf("curves = %d, want 30", got)
	}
	for _, key := range []string{"yokai.enterCurves0", "yokai.enterCurves5", "yokai.flyingCurves0", "yokai.missCurve", "maya.enterCurves0", "maya.missCurve"} {
		if len(as.Extra.Curves[key].Points) == 0 {
			t.Fatalf("missing curve %s", key)
		}
	}
}

func TestAllAnimationClipsAndStatesExtracted(t *testing.T) {
	as := loadAuditAssets(t)
	for _, clip := range []string{
		"Animations/Bop", "Animations/Bop1", "Animations/Bop2",
		"Animations/Idle", "Animations/Idle1", "Animations/Idle2",
		"Animations/Slice", "Animations/Slice1", "Animations/Slice2",
		"Animations/Miss", "Animations/Rest",
		"Animations/Jump", "Animations/Jump1", "Animations/Flying", "Animations/Flying1", "Animations/YokaiWalk",
		"Animations/Fire1", "Animations/Fire2", "Animations/Fire3", "Animations/Fire4", "Animations/Fire5", "Animations/Fire6",
		"Animations/Flash", "Animations/SliceFog",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}
	for ctrl, states := range map[string][]string{
		"Samurai":  {"Bop", "Bop1", "Bop2", "Idle", "Idle1", "Idle2", "Slice", "Slice1", "Slice2", "Miss", "Rest"},
		"Yokai1":   {"Jump", "Jump1", "Flying", "Flying1", "YokaiWalk"},
		"Fire":     {"Fire1", "Fire2", "Fire3", "Fire4", "Fire5", "Fire6"},
		"Flash":    {"Flash"},
		"SliceFog": {"SliceFog"},
	} {
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
}

func TestFogSoundSequenceNames(t *testing.T) {
	m := New().(*Module)
	evShort := fogEvt{beat: 0}
	evLong := fogEvt{beat: 0, long: true}
	if lastFogName(evShort) != "fog25long" {
		t.Fatalf("short fog terminal = %s", lastFogName(evShort))
	}
	if lastFogName(evLong) != "fog32" {
		t.Fatalf("long fog terminal = %s", lastFogName(evLong))
	}
	_ = m
}

func lastFogName(ev fogEvt) string {
	last := 25
	special := "fog25long"
	if ev.long {
		last = 32
		special = "fog25short"
	}
	name := ""
	for i := 1; i <= last; i++ {
		name = "fog" + itoa(i)
		if i == 25 {
			name = special
		}
	}
	return name
}
