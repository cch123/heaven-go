package main

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

func TestVerifyRegistersEveryPlayableGame(t *testing.T) {
	root := scanRegisterIDsForTest(t, "../../registry.go")
	verify := scanRegisterIDsForTest(t, "../verify/main.go")
	var missing []string
	for id := range root {
		if !verify[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("cmd/verify registration is missing %d game(s): %v", len(missing), missing)
	}
}

func scanRegisterIDsForTest(t *testing.T, path string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`engine\.Register\("([^"]+)"`)
	out := map[string]bool{}
	for _, m := range re.FindAllSubmatch(raw, -1) {
		out[string(m[1])] = true
	}
	return out
}
