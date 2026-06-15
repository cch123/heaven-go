package builttoscaleds

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
)

type builtToScaleDSAssets struct {
	Sheet       kmdata.Sheet
	Rig         kmdata.Rig
	Roles       kmdata.Roles
	Extra       kmdata.Extra
	Anims       map[string]*kmdata.Anim
	Controllers map[string]kmdata.Controller
	Animators   kmdata.Animators
	Sounds      map[string]bool
}

func loadBuiltToScaleDSAssets(t *testing.T) *builtToScaleDSAssets {
	t.Helper()
	root := filepath.Join("..", "..", "assets", "builtToScaleDS")
	as := &builtToScaleDSAssets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string]bool{},
	}
	readAssetJSON(t, filepath.Join(root, "sprites.json"), &as.Sheet)
	readAssetJSON(t, filepath.Join(root, "scene.json"), &as.Rig)
	readAssetJSON(t, filepath.Join(root, "roles.json"), &as.Roles)
	readAssetJSON(t, filepath.Join(root, "extra.json"), &as.Extra)
	readAssetJSON(t, filepath.Join(root, "anims.json"), &as.Anims)
	readAssetJSON(t, filepath.Join(root, "controllers.json"), &as.Controllers)
	readAssetJSON(t, filepath.Join(root, "animators.json"), &as.Animators)

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

func readAssetJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestBuiltToScaleDSBindings(t *testing.T) {
	as := loadBuiltToScaleDSAssets(t)
	if len(as.Sheet.Atlases) != 0 || len(as.Sheet.Sprites) != 0 {
		t.Fatalf("BuiltToScaleDS should be exported as 3D-only sheet, got %d atlases %d sprites", len(as.Sheet.Atlases), len(as.Sheet.Sprites))
	}
	for role, want := range map[string]string{
		"camPos":           "CameraPivot/CameraPos",
		"cameraPivot":      "CameraPivot",
		"flyingRodBase":    "Game/Models/Prefabs/FlyingRod",
		"movingBlocksBase": "Game/Models/Prefabs/MovingBlocks",
		"hitPartsBase":     "Game/Models/Prefabs/HitParts",
		"missPartsBase":    "Game/Models/Prefabs/MissParts",
		"partsHolder":      "Game/Models/PartsHolder",
		"blocksHolder":     "Game/Models/BlocksHolder",
		"shooterAnim":      "Game/Models/Shooter",
		"elevatorAnim":     "Game/Models/ElevatorWithRod",
	} {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}
}

func TestBuiltToScaleDSComponents(t *testing.T) {
	as := loadBuiltToScaleDSAssets(t)
	game := requireComponent(t, as.Extra, "game")
	for field, want := range map[string]string{
		"camPos":              "CameraPivot/CameraPos",
		"cameraPivot":         "CameraPivot",
		"environmentRenderer": "Game/Models/Environment",
		"elevatorRenderer":    "Game/Models/ElevatorWithRod",
		"flyingRodBase":       "Game/Models/Prefabs/FlyingRod",
		"movingBlocksBase":    "Game/Models/Prefabs/MovingBlocks",
		"hitPartsBase":        "Game/Models/Prefabs/HitParts",
		"missPartsBase":       "Game/Models/Prefabs/MissParts",
		"partsHolder":         "Game/Models/PartsHolder",
		"blocksHolder":        "Game/Models/BlocksHolder",
		"shooterAnim":         "Game/Models/Shooter",
		"elevatorAnim":        "Game/Models/ElevatorWithRod",
		"shooterMaterial":     "Shooter",
		"objectMaterial":      "Object",
		"gridPlaneMaterial":   "GridPlane",
		"elevatorMaterial":    "Grid",
		"beltMaterial":        "Belt",
	} {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}
	assertNear(t, game.Nums["cameraFoV"], 13)
	assertNear(t, game.Nums["beltSpeed"], 4.33)
	assertStringSlice(t, game.RefArrays["firstPatternLights"], []string{"Lights 3", "Lights 4", "Lights"})
	assertStringSlice(t, game.RefArrays["secondPatternLights"], []string{"Lights 1", "Lights 2"})

	blocks := requireComponent(t, as.Extra, "blocks")
	if blocks.Path != "Game/Models/Prefabs/MovingBlocks" {
		t.Fatalf("blocks path = %q", blocks.Path)
	}
	if got := blocks.Refs["anim"]; got != "Game/Models/Prefabs/MovingBlocks" {
		t.Fatalf("blocks anim = %q", got)
	}
	for name, wantPath := range map[string]string{
		"flyingRodPiece": "Game/Models/Prefabs/FlyingRod",
		"hitPartsPiece":  "Game/Models/Prefabs/HitParts",
		"missPartsPiece": "Game/Models/Prefabs/MissParts",
	} {
		c := requireComponent(t, as.Extra, name)
		if c.Path != wantPath || c.Refs["anim"] != wantPath {
			t.Fatalf("%s = path %q anim %q, want %q", name, c.Path, c.Refs["anim"], wantPath)
		}
	}
}

