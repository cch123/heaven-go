package fireworks

import (
	"math"
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/fireworks", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, p := range []string{
		"BG", "BG/Cityold", "BG/City_Old",
		"SpawnPositions/Left", "SpawnPositions/Right", "SpawnPositions/Middle",
		"BombCurve/Point 0", "BombCurve/Point 1", "BombSpawnPoint",
		"agasgagag/Gradient", "agasgagag/City", "agasgagag/Sky",
		"agasgagag/Stars", "agasgagag/Faces", "agasgagag/Faces/face_0", "agasgagag/Faces/face_1",
	} {
		if !nodeSet[p] {
			t.Errorf("scene path %q missing", p)
		}
	}
	for _, snd := range requiredSounds() {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
	for _, sprite := range requiredSprites() {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Errorf("missing sprite %s", sprite)
		}
	}
}

func TestControllersAndRuntimeClips(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"Firework", "Bomb"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for _, want := range []struct {
		ctrl  string
		state string
		clip  string
	}{
		{"Firework", "Rocket", "Animations/Rocket"},
		{"Firework", "Sparkler", "Animations/Sparkler"},
		{"Firework", "End", "Animations/End"},
		{"Bomb", "IdleBomb", "Animations/IdleBomb"},
		{"Bomb", "ExplodeBomb", "Animations/ExplodeBomb"},
	} {
		st, ok := as.Controllers[want.ctrl].States[want.state]
		if !ok {
			t.Errorf("controller %s missing state %s", want.ctrl, want.state)
			continue
		}
		if st.Clip != want.clip {
			t.Errorf("controller %s state %s clip = %q, want %q", want.ctrl, want.state, st.Clip, want.clip)
		}
		if _, ok := as.Anims[want.clip]; !ok {
			t.Errorf("clip %q missing", want.clip)
		}
	}
}

func TestAllClipsAccounted(t *testing.T) {
	as := loadAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
			}
		}
	}
	for name := range as.Anims {
		if !strings.Contains(name, "/") {
			continue
		}
		if !ctrlClips[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
}

func TestFloatAttrsSupported(t *testing.T) {
	as := loadAssets(t)
	supported := map[string]bool{
		"m_IsActive": true, "m_Enabled": true, "m_SortingOrder": true,
		"m_FlipX": true, "m_FlipY": true, "m_Size.x": true, "m_Size.y": true,
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
	}
	for name, a := range as.Anims {
		for path, attrs := range a.Floats {
			for attr := range attrs {
				if !supported[attr] {
					t.Errorf("clip %s path %q attr %q unsupported", name, path, attr)
				}
			}
		}
	}
}

func TestSpriteSwapsResolve(t *testing.T) {
	as := loadAssets(t)
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

func TestRuntimeRules(t *testing.T) {
	if rocketTimer(false) != 3 || rocketTimer(true) != 1 {
		t.Fatalf("rocket timers changed")
	}
	if rocketLifetime(false) <= rocketTimer(false) || rocketLifetime(true) <= rocketTimer(true) {
		t.Fatalf("rocket lifetimes must outlive input timers")
	}
	if countSound(countOne) != "count1" || countSound(countTwo) != "count2" ||
		countSound(countThree) != "count3" || countSound(countHey) != "countHey" {
		t.Fatalf("count-in sound mapping changed")
	}
	x, y := spawnPoint(spawnLeft)
	if math.Abs(x+5.04) > 1e-9 || math.Abs(y+6.48) > 1e-9 {
		t.Fatalf("left spawn = (%v,%v)", x, y)
	}
	rx, ry := rocketPos(rocketCue{beat: 8, spawn: spawnMiddle}, 11)
	if math.Abs(rx) > 1e-9 || ry <= -2 || ry >= 0 {
		t.Fatalf("rocket hit position = (%v,%v), want centered mid-air", rx, ry)
	}
	for typ := explosionBig; typ <= explosionMixed; typ++ {
		if b := newBurst(12, 0, 0, typ); len(b.particles) == 0 {
			t.Fatalf("explosion type %d produced no particles", typ)
		}
	}
}

func requiredSounds() []string {
	return []string{
		"common_applause", "count1", "count2", "count3", "countHey",
		"explode_5", "miss", "nuei", "practice1", "practice2", "practice3",
		"practiceAww", "practiceHai", "rocket_2", "taikoExplode", "tamaya_4",
	}
}

func requiredSprites() []string {
	return []string{
		"bg_gradient", "bomb", "city_1", "city_2", "city_3", "city_4", "city_5",
		"explosion_0", "explosion_1", "explosion_2", "explosion_3", "explosion_4", "explosion_5",
		"rocket_0", "rocket_1", "rocket_2", "rocket_3", "rocket_4", "rocket_5",
		"rocket_6", "rocket_7", "rocket_8", "rocket_9", "rocket_10", "rocket_11",
		"sparkBlue_0", "sparkBlue_1", "sparkGreen_0", "sparkGreen_1", "sparkRed_0", "sparkRed_1",
	}
}
