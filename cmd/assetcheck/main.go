// assetcheck audits extracted Unity scene assets without starting Ebitengine.
//
// It intentionally focuses on checks that every scene-based minigame port needs
// before gameplay code can be trusted: script roles must point at scene nodes,
// AnimatorController states must resolve their clips, and every animated path
// must exist below the animator root that plays the clip.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hsdemo/kmdata"
)

type report struct {
	game          string
	nodes         int
	roles         int
	animatorRoots int
	controllers   int
	checkedPaths  int
	errs          []string
}

func main() {
	assetsRoot := flag.String("assets", "assets", "extracted assets root")
	game := flag.String("game", "", "single game id to audit")
	flag.Parse()

	if *game == "" {
		fmt.Fprintln(os.Stderr, "assetcheck: -game is required")
		os.Exit(2)
	}
	r := auditGame(filepath.Join(*assetsRoot, *game), *game)
	printReport(r)
	if len(r.errs) > 0 {
		os.Exit(1)
	}
}

func auditGame(dir, game string) report {
	r := report{game: game}

	var rig kmdata.Rig
	if err := readJSON(filepath.Join(dir, "scene.json"), &rig); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read scene.json: %v", err))
		return r
	}
	r.nodes = len(rig.Nodes)
	nodes := map[string]bool{}
	for _, n := range rig.Nodes {
		nodes[n.Path] = true
	}

	var roles kmdata.Roles
	if err := readOptionalJSON(filepath.Join(dir, "roles.json"), &roles); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read roles.json: %v", err))
	}
	r.roles = len(roles)
	for field, path := range roles {
		if !nodes[path] {
			r.errs = append(r.errs, fmt.Sprintf("role %s points at missing path %q", field, path))
		}
	}

	var anims map[string]*kmdata.Anim
	if err := readJSON(filepath.Join(dir, "anims.json"), &anims); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read anims.json: %v", err))
		return r
	}

	var controllers map[string]kmdata.Controller
	if err := readOptionalJSON(filepath.Join(dir, "controllers.json"), &controllers); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read controllers.json: %v", err))
	}
	r.controllers = len(controllers)

	var animators kmdata.Animators
	if err := readOptionalJSON(filepath.Join(dir, "animators.json"), &animators); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read animators.json: %v", err))
	}
	r.animatorRoots = len(animators)

	for name, ctrl := range controllers {
		if ctrl.Default != "" {
			if _, ok := ctrl.States[ctrl.Default]; !ok {
				r.errs = append(r.errs, fmt.Sprintf("controller %s default state %q missing", name, ctrl.Default))
			}
		}
		for stateName, st := range ctrl.States {
			if st.Clip != "" && anims[st.Clip] == nil {
				r.errs = append(r.errs, fmt.Sprintf("controller %s state %s clip %q missing", name, stateName, st.Clip))
			}
			for _, tr := range st.Transitions {
				if _, ok := ctrl.States[tr.Dst]; !ok {
					r.errs = append(r.errs, fmt.Sprintf("controller %s state %s transition dst %q missing", name, stateName, tr.Dst))
				}
			}
		}
	}

	for root, ctrlName := range animators {
		if !nodes[root] {
			r.errs = append(r.errs, fmt.Sprintf("animator root %q missing from scene", root))
			continue
		}
		ctrl, ok := controllers[ctrlName]
		if !ok {
			r.errs = append(r.errs, fmt.Sprintf("animator root %q controller %q missing", root, ctrlName))
			continue
		}
		for stateName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			clip := anims[st.Clip]
			if clip == nil {
				continue
			}
			for _, rel := range animatedPaths(clip) {
				full := resolveAnimPath(game, root, rel, nodes)
				r.checkedPaths++
				if !nodes[full] {
					r.errs = append(r.errs, fmt.Sprintf(
						"animator %q controller %s state %s clip %s path %q resolves to missing %q",
						root, ctrlName, stateName, st.Clip, rel, full,
					))
				}
			}
		}
	}
	sort.Strings(r.errs)
	return r
}

func resolveAnimPath(game, root, rel string, nodes map[string]bool) string {
	full := root
	if rel != "" {
		full = root + "/" + rel
	}
	if nodes[full] || game != "nightWalkAgb" || !strings.HasPrefix(root, "JumpPlatform/rollPlatform/RodHolder") {
		return full
	}
	// Night Walk GBA reuses JumpPlatform.controller on the roll RodHolder.
	// Several clips bind sibling paths under JumpPlatform/rollPlatform while
	// Rod-only curves still resolve below RodHolder, so try the prefab root only
	// after the normal Animator-root resolution fails.
	alt := "JumpPlatform"
	if rel != "" {
		alt += "/" + rel
	}
	if nodes[alt] {
		return alt
	}
	return full
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func readOptionalJSON(path string, v any) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return readJSON(path, v)
}

func animatedPaths(a *kmdata.Anim) []string {
	seen := map[string]bool{}
	add := func(path string) {
		seen[path] = true
	}
	for p := range a.Pos {
		add(p)
	}
	for p := range a.Scale {
		add(p)
	}
	for p := range a.Euler {
		add(p)
	}
	for p := range a.Sprites {
		add(p)
	}
	for p := range a.Floats {
		add(p)
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func printReport(r report) {
	status := "ok"
	if len(r.errs) > 0 {
		status = "failed"
	}
	fmt.Printf("%s: %s\n", r.game, status)
	fmt.Printf("nodes=%d roles=%d controllers=%d animatorRoots=%d checkedAnimationPaths=%d\n",
		r.nodes, r.roles, r.controllers, r.animatorRoots, r.checkedPaths)
	if len(r.errs) == 0 {
		return
	}
	fmt.Printf("errors=%d\n", len(r.errs))
	for _, err := range r.errs {
		fmt.Println("- " + strings.TrimSpace(err))
	}
}
