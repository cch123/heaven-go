package doubledate

import (
	"math"
	"path"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/doubleDate", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedSceneComponentsAndTemplates(t *testing.T) {
	as := loadAuditAssets(t)
	for _, node := range []string{
		"Boy", "Girl", "Tree", "Weasels", "Weasels/WeaselGirl", "Weasels/Shock2",
		"Background", "Background/CloudGroup", "Background/GradientBackground",
		"Background/GradientBackground/Square", "Weasels/WeaselBush", "Leaves",
		"DropShadow", "DropShadow/Shadow", "SoccerBall", "BasketBall", "Football",
	} {
		if _, ok := as.NodeIndex(node); !ok {
			t.Fatalf("missing scene node %s", node)
		}
	}
	for _, role := range []string{
		"boyAnim", "girlAnim", "weasels", "treeAnim", "clouds",
		"girlObj", "girlWeaselObj", "girlWeaselShockObj", "bgGO", "bushGO",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"SoccerBall", "BasketBall", "Football", "DropShadow"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
	comp := as.Extra.Components["game"]
	for _, ref := range []string{
		"soccer", "basket", "football", "dropShadow", "leaves",
		"boyAnim", "girlAnim", "weasels", "treeAnim", "clouds",
		"bgSquare", "bgGradient", "bgGO", "bushGO",
	} {
		if comp.Refs[ref] == "" {
			t.Fatalf("game component missing ref %s", ref)
		}
	}
	for _, num := range []string{"cloudSpeed", "cloudDistance", "floorHeight", "shadowDepthScaleMin", "shadowDepthScaleMax"} {
		if _, ok := comp.Nums[num]; !ok {
			t.Fatalf("game component missing num %s", num)
		}
	}
	if comp.Sprites["bgIntro"] != "GradientIntro" || comp.Sprites["bgLong"] != "GradientBackground" {
		t.Fatalf("background sprite refs not extracted: %#v", comp.Sprites)
	}
}

func TestControllersSoundsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"Boy": "Boy", "Girl": "Girl", "Tree": "Tree", "Weasels": "Weasels",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"Boy": {
			"Idle", "IdleStare", "IdleBop", "IdleBop2", "Ready", "UnReady", "Kick", "Barely",
		},
		"Girl":    {"IdleGirl", "GirlBlush", "GirlBop", "GirlLookUp", "GirlSad"},
		"Tree":    {"TreeIdle", "TreeRustle"},
		"Weasels": {"WeaselsIdle", "WeaselsBop", "WeaselsHappy", "WeaselsHide", "WeaselsHit", "WeaselsJump", "WeaselsSurprised", "WeaselsAppearUpset"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{
		"soccerBounce", "basketballBounce", "footballBounce", "kick", "footballKick",
		"kick_whiff", "weasel_hide", "weasel_hit", "weasel_scream", "common_miss",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
	for clip, root := range map[string]string{
		"Animations/Idle": "Boy", "Animations/IdleStare": "Boy", "Animations/IdleBop": "Boy",
		"Animations/IdleBop2": "Boy", "Animations/Ready": "Boy", "Animations/UnReady": "Boy",
		"Animations/Kick": "Boy", "Animations/Barely": "Boy",
		"Animations/IdleGirl": "Girl", "Animations/GirlBlush": "Girl", "Animations/GirlBop": "Girl",
		"Animations/GirlLookUp": "Girl", "Animations/GirlSad": "Girl",
		"Animations/TreeIdle": "Tree", "Animations/TreeRustle": "Tree",
		"Animations/WeaselsIdle": "Weasels", "Animations/WeaselsBop": "Weasels",
		"Animations/WeaselsHappy": "Weasels", "Animations/WeaselsHide": "Weasels",
		"Animations/WeaselsHit": "Weasels", "Animations/WeaselsJump": "Weasels",
		"Animations/WeaselsSurprised": "Weasels", "Animations/WeaselsAppearUpset": "Weasels",
	} {
		assertClipPaths(t, as, clip, root)
	}
}

func TestBallPathData(t *testing.T) {
	as := loadAuditAssets(t)
	paths := parseBallPaths(as.Extra.Components["game"].Lists["ballBouncePaths"])
	want := []string{
		"SoccerIn", "SoccerJust", "SoccerNgLate", "SoccerNgEarly",
		"BasketBallIn", "BasketBallJust", "BasketBallNgLate", "BasketBallNgEarly",
		"FootBallIn", "FootBallInNoHit", "FootBallNgLate", "FootBallNgEarly", "FootBallJust", "FootBallFall",
	}
	for _, name := range want {
		if paths[name] == nil || len(paths[name].points) < 2 {
			t.Fatalf("missing path %s", name)
		}
	}
	if got := paths["SoccerIn"].points[0].values["rot"]; math.Abs(got-135) > 1e-6 {
		t.Fatalf("SoccerIn first rot = %v", got)
	}
	if got := paths["FootBallFall"].points[0].values["rot"]; math.Abs(got-2160) > 1e-6 {
		t.Fatalf("FootBallFall rot = %v", got)
	}
	if tag := paths["FootBallIn"].points[4].tag; tag != "impact" {
		t.Fatalf("FootBallIn impact tag = %q", tag)
	}
	if got := pointTimeByTag(paths["FootBallIn"], "impact"); math.Abs(got-3.25) > 1e-6 {
		t.Fatalf("impact time = %v", got)
	}
	pos, h, val := samplePath(paths["SoccerIn"], 1.5, 0, [3]float64{})
	if pos[0] <= -7.75 || pos[0] >= -2.15 {
		t.Fatalf("sample x = %v", pos[0])
	}
	if h <= 0 || math.Abs(val("rot")-202.5) > 1e-6 {
		t.Fatalf("sample height/rot = %v/%v", h, val("rot"))
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
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}
