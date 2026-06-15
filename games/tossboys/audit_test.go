package tossboys

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

type tossBoysAssets struct {
	roles kmdata.Roles
	extra kmdata.Extra
}

func loadAssets(t *testing.T) tossBoysAssets {
	t.Helper()
	var out tossBoysAssets
	readJSON(t, "roles.json", &out.roles)
	readJSON(t, "extra.json", &out.extra)
	return out
}

func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "assets", "tossBoys", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

func TestTossBoysExtractedBindings(t *testing.T) {
	as := loadAssets(t)

	for _, role := range []string{
		"akachan", "aokun", "kiiyan", "hatchAnim", "soshiAnim",
		"ballPrefab", "specialAka", "specialAo", "specialKii",
		"soshi", "bg", "soshiPants",
	} {
		if as.roles[role] == "" {
			t.Fatalf("role %s missing", role)
		}
	}

	game := as.extra.Components["game"]
	if game.Refs["ballPrefab"] != "Ball" {
		t.Fatalf("game.ballPrefab = %q, want Ball", game.Refs["ballPrefab"])
	}
	paths := loadBallPaths(game.Lists["ballPaths"])
	if got := len(paths); got != 27 {
		t.Fatalf("ball path count = %d, want 27", got)
	}
	for _, name := range []string{
		"RedDispense", "BlueDispense", "YellowDispense",
		"RedBlue", "RedYellow", "BlueRed", "BlueYellow", "YellowRed", "YellowBlue",
		"RedBlueDual", "RedYellowDual", "BlueRedDual", "BlueYellowDual", "YellowRedDual", "YellowBlueDual",
		"RedBlueHigh", "RedYellowHigh", "BlueRedHigh", "BlueYellowHigh", "YellowRedHigh", "YellowBlueHigh",
		"RedBlur", "RedKeep", "BlueBlur", "BlueKeep", "YellowBlur", "YellowKeep",
	} {
		if len(paths[name].points) < 2 {
			t.Fatalf("path %s missing or underspecified", name)
		}
	}
}

func TestTossBoysKidComponents(t *testing.T) {
	as := loadAssets(t)
	want := map[string]struct {
		path, prefix, arrow, effect string
	}{
		"kid0": {"Akachan", "Aka", "Arrows/AkaChanArrow", "Akachan/HitParticle"},
		"kid1": {"Aokun", "Ao", "Arrows/AoKunArrow", "Aokun/HitParticle"},
		"kid2": {"Kiiyan", "Kii", "Arrows/KiiYanArrow", "Kiiyan/HitParticle"},
	}
	for key, w := range want {
		c := as.extra.Components[key]
		if c.Path != w.path || c.Strs["prefix"] != w.prefix ||
			c.Refs["arrow"] != w.arrow || c.Refs["_hitEffect"] != w.effect {
			t.Fatalf("%s = path %q prefix %q arrow %q effect %q",
				key, c.Path, c.Strs["prefix"], c.Refs["arrow"], c.Refs["_hitEffect"])
		}
	}
}
