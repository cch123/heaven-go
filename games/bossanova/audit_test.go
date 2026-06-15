package bossanova

import (
	"path"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "bossaNova"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsComponentsCurvesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"bossaAnim":    "CharacterPositions/Bossa",
		"novaAnim":     "CharacterPositions/Nova",
		"ringL":        "RingL",
		"ringR":        "RingR",
		"cloudAnim":    "Cloud",
		"positionAnim": "CharacterPositions",
		"ballShape":    "BallHolder",
		"cubeShape":    "CubeHolder",
		"bgOne":        "BG/1P BG",
		"bgTwo":        "BG/2P BG",
		"bgTwoSR":      "BG/2P BG/Square",
	}
	for k, want := range wantRoles {
		if got := as.Roles[k]; got != want {
			t.Fatalf("role %s = %q, want %q", k, got, want)
		}
	}

	for key, want := range map[string]string{
		"game":      "",
		"ballShape": "BallHolder",
		"cubeShape": "CubeHolder",
	} {
		if got := as.Extra.Components[key].Path; got != want {
			t.Fatalf("component %s path = %q, want %q", key, got, want)
		}
	}
	for key, refs := range map[string]map[string]string{
		"ballShape": {"shapeTransform": "BallHolder/Ball", "Shadow": "ShapeShadow", "enterCurve": "EnterCurveR", "hitCurve": "HitCurveR", "missCurve": "MissCurveR"},
		"cubeShape": {"shapeTransform": "CubeHolder/Cube", "Shadow": "ShapeShadow", "enterCurve": "EnterCurveL", "hitCurve": "HitCurveL", "missCurve": "MissCurveL"},
	} {
		for field, want := range refs {
			if got := as.Extra.Components[key].Refs[field]; got != want {
				t.Fatalf("%s.%s = %q, want %q", key, field, got, want)
			}
		}
	}
	for _, key := range []string{
		"ballShape.enterCurve", "ballShape.hitCurve", "ballShape.missCurve",
		"cubeShape.enterCurve", "cubeShape.hitCurve", "cubeShape.missCurve",
	} {
		c := as.Extra.Curves[key]
		if c.Sampling != 25 || len(c.Points) != 2 {
			t.Fatalf("curve %s = sampling %d points %d, want 25/2", key, c.Sampling, len(c.Points))
		}
	}
	for _, snd := range []string{
		"SE_BOSSA_EN_BALL", "SE_BOSSA_EN_BALL_MISS", "SE_BOSSA_EN_BALL_OTHER",
		"SE_BOSSA_EN_NUT", "SE_BOSSA_EN_NUT_OTHER", "SE_BOSSA_EN_MISS_HEAD",
		"SE_BOSSA_EN_SWING_BALL", "SE_BOSSA_EN_SWING_NUT",
		"SE_BOSSA_EN_CHANGE_PUSH", "SE_BOSSA_EN_CHANGE_ROLL_4",
		"Bossa/SE_BOSSA_EN_4", "Bossa/SE_BOSSA_EN_27", "Bossa/SE_BOSSA_EN_69",
		"Nova/SE_BOSSA_EN_28", "Nova/SE_BOSSA_EN_55", "Nova/SE_BOSSA_EN_71",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimationControllersAndPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"CharacterPositions":       "PositionsAnim",
		"CharacterPositions/Bossa": "BossaAnim",
		"CharacterPositions/Nova":  "NovaAnim",
		"Cloud":                    "CloudAnim",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"BossaAnim":     {"Idle", "Hit", "Barely", "Miss", "Whiff", "Head Bump", "Spin Left", "Spin Right"},
		"NovaAnim":      {"Idle", "Hit", "Whiff", "Head Bump", "Spin Left", "Spin Right"},
		"CloudAnim":     {"Idle", "Sink", "Spin"},
		"PositionsAnim": {"IdleL", "IdleR", "Sink Left", "Sink Right", "Spin Left", "Spin Right"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for ctrlName, ctrl := range as.Controllers {
		switch ctrlName {
		case "BossaAnim", "NovaAnim", "CloudAnim", "PositionsAnim":
		default:
			continue
		}
		for state, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			if as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrlName, state, st.Clip)
			}
		}
	}
	for clip, anim := range as.Anims {
		root, ok := rootForClip(clip)
		if !ok {
			continue
		}
		assertClipPaths(t, as, clip, root)
		assertSupportedFloatAttrs(t, clip, anim)
	}
}

func TestRuntimePatternConstants(t *testing.T) {
	m := &Module{}
	normal := m.patternThrows(10, false)
	wantNormal := []plannedThrow{
		{start: 9, voice: maleBounce1},
		{start: 10, voice: femaleBounce2, cube: true},
		{start: 10.5, voice: maleBounce3},
		{start: 11, voice: maleBounce4},
		{start: 11.5, voice: femaleBounce4, cube: true},
		{start: 12.5, voice: maleBounce6},
	}
	if len(normal) != len(wantNormal) {
		t.Fatalf("normal pattern len = %d, want %d", len(normal), len(wantNormal))
	}
	for i := range normal {
		if normal[i] != wantNormal[i] {
			t.Fatalf("normal throw %d = %#v, want %#v", i, normal[i], wantNormal[i])
		}
	}
	spin := m.patternThrows(10, true)
	if len(spin) != 5 || spin[3].voice != maleSpin || spin[4].voice != femaleSpin || !spin[3].spinVoice || !spin[4].spinVoice {
		t.Fatalf("spin pattern changed: %#v", spin)
	}
}

func rootForClip(clip string) (string, bool) {
	if strings.Contains(clip, "New Animation") {
		return "", false
	}
	switch {
	case strings.HasPrefix(clip, "Bossa/"), strings.HasPrefix(clip, "Bossa"):
		return "CharacterPositions/Bossa", true
	case strings.HasPrefix(clip, "Nova/"), strings.HasPrefix(clip, "Nova"):
		return "CharacterPositions/Nova", true
	case strings.HasPrefix(clip, "Cloud/"), strings.HasPrefix(clip, "Cloud"):
		return "Cloud", true
	case strings.HasPrefix(clip, "Positions/"), clip == "IdleL", clip == "IdleR",
		clip == "SinkLeft", clip == "SinkRight", clip == "SpinLeft", clip == "SpinRight":
		return "CharacterPositions", true
	default:
		return "", false
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("missing clip %s", clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s curve path %q resolved to missing node %q", clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}

func assertSupportedFloatAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	supported := map[string]bool{
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
		"m_IsActive": true, "m_Enabled": true, "m_FlipX": true, "m_FlipY": true,
		"m_SortingOrder": true, "m_Size.x": true, "m_Size.y": true,
	}
	for p, attrs := range anim.Floats {
		for attr := range attrs {
			if !supported[attr] {
				t.Fatalf("%s has unsupported float attr %s on %s", clip, attr, p)
			}
		}
	}
}
