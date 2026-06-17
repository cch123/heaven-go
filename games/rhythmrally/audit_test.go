package rhythmrally

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
)

type rhythmRallyAssets struct {
	Sheet       kmdata.Sheet
	Rig         kmdata.Rig
	Roles       kmdata.Roles
	Extra       kmdata.Extra
	Anims       map[string]*kmdata.Anim
	Controllers map[string]kmdata.Controller
	Animators   kmdata.Animators
	Meshes      kmdata.MeshData
	Particles   kmdata.ParticleData
	Sounds      map[string]bool
}

func loadRhythmRallyAssets(t *testing.T) *rhythmRallyAssets {
	t.Helper()
	root := filepath.Join("..", "..", "assets", "rhythmRally")
	as := &rhythmRallyAssets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string]bool{},
	}
	readRhythmRallyJSON(t, filepath.Join(root, "sprites.json"), &as.Sheet)
	readRhythmRallyJSON(t, filepath.Join(root, "scene.json"), &as.Rig)
	readRhythmRallyJSON(t, filepath.Join(root, "roles.json"), &as.Roles)
	readRhythmRallyJSON(t, filepath.Join(root, "extra.json"), &as.Extra)
	readRhythmRallyJSON(t, filepath.Join(root, "anims.json"), &as.Anims)
	readRhythmRallyJSON(t, filepath.Join(root, "controllers.json"), &as.Controllers)
	readRhythmRallyJSON(t, filepath.Join(root, "animators.json"), &as.Animators)
	readRhythmRallyJSON(t, filepath.Join(root, "meshes.json"), &as.Meshes)
	readRhythmRallyJSON(t, filepath.Join(root, "particles.json"), &as.Particles)

	soundRoot := filepath.Join(root, "sounds")
	if err := filepath.WalkDir(soundRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".wav" && ext != ".ogg" {
			return nil
		}
		rel, err := filepath.Rel(soundRoot, p)
		if err != nil {
			return err
		}
		as.Sounds[strings.TrimSuffix(filepath.ToSlash(rel), ext)] = true
		return nil
	}); err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func readRhythmRallyJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestRhythmRallyBindingsAndComponents(t *testing.T) {
	as := loadRhythmRallyAssets(t)
	if len(as.Sheet.Atlases) != 8 || len(as.Sheet.Sprites) != 8 {
		t.Fatalf("sheet = %d atlases %d sprites, want 8/8", len(as.Sheet.Atlases), len(as.Sheet.Sprites))
	}
	for _, sprite := range []string{"body", "boy", "L_stage", "L_stage_shadow", "stand"} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing sprite %s", sprite)
		}
	}

	wantRoles := map[string]string{
		"cameraPivot":  "Game/CameraPivot",
		"cameraPos":    "Game/CameraPivot/RallyCam",
		"ball":         "Game/Ball",
		"ballShadow":   "Game/Ball/Shadow",
		"ballTrail":    "Game/Ball/Particle System",
		"serveCurve":   "Game/Curves/ServeCurve",
		"returnCurve":  "Game/Curves/ReturnCurve",
		"tossCurve":    "Game/Curves/TossCurve",
		"missCurve":    "Game/Curves/MissCurve",
		"ballHitFX":    "Game/BounceFX",
		"playerAnim":   "Game/Paddlers/Player",
		"opponentAnim": "Game/Paddlers/Opponent",
		"BG":           "Game/stageLmodel",
		"WBG":          "Game/WhiteBG",
		"Lights":       "Game/CameraPivot/RallyCam/Lights",
		"paddlers":     "Game/Paddlers",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}

	game := requireRhythmRallyComponent(t, as.Extra, "game")
	for field, want := range wantRoles {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}
	if got := game.Refs["voidMat"]; got != "Void" {
		t.Fatalf("voidMat = %q, want Void", got)
	}
	assertRhythmRallyNear(t, game.Nums["cameraFOV"], 42)
	assertRhythmRallyNear(t, game.Nums["rallySpeed"], 1)
}

func TestRhythmRallyCurves(t *testing.T) {
	as := loadRhythmRallyAssets(t)
	for key, points := range map[string]int{
		"serveCurve":       3,
		"returnCurve":      3,
		"tossCurve":        3,
		"missCurve":        2,
		"game.serveCurve":  3,
		"game.returnCurve": 3,
		"game.tossCurve":   3,
		"game.missCurve":   2,
	} {
		c := as.Extra.Curves[key]
		if c.Sampling != 25 || len(c.Points) != points {
			t.Fatalf("curve %s = sampling %d points %d, want 25/%d", key, c.Sampling, len(c.Points), points)
		}
	}
	if as.Extra.Curves["serveCurve"].Points[0].P[2] < 2 {
		t.Fatalf("serve curve should start on opponent side, got z=%v", as.Extra.Curves["serveCurve"].Points[0].P[2])
	}
	if as.Extra.Curves["returnCurve"].Points[0].P[2] > -2 {
		t.Fatalf("return curve should start on player side, got z=%v", as.Extra.Curves["returnCurve"].Points[0].P[2])
	}
}

