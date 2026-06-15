package main

import (
	"os"
	"path/filepath"
	"reflect"
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
