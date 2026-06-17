package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestScanGameActionsHandlesTargetTypedActions(t *testing.T) {
	src := `
return new Minigame("demo", "Demo", "fff", false, false, new List<GameAction>()
{
    new("bop", "Bop")
    {
        parameters = new()
        {
            new("toggle", true, "Toggle")
        }
    },
    new GameAction("hit", "Hit")
    {
        parameters = new List<Param>()
        {
            new Param("strength", 1, "Strength")
        }
    },
},
new List<string>() { "ctr" });
`
	got := scanGameActions(src)
	want := []string{"bop", "hit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
}

func TestScanGameActionsHandlesTargetTypedActionList(t *testing.T) {
	src := `
return new Minigame("demo", "Demo", "fff", false, false, new()
{
    new GameAction("countIn/count-in", "Count-In"),
    new("start", "Start")
    {
        parameters = new()
        {
            new("cue", true, "Cue")
        }
    },
});
`
	got := scanGameActions(src)
	want := []string{"countIn/count-in", "start"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
}

func TestScanRegisterCallsAcrossRootGoFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":     "package main\nfunc main() {}\n",
		"registry.go": "package main\nfunc registerGames() { engine.Register(\"airRally\", nil) }\n",
		"ignored.txt": "engine.Register(\"notGo\", nil)",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := scanRegisterCalls(dir)
	if !got["airRally"] {
		t.Fatalf("registration in registry.go was not detected: %v", got)
	}
	if got["notGo"] {
		t.Fatalf("non-Go files should not be scanned: %v", got)
	}
}

func TestScanExtractSpecsIncludesLegacyKarateMan(t *testing.T) {
	dir := t.TempDir()
	main := `package main
func main() {
	if *game != "karateman" {
		extractScene(*game)
		return
	}
	exportRigAndStage(nil)
}`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	specs := `package main
var sceneSpecs = map[string]sceneSpec{
	"rhythmSomen": basicOfficialSceneSpec("RhythmSomen", "rhythmSomen.prefab"),
}`
	if err := os.WriteFile(filepath.Join(dir, "scene.go"), []byte(specs), 0o644); err != nil {
		t.Fatal(err)
	}

	got := scanExtractSpecs(dir)
	for _, id := range []string{"karateman", "rhythmSomen"} {
		if !got[id] {
			t.Fatalf("missing extract spec %s in %v", id, got)
		}
	}
}

func TestMarchingOrdersOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "marchingOrders", filepath.Join("..", "..", "games", "marchingorders"))
}

func TestQuizShowOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "quizShow", filepath.Join("..", "..", "games", "quizshow"))
}

func TestWarioDeMamboOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "warioDeMambo", filepath.Join("..", "..", "games", "wariodemambo"))
}

func TestSpaceSoccerOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "spaceSoccer", filepath.Join("..", "..", "games", "spacesoccer"))
}

func TestTapTrialOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "tapTrial", filepath.Join("..", "..", "games", "taptrial"))
}

func TestTapTroupeOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "tapTroupe", filepath.Join("..", "..", "games", "taptroupe"))
}

func TestTotemClimbOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "totemClimb", filepath.Join("..", "..", "games", "totemclimb"))
}

func TestKarateManOfficialActionsAreHandled(t *testing.T) {
	assertOfficialActionsHandled(t, "karateman", filepath.Join("..", "..", "games", "karateman"))
}

func TestAllRegisteredOfficialActionsAreHandled(t *testing.T) {
	const hsRoot = "/Users/xargin/Downloads/HeavenStudio-master"
	if _, err := os.Stat(hsRoot); err != nil {
		t.Skipf("Heaven Studio source tree not present: %v", err)
	}
	games, err := scanLoaders(hsRoot)
	if err != nil {
		t.Fatal(err)
	}
	regDirs := scanRegisterGameDirsForTest(t, filepath.Join("..", "..", "registry.go"))

	var missing []string
	for id, dir := range regDirs {
		game, ok := games[id]
		if !ok {
			t.Fatalf("registered game %s has no Heaven Studio loader", id)
		}
		cases := scanGoActionCases(t, filepath.Join("..", "..", "games", dir))
		for _, action := range game.Actions {
			if !cases[action] && !cases[id+"/"+action] && !engineHandlesOfficialAction(action) {
				missing = append(missing, id+"/"+action)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		t.Fatalf("missing official action handlers: %v", missing)
	}
}

func assertOfficialActionsHandled(t *testing.T, id, dir string) {
	t.Helper()
	const hsRoot = "/Users/xargin/Downloads/HeavenStudio-master"
	if _, err := os.Stat(hsRoot); err != nil {
		t.Skipf("Heaven Studio source tree not present: %v", err)
	}
	games, err := scanLoaders(hsRoot)
	if err != nil {
		t.Fatal(err)
	}
	game, ok := games[id]
	if !ok {
		t.Fatalf("%s loader not found", id)
	}
	cases := scanGoActionCases(t, dir)
	var missing []string
	for _, action := range game.Actions {
		if !cases[action] && !cases[id+"/"+action] && !engineHandlesOfficialAction(action) {
			missing = append(missing, action)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("missing %s actions: %v", id, missing)
	}
}

func scanRegisterGameDirsForTest(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	importRe := regexp.MustCompile(`"hsdemo/games/([^"]+)"`)
	aliasToDir := map[string]string{}
	for _, m := range importRe.FindAllSubmatch(raw, -1) {
		dir := string(m[1])
		aliasToDir[filepath.Base(dir)] = dir
	}
	registerRe := regexp.MustCompile(`engine\.Register\("([^"]+)",\s*([A-Za-z0-9_]+)\.New\)`)
	out := map[string]string{}
	for _, m := range registerRe.FindAllSubmatch(raw, -1) {
		id, alias := string(m[1]), string(m[2])
		dir := aliasToDir[alias]
		if dir == "" {
			t.Fatalf("register %s uses unknown import alias %s", id, alias)
		}
		out[id] = dir
	}
	return out
}

func scanGoActionCases(t *testing.T, dir string) map[string]bool {
	t.Helper()
	reCase := regexp.MustCompile(`case\s+([^:\n]+):`)
	reString := regexp.MustCompile(`"((?:\\.|[^"])*)"`)
	out := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range reCase.FindAllStringSubmatch(string(raw), -1) {
			for _, s := range reString.FindAllStringSubmatch(m[1], -1) {
				action := unescapeCSharpString(s[1])
				out[action] = true
				if _, rest, ok := strings.Cut(action, "/"); ok {
					out[rest] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func engineHandlesOfficialAction(action string) bool {
	return strings.HasPrefix(action, "countIn/")
}