func TestBuiltToScaleDSControllersAndImportedClips(t *testing.T) {
	as := loadBuiltToScaleDSAssets(t)
	wantStates := map[string]map[string]string{
		"Shooter": {
			"Idle":     "Models/Shooter/machine_wait_machine",
			"Shoot":    "Models/Shooter/machine_shot_machine",
			"Windup":   "Models/Shooter/machine_touch_machine",
			"WindDown": "Models/Shooter/machine_touch_machine",
		},
		"Elevator": {
			"Down":    "Models/ElevatorWithRod/elevator_down",
			"Idle":    "Models/ElevatorWithRod/elevator_wait",
			"MakeRod": "Models/ElevatorWithRod/elevator_up",
		},
		"MovingBlocks": {"Move": "Models/MovingBlocks/piece_LR"},
		"FlyingRod":    {"Fly": "Models/FlyingRod/parts_airshot"},
		"HitParts":     {"PartsHit": "PartsHit"},
		"MissParts":    {"PartsMiss": "Models/MissParts/parts_ng"},
	}
	for ctrl, states := range wantStates {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for state, clip := range states {
			cs, ok := c.States[state]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, state)
			}
			if cs.Clip != clip {
				t.Fatalf("controller %s state %s clip = %q, want %q", ctrl, state, cs.Clip, clip)
			}
			if as.Anims[clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, state, clip)
			}
		}
	}
	for path, ctrl := range map[string]string{
		"Game/Models/Shooter":              "Shooter",
		"Game/Models/ElevatorWithRod":      "Elevator",
		"Game/Models/Prefabs/MovingBlocks": "MovingBlocks",
		"Game/Models/Prefabs/FlyingRod":    "FlyingRod",
		"Game/Models/Prefabs/HitParts":     "HitParts",
		"Game/Models/Prefabs/MissParts":    "MissParts",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
	assertNear(t, as.Anims["Models/MovingBlocks/piece_LR"].Duration, 80.0/24.0)
	assertNear(t, as.Anims["Models/Shooter/machine_shot_machine"].Duration, 28.0/24.0)
	assertNear(t, as.Anims["Models/ElevatorWithRod/elevator_up"].Duration, 40.0/24.0)
	assertNear(t, as.Anims["PartsHit"].Duration, 7.583333)
}

func TestBuiltToScaleDSSceneTargetsAndSounds(t *testing.T) {
	as := loadBuiltToScaleDSAssets(t)
	for _, path := range []string{
		"Game/Models/Environment",
		"Game/Models/Prefabs/HitParts/Pings",
		"Game/Models/Prefabs/HitParts/Pings/PingEffect",
		"Game/Models/Prefabs/HitParts/Pings/PingEffect (1)",
		"Game/Models/Prefabs/HitParts/parts_ok",
		"Game/Models/Prefabs/HitParts/parts_ok/parts",
		"Game/Models/Prefabs/HitParts/parts_ok/parts/effect00",
		"Game/Models/Prefabs/HitParts/parts_ok/parts/effect03",
	} {
		if !hasNode(as.Rig, path) {
			t.Fatalf("missing scene node %s", path)
		}
	}
	for _, snd := range []string{"Boing", "Crumble", "Hit", "Piano", "Sink"} {
		if !as.Sounds[snd] {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func requireComponent(t *testing.T, extra kmdata.Extra, name string) kmdata.Component {
	t.Helper()
	c, ok := extra.Components[name]
	if !ok {
		t.Fatalf("missing component %s", name)
	}
	return c
}

func hasNode(r kmdata.Rig, path string) bool {
	for _, n := range r.Nodes {
		if n.Path == path {
			return true
		}
	}
	return false
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d, want %d (%v)", len(got), len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
