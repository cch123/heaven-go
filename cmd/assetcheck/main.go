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
	meshBindings  int
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

	var meshes kmdata.MeshData
	if err := readOptionalJSON(filepath.Join(dir, "meshes.json"), &meshes); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("read meshes.json: %v", err))
	}
	r.meshBindings = len(meshes.Bindings)
	for _, b := range meshes.Bindings {
		if !nodes[b.Path] {
			r.errs = append(r.errs, fmt.Sprintf("mesh binding %q points at missing scene path", b.Path))
		}
		if b.Mesh.FileID == 0 && b.Mesh.GUID == "" {
			r.errs = append(r.errs, fmt.Sprintf("mesh binding %q has empty mesh reference", b.Path))
		}
		for _, mat := range b.Materials {
			if mat.GUID == "" || mat.GUID == "0" {
				continue
			}
			if _, ok := meshes.Materials[mat.GUID]; !ok {
				r.errs = append(r.errs, fmt.Sprintf("mesh binding %q material guid %s missing from materials table", b.Path, mat.GUID))
			}
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
				if tr.Dst == "" {
					// Empty destinations are Unity Exit transitions (m_IsExit=1).
					// They leave the current state machine layer instead of naming
					// another state, so there is no controller state to resolve.
					continue
				}
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
			if skipAnimatorStateAudit(game, root, ctrlName, stateName, st.Clip) {
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
		if root == "" {
			full = rel
		} else {
			full = root + "/" + rel
		}
	}
	if nodes[full] {
		return full
	}
	if game == "sumoBrothers" {
		switch {
		case root == "backgroundChanges/bgStatic" && rel == "mask" && nodes["backgroundChanges/bgMove/mask"]:
			return "backgroundChanges/bgMove/mask"
		case (root == "sumoBrotherP" || root == "sumoBrotherG") && rel == "head/head" && nodes[root+"/head/headdy"]:
			return root + "/head/headdy"
		case root == "sumoBrotherG" && rel == "effects/stompEffect2" && nodes["sumoBrotherP/effects/stompEffect2"]:
			return "sumoBrotherP/effects/stompEffect2"
		}
	}
	if game == "showtime" && root == "WaterHolder" && rel == "Water (1)" {
		// Showtime's Move.anim still contains a stale binding to Water (1), but
		// the shipped prefab has only Water under WaterHolder. Unity therefore
		// ignores this curve; returning the root marks it as intentionally audited
		// rather than a missing extracted node.
		return root
	}
	if game == "shootEmUp" && root == "prefabs/trajectory/sprite" && rel == "sprite" {
		// The trajectory prefab has an unused child Animator carrying the same
		// controller as the prefab root. Its clips target a child named "sprite",
		// which does not exist below that child Animator; ShootEmUp's C# only
		// plays trajectory.GetComponent<Animator>() on the prefab root.
		return root
	}
	if game == "loveRap" {
		switch {
		case root == "GirlHolder/Legs/Body" && rel == "animation_44.017":
			// HB.anim has a leftover empty-sprite binding to an importer-generated
			// child that is not present in the shipped prefab. Unity ignores this
			// missing binding; the visible girl body curves are bound to real limbs.
			return root
		case strings.HasSuffix(root, "/OtherRapper/rap_body") && rel == "animation_01.000":
			// The male rapper S.anim files carry the same stale empty-sprite
			// binding on both left and right rappers. The prefab has no matching
			// child, so there is no runtime object to drive in Unity either.
			return root
		}
	}
	if game == "rhythmTestGBA" && root == "Countdown/BG/Left" {
		// The Left sprite carries a duplicate BG Animator component, but the BG
		// clips are authored against the parent Countdown/BG children. Gameplay
		// drives the parent animator; resolving here keeps the duplicate Unity
		// component audited without pretending it owns sibling curves.
		alt := "Countdown/BG"
		if rel != "" {
			alt += "/" + rel
		}
		if nodes[alt] {
			return alt
		}
	}
	if game == "balloonHunter" && (root == "PopLeft" || root == "PopRight") && rel == "PopEffectR" {
		// PopEffect.anim is shared by left/middle/right burst prefabs. Only the
		// middle prefab has the second PopEffectR sprite branch; left/right each
		// keep a single PopEffect child, with the remaining burst handled by the
		// particle effect nodes outside this Animator root.
		return root
	}
	if game == "clappyTrio" && root == "Lion" && strings.HasPrefix(rel, "Hands/ClapEffect_") {
		// Clap.anim contains stale flat paths from before the clap sprites were
		// grouped under Hands/ClapEffect. The same clip still contains live
		// Hands/ClapEffect/ClapEffect_* curves, which remain fully audited.
		return root
	}
	if game == "forkLifter" && (rel == "Fork_Lifter_Gameplay" || rel == "Fork_Lifter_Gameplay (1)") {
		// Fork Lifter keeps importer-era empty sprite bindings on helper nodes
		// that are not present in the shipped prefab. The food/fork visuals are
		// driven by Sprite, Sprite/follow, and the player fork stack instead.
		return root
	}
	if game == "kitties" && rel == "Effects/GameObject" {
		// FaceClapFail has an old helper GameObject path that is absent from the
		// Kitties prefab. The visible fail face sprite is bound to Kitty and the
		// hand/effect children remain resolved normally.
		return root
	}
	if game == "ninjaBodyguard" && rel == "NinjaCutL.001" {
		// The player stay/hold/apology clips retain empty sprite bindings to a
		// generated NinjaCutL.001 object that is not in the prefab. Actual cut
		// slashes are separate states and the hand-written HitParticle effect.
		return root
	}
	if game == "spaceSoccer" && (rel == "Square (3)" || rel == "lifting.NCER_21") {
		// Space Soccer's kicker clips have legacy empty sprite-swap bindings from
		// the source CellAnim import. The playable kicker uses Holder/* nodes and
		// toeFX/platform sprites, all of which are still audited.
		return root
	}
	if game == "tapTrial" {
		switch {
		case strings.HasSuffix(rel, "/star_0") || strings.HasSuffix(rel, "/star_1"):
			// Tap Trial's jump-tap clips still animate two star child objects under
			// tap_effect, but the shipped prefab only keeps the wave child. The Go
			// port recreates those stars with the hand-written burst() particles;
			// resolve to the parent effect so the prefab anchor remains audited.
			parent := rel[:strings.LastIndex(rel, "/")]
			fullParent := root + "/" + parent
			if nodes[fullParent] {
				return fullParent
			}
		case rel == "root_body/ref (1)":
			// Player tap clips retain an importer-era empty sprite binding to a
			// missing reference helper. It has no transform/color curves and no
			// visible sprite, so Unity has no runtime object to drive.
			return root
		case root == "MonkeyL" && rel == "root_body/monkey_head/tongue bg":
			// The shared monkey clips bind a tongue-bg helper that only exists on
			// MonkeyR in the prefab. MonkeyL's visible head/mouth/body bindings are
			// still checked normally.
			return root
		}
	}
	if game == "octopusMachine" {
		switch rel {
		case "CorkString":
			if nodes[root+"/Body/Head/CorkString"] {
				return root + "/Body/Head/CorkString"
			}
		case "Body/Head/Cork":
			if nodes[root+"/Body/Head/Mouth/Cork"] {
				return root + "/Body/Head/Mouth/Cork"
			}
		}
	}
	if strings.Contains(full, "/Upper/Head") {
		// Some nested prefabs keep parent clips authored against Upper/Head,
		// while extraction inserts a child Animator wrapper. Mirror SceneInst's
		// runtime fallback so the audit checks the real driven head nodes.
		alt := strings.Replace(full, "/Upper/Head", "/Upper/HeadAnim/Upper/Head", 1)
		if nodes[alt] {
			return alt
		}
	}
	if game != "nightWalkAgb" || !strings.HasPrefix(root, "JumpPlatform/rollPlatform/RodHolder") {
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

func skipAnimatorStateAudit(game, root, ctrlName, stateName, clip string) bool {
	if game == "cheerReaders" && root == "GirlHolder" && ctrlName == "BaseAnim" {
		// GirlHolder is the stripped authoring template left in the scene. The
		// playable cheerleaders are the pepSquad prefab instances referenced by
		// firstRow/secondRow/thirdRow/player, and those contain Body (1) plus the
		// OpenBook pages that the BaseAnim states target.
		return true
	}
	if game == "flipperFlop" && ctrlName == "Flipper" && stateName == "Test" && clip == "Animations/Test" {
		// The shipped Flipper controller includes an editor-only Test state
		// targeting a pre-refactor Face path. FlipperFlop.cs never plays it, and
		// all real face states are owned by the FaceFlipper controller.
		return true
	}
	if game == "lumbearjack" && root == "CatHolder" && ctrlName == "Cat" && (stateName == "CatDance" || stateName == "CatGrab") {
		// The extracted scene has a bare CatHolder helper that lacks the Arms and
		// ObjectHolder branches used by the full cat prefab. Gameplay drives
		// Cat/CatHolder, CatLeft/CatHolder, and BgCats/*/CatHolder instead.
		return true
	}
	if game == "rhythmTweezers" && ctrlName == "HairHolder" {
		if strings.HasSuffix(root, "/HairHolder") && (stateName == "LongAppear" || stateName == "LoopPull" || stateName == "LoopPullReverse") {
			// Short-hair instances use HairHolder and only play SmallAppear. The
			// long-hair states target the sibling LongHairHolder prefab and are
			// driven through the long-hair template instead.
			return true
		}
		if strings.HasSuffix(root, "/LongHairHolder") && stateName == "SmallAppear" {
			// Long-hair instances use LongHairHolder and never play the short-hair
			// SmallAppear state; those curves target HairHolder/Hair.
			return true
		}
	}
	if game == "tapTrial" && ctrlName == "MonkeyTapTrial" {
		if stateName == "JumpPrepare" || stateName == "JumpTap" {
			// These two state names are shared with the player controller, but the
			// MonkeyTapTrial copies point at the girl's ready/tap clips. TapTrial's
			// C# only plays them on Player; monkeys use Jump and Jumpactualtap for
			// the jump-tap sequence.
			return true
		}
	}
	return false
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
	fmt.Printf("nodes=%d roles=%d meshBindings=%d controllers=%d animatorRoots=%d checkedAnimationPaths=%d\n",
		r.nodes, r.roles, r.meshBindings, r.controllers, r.animatorRoots, r.checkedPaths)
	if len(r.errs) == 0 {
		return
	}
	fmt.Printf("errors=%d\n", len(r.errs))
	for _, err := range r.errs {
		fmt.Println("- " + strings.TrimSpace(err))
	}
}
