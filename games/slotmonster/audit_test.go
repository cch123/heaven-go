package slotmonster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

func TestExtractedSlotMonsterAssets(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "slotMonster")
	var extra kmdata.Extra
	readJSON(t, filepath.Join(root, "extra.json"), &extra)
	if got := extra.RefArrays["buttons"]; len(got) != 3 {
		t.Fatalf("buttons refs = %v, want 3", got)
	}
	if got := extra.RefArrays["eyeAnims"]; len(got) != 3 {
		t.Fatalf("eyeAnims refs = %v, want 3", got)
	}
	for i := 0; i < 3; i++ {
		b := extra.Components["button"+intString(i)]
		if b.Path == "" || b.Refs["anim"] == "" || len(b.RefArrays["srs"]) != 2 {
			t.Fatalf("button%d component incomplete: %#v", i, b)
		}
	}

	var anims map[string]kmdata.Anim
	readJSON(t, filepath.Join(root, "anims.json"), &anims)
	for _, name := range []string{"SlotMonster/Prepare", "SlotMonster/Release", "SlotMonster/Win", "SlotMonster/Lose", "Button/Flash", "Button/Press", "Eyes/Spin"} {
		if _, ok := anims[name]; !ok {
			t.Fatalf("missing clip %s", name)
		}
	}
	for i := 1; i <= 10; i++ {
		for _, suffix := range []string{"", "Barely"} {
			name := "Eyes/EyeItem" + intString(i) + suffix
			if _, ok := anims[name]; !ok {
				t.Fatalf("missing eye result clip %s", name)
			}
		}
	}

	for _, snd := range []string{"common_bassDrumNTR.wav", "common_snareDrumNTR.wav", "start_touch.wav", "rolling.wav", "win.wav"} {
		if _, err := os.Stat(filepath.Join(root, "sounds", snd)); err != nil {
			t.Fatalf("missing sound %s: %v", snd, err)
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