func TestRhythmRallyControllerImportedClips(t *testing.T) {
	as := loadRhythmRallyAssets(t)
	ctrl, ok := as.Controllers["Paddler"]
	if !ok {
		t.Fatal("missing Paddler controller")
	}
	if ctrl.Default != "Idle" {
		t.Fatalf("default state = %q, want Idle", ctrl.Default)
	}
	wantStates := map[string]struct {
		clip  string
		speed float64
	}{
		"Beat":      {"Models/Paddler/pingpong_beat", 4},
		"Idle":      {"Models/Paddler/pingpong_beat", 0},
		"Pose":      {"Models/Paddler/pingpong_pose", 4},
		"Ready0":    {"Models/Paddler/pingpong_ready00", 4},
		"Ready1":    {"Models/Paddler/pingpong_ready01", 4},
		"ReadyBeat": {"Models/Paddler/pingpong_ready_beat", 4},
		"Swing":     {"Models/Paddler/pingpong_swing", 4},
		"UnReady1":  {"Models/Paddler/pingpong_ready01", -4},
	}
	for state, want := range wantStates {
		got, ok := ctrl.States[state]
		if !ok {
			t.Fatalf("missing Paddler state %s", state)
		}
		if got.Clip != want.clip {
			t.Fatalf("state %s clip = %q, want %q", state, got.Clip, want.clip)
		}
		assertRhythmRallyNear(t, got.Speed, want.speed)
		if as.Anims[want.clip] == nil {
			t.Fatalf("state %s references missing imported clip %s", state, want.clip)
		}
	}
	for _, state := range []string{"Swing", "UnReady1"} {
		transitions := ctrl.States[state].Transitions
		if len(transitions) != 1 || transitions[0].Dst != "Idle" {
			t.Fatalf("state %s transitions = %#v, want single transition to Idle", state, transitions)
		}
	}
	for path, ctrl := range map[string]string{
		"Game/Paddlers/Player":   "Paddler",
		"Game/Paddlers/Opponent": "Paddler",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
}

func TestRhythmRallyMeshAndParticleAssets(t *testing.T) {
	as := loadRhythmRallyAssets(t)
	ball := rhythmRallyMeshBinding(t, as.Meshes, "Game/Ball")
	if ball.Renderer != "MeshRenderer" || ball.Mesh.GUID != "0" || ball.Mesh.FileID != 10207 {
		t.Fatalf("ball mesh binding = %#v, want built-in Sphere MeshRenderer", ball)
	}
	if len(ball.Materials) != 1 || ball.Materials[0].Name != "Ball" {
		t.Fatalf("ball materials = %#v, want Ball material", ball.Materials)
	}
	stage := rhythmRallyMeshBinding(t, as.Meshes, "Game/WhiteBG")
	if stage.Renderer != "SkinnedMeshRenderer" || stage.Mesh.Name != "stageLmodel" {
		t.Fatalf("stage binding = %#v, want stageLmodel SkinnedMeshRenderer", stage)
	}
	if geoms := as.Meshes.Geometries[stage.Mesh.GUID]; len(geoms) != 6 {
		t.Fatalf("stage geometry count = %d, want 6 from stageLmodel.fbx", len(geoms))
	}
	if got := len(as.Particles.Systems); got != 1 {
		t.Fatalf("particle systems = %d, want ball trail", got)
	}
	trail := as.Particles.Systems[0]
	if trail.Path != "Game/Ball/Particle System" || !trail.Looping || !trail.PlayOnAwake {
		t.Fatalf("ball trail particle = %#v", trail)
	}
	if trail.Emission.RateOverDistance.Scalar != 8 || trail.Renderer.LengthScale != 2 {
		t.Fatalf("ball trail params drifted: emission=%#v renderer=%#v", trail.Emission, trail.Renderer)
	}
}

func rhythmRallyMeshBinding(t *testing.T, meshes kmdata.MeshData, path string) kmdata.MeshBinding {
	t.Helper()
	for _, b := range meshes.Bindings {
		if b.Path == path {
			return b
		}
	}
	t.Fatalf("missing mesh binding %s", path)
	return kmdata.MeshBinding{}
}

func TestRhythmRallySceneTargetsAndSounds(t *testing.T) {
	as := loadRhythmRallyAssets(t)
	for _, path := range []string{
		"Game/Ball",
		"Game/Ball/Shadow",
		"Game/Ball/Particle System",
		"Game/BounceFX",
		"Game/Curves/ServeCurve",
		"Game/Curves/ReturnCurve",
		"Game/Curves/TossCurve",
		"Game/Curves/MissCurve",
		"Game/Paddlers/Player",
		"Game/Paddlers/Opponent",
		"Game/CameraPivot/RallyCam/Lights",
		"Game/WhiteBG",
	} {
		if !rhythmRallyHasNode(as.Rig, path) {
			t.Fatalf("missing scene node %s", path)
		}
	}
	for _, snd := range []string{"Serve", "ServeBounce", "Return", "ReturnBounce", "Tonk", "Tink", "Whistle", "common_miss"} {
		if !as.Sounds[snd] {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func requireRhythmRallyComponent(t *testing.T, extra kmdata.Extra, name string) kmdata.Component {
	t.Helper()
	c, ok := extra.Components[name]
	if !ok {
		t.Fatalf("missing component %s", name)
	}
	return c
}

func rhythmRallyHasNode(r kmdata.Rig, path string) bool {
	for _, n := range r.Nodes {
		if n.Path == path {
			return true
		}
	}
	return false
}

func assertRhythmRallyNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
