package seesaw

// 交付前审计：路径/锚点/剪辑/调色板，资产未提取时跳过。

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/seeSaw", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

// TestJumpPathsResolve：21 条 jumpPath 的双端点（Camera 除外）都在场景树。
func TestJumpPathsResolve(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	paths := as.Extra.Components["game"].Lists["jumpPaths"]
	if len(paths) != 21 {
		t.Fatalf("jumpPaths = %d, want 21", len(paths))
	}
	for _, p := range paths {
		name := p.Strs["name"]
		if name == "Camera" {
			continue // 纯抛物线（无 target）
		}
		for i, pos := range p.Items["positions"] {
			tgt := pos.Refs["target"]
			if tgt == "" || !nodeSet[tgt] {
				t.Errorf("path %s 端点 %d 目标 %q 未解析", name, i, tgt)
			}
		}
	}
	// 落点锚点
	for _, p := range []string{
		"Game/Curves/See/OutSee", "Game/Curves/See/InSee",
		"Game/Curves/Saw/OutSaw", "Game/Curves/Saw/InSaw",
		"Game/Curves/See/SeeStartJump/Point0",
	} {
		if !nodeSet[p] {
			t.Errorf("锚点 %q 不在场景树", p)
		}
	}
}

func TestGameLocalAnchorIgnoresCameraOffset(t *testing.T) {
	as := loadAssets(t)
	sc := kart.NewScene(as)
	const anchor = "Game/Curves/Saw/OutSaw"

	localAt := func(beat float64) [2]float64 {
		t.Helper()
		sc.Sample(beat)
		game, ok := sc.NodeWorld("Game")
		if !ok {
			t.Fatal("Game node missing")
		}
		node, ok := sc.NodeWorld(anchor)
		if !ok {
			t.Fatalf("%s node missing", anchor)
		}
		x, y := inverseApply(game, node.Tx, node.Ty)
		return [2]float64{x, y}
	}

	base := localAt(0)
	sc.SetPosOver("Game", 0, -7)
	shifted := localAt(1)
	if math.Abs(base[0]-shifted[0]) > 1e-9 || math.Abs(base[1]-shifted[1]) > 1e-9 {
		t.Fatalf("Game-local anchor changed after camera offset: base=%v shifted=%v", base, shifted)
	}
}

// TestStatesAndClips：模块用到的全部状态在 Saw/Seesaw controller 中存在。
func TestStatesAndClips(t *testing.T) {
	as := loadAssets(t)
	saw := as.Controllers["Saw"]
	for _, st := range []string{
		"NeutralSee", "NeutralSaw", "Jump_OutOut_Start", "Jump_InIn_Start",
		"Jump_OutIn_Start", "Jump_OutOut_Fall", "Jump_InIn_Fall",
		"Jump_InOut_Tuck", "Jump_OutIn_Tuck", "Jump_OutOut_Transform",
		"Jump_OutIn_Transform", "BadOut_SeeReact", "BadIn_SeeReact",
		"BopSaw", "BopSaw_Strum", "BopSee", "BopSee_Strum",
		"Choke_Saw_Intro", "Choke_Saw", "Choke_See_Intro", "Choke_See", "Explode",
		"Land_Out", "Land_In", "Land_Out_Big", "Land_In_Big",
		"Land_Out_Miss", "Land_In_Miss", "Land_Out_Barely", "Land_In_Barely",
		"GetUp_Out", "GetUp_In", "GetUp_Out_Big", "GetUp_In_Big",
		"GetUp_Out_Miss", "GetUp_In_Miss",
	} {
		if _, ok := saw.States[st]; !ok {
			t.Errorf("Saw controller 缺状态 %q", st)
		}
	}
	plank := as.Controllers["Seesaw"]
	for _, st := range []string{"Neut", "Good", "Bad", "Lightning"} {
		if _, ok := plank.States[st]; !ok {
			t.Errorf("Seesaw controller 缺状态 %q", st)
		}
	}
	for ctrl, states := range map[string][]string{
		"GuyInverter": {"NoInvert", "Invert"},
		"SeeInverter": {"NoInvert", "Invert"},
	} {
		c := as.Controllers[ctrl]
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Errorf("%s controller 缺状态 %q", ctrl, st)
			}
		}
	}
}

// TestMappedNodes：guy 子树是映射材质（recolor 生效面）。
func TestMappedNodes(t *testing.T) {
	as := loadAssets(t)
	mapped := 0
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			mapped++
		}
	}
	if mapped < 30 {
		t.Errorf("mapped 节点 %d 过少（guys + plank 应 30+）", mapped)
	}
	for _, s := range []string{"Seesaw1_33", "Seesaw1_34"} {
		if _, ok := as.Sheet.Sprites[s]; !ok {
			t.Errorf("轨道珠切片 %s 缺失", s)
		}
	}
}

func TestBopAndChokeAssets(t *testing.T) {
	as := loadAssets(t)
	for _, sound := range []string{"explosionBlack", "explosionWhite"} {
		if as.Sounds[sound] == nil {
			t.Fatalf("missing choke sound %q", sound)
		}
	}
	for comp, want := range map[string]string{
		"see": "Game/Guys/SeeHolder/BlackOrbs (1)",
		"saw": "Game/Guys/SawHolder/BlackOrbs (2)",
	} {
		got := as.Extra.Components[comp].Refs["deathParticle"]
		if got != want {
			t.Fatalf("%s deathParticle = %q, want %q", comp, got, want)
		}
		if _, ok := as.NodeIndex(got); !ok {
			t.Fatalf("%s deathParticle target %q missing", comp, got)
		}
	}
}

func TestBopEventSchedulesLikeUnityLoop(t *testing.T) {
	m := &Module{}
	if got := m.bopActionCount(0.5); got != 1 {
		t.Fatalf("bop count for length 0.5 = %d, want 1", got)
	}
	if got := m.bopActionCount(1); got != 1 {
		t.Fatalf("bop count for length 1 = %d, want 1", got)
	}
	if got := m.bopActionCount(1.5); got != 2 {
		t.Fatalf("bop count for length 1.5 = %d, want 2", got)
	}
	if got := m.bopActionCount(4); got != 4 {
		t.Fatalf("bop count for length 4 = %d, want 4", got)
	}
}

func TestChokeDefersUntilEndJumpLandingWindow(t *testing.T) {
	m := &Module{}
	g := &guy{canBop: true, wantChoke: 12, wantLen: 4}
	if !m.shouldRunQueuedChoke(g, 11.75) || !m.shouldRunQueuedChoke(g, 12.25) {
		t.Fatal("queued choke should run inside Unity +/-0.25 beat landing window")
	}
	if m.shouldRunQueuedChoke(g, 11.74) || m.shouldRunQueuedChoke(g, 12.26) {
		t.Fatal("queued choke should not run outside Unity +/-0.25 beat landing window")
	}
}
