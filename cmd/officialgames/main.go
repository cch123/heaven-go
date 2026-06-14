// officialgames prints the porting matrix between Heaven Studio's loader list,
// the official Pack-In levels, and the local Go implementation status.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"hsdemo/riq"
)

type gameInfo struct {
	ID       string
	Name     string
	Source   string
	Actions  []string
	PackIn   []string
	Registry bool
	Extract  bool
}

func main() {
	hsRoot := flag.String("hs-root", "/Users/xargin/Downloads/HeavenStudio-master", "Heaven Studio Unity project root")
	levelsDir := flag.String("levels", "/Users/xargin/Downloads/Heaven Studio.app/Contents/Resources/Data/StreamingAssets/Library Pack-In/Heaven Studio Pack-In Levels", "official Pack-In levels directory")
	repo := flag.String("repo", ".", "heaven-go repository root")
	flag.Parse()

	games, err := scanLoaders(*hsRoot)
	if err != nil {
		log.Fatal(err)
	}
	registered := scanStringSet(filepath.Join(*repo, "main.go"), regexp.MustCompile(`engine\.Register\("([^"]+)"`))
	extractSpecs := scanExtractSpecs(filepath.Join(*repo, "cmd", "extract"))
	packIn, err := scanPackIn(*levelsDir)
	if err != nil {
		log.Printf("warn: Pack-In levels skipped: %v", err)
	}

	for id, g := range games {
		g.Registry = registered[id]
		g.Extract = extractSpecs[id]
		g.PackIn = packIn[id]
		sort.Strings(g.PackIn)
		games[id] = g
	}

	printMarkdown(games)
}

func scanLoaders(root string) (map[string]gameInfo, error) {
	dir := filepath.Join(root, "Assets", "Scripts", "Games")
	out := map[string]gameInfo{}
	reGame := regexp.MustCompile(`return\s+new\s+Minigame\s*\(\s*"([^"]+)"\s*,\s*"((?:\\.|[^"])*)"`)
	reAction := regexp.MustCompile(`new\s+GameAction\s*\(\s*"([^"]+)"`)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".cs" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := stripLineComments(string(raw))
		m := reGame.FindStringSubmatch(text)
		if m == nil {
			return nil
		}
		id := m[1]
		rel, _ := filepath.Rel(dir, path)
		seenActions := map[string]bool{}
		var actions []string
		for _, am := range reAction.FindAllStringSubmatch(text, -1) {
			if !seenActions[am[1]] {
				seenActions[am[1]] = true
				actions = append(actions, am[1])
			}
		}
		sort.Strings(actions)
		out[id] = gameInfo{
			ID:      id,
			Name:    unescapeCSharpString(m[2]),
			Source:  filepath.ToSlash(rel),
			Actions: actions,
		}
		return nil
	})
	return out, err
}

func stripLineComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func unescapeCSharpString(s string) string {
	repl := strings.NewReplacer(`\n`, " ", `\"`, `"`, `\\`, `\`)
	return strings.Join(strings.Fields(repl.Replace(s)), " ")
}

func scanStringSet(path string, re *regexp.Regexp) map[string]bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]] = true
	}
	return out
}

func scanExtractSpecs(dir string) map[string]bool {
	out := map[string]bool{}
	re := regexp.MustCompile(`(?m)^\s*"([A-Za-z0-9]+)"\s*:\s*(?:\{|basicOfficialSceneSpec\()`)
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			out[m[1]] = true
		}
		return nil
	})
	return out
}

func scanPackIn(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, ent := range entries {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".riq") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		r, err := riq.Load(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		used := map[string]bool{}
		for i := range r.Beatmap.Entities {
			e := &r.Beatmap.Entities[i]
			if g, ok := strings.CutPrefix(e.Datamodel, "gameManager/switchGame/"); ok {
				used[g] = true
				continue
			}
			switch e.Game() {
			case "gameManager", "global", "vfx", "ppe", "countIn":
			default:
				used[e.Game()] = true
			}
		}
		for id := range used {
			out[id] = append(out[id], ent.Name())
		}
	}
	return out, nil
}

func printMarkdown(games map[string]gameInfo) {
	ids := make([]string, 0, len(games))
	var registered, extract, packIn int
	for id, g := range games {
		ids = append(ids, id)
		if g.Registry {
			registered++
		}
		if g.Extract {
			extract++
		}
		if len(g.PackIn) > 0 {
			packIn++
		}
	}
	sort.Strings(ids)

	fmt.Println("# Official Game Port Matrix")
	fmt.Println()
	fmt.Printf("- Heaven Studio loaders: %d\n", len(ids))
	fmt.Printf("- Used by Pack-In levels: %d\n", packIn)
	fmt.Printf("- Registered playable modules: %d\n", registered)
	fmt.Printf("- Basic extraction specs: %d\n", extract)
	fmt.Println()
	fmt.Println("| id | name | Pack-In | registered | extract | actions | source |")
	fmt.Println("|---|---|---:|:---:|:---:|---:|---|")
	for _, id := range ids {
		g := games[id]
		fmt.Printf("| `%s` | %s | %d | %s | %s | %d | `%s` |\n",
			escapeMD(g.ID), escapeMD(g.Name), len(g.PackIn),
			yesNo(g.Registry), yesNo(g.Extract), len(g.Actions), escapeMD(g.Source))
	}
}

func yesNo(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}
