package audittest

import (
	"path"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func LoadAssets(t *testing.T, game string) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", game), engine.SampleRate)
	if err != nil {
		t.Skipf("%s assets not extracted: %v", game, err)
	}
	return as
}

func RequireRoles(t *testing.T, as *kart.Assets, roles map[string]string) {
	t.Helper()
	for role, want := range roles {
		got := as.Roles[role]
		if got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
		if _, ok := as.NodeIndex(got); !ok {
			t.Fatalf("role %s points to missing node %q", role, got)
		}
	}
}

func RequireNodes(t *testing.T, as *kart.Assets, nodes ...string) {
	t.Helper()
	for _, node := range nodes {
		if _, ok := as.NodeIndex(node); !ok {
			t.Fatalf("missing node %q", node)
		}
	}
}

func RequireSounds(t *testing.T, as *kart.Assets, sounds ...string) {
	t.Helper()
	for _, sound := range sounds {
		if as.Sounds[sound] == nil {
			t.Fatalf("missing sound %q", sound)
		}
	}
}

func RequireSequences(t *testing.T, as *kart.Assets, seqs ...string) {
	t.Helper()
	for _, seq := range seqs {
		if len(as.Extra.Sequences[seq]) == 0 {
			t.Fatalf("missing sound sequence %q", seq)
		}
		for _, clip := range as.Extra.Sequences[seq] {
			if as.Sounds[clip.Clip] == nil {
				t.Fatalf("sequence %q references missing sound %q", seq, clip.Clip)
			}
		}
	}
}

func RequireControllerStates(t *testing.T, as *kart.Assets, controller string, states ...string) {
	t.Helper()
	ctrl, ok := as.Controllers[controller]
	if !ok {
		t.Fatalf("missing controller %q", controller)
	}
	if ctrl.Default == "" {
		t.Fatalf("controller %q has empty default state", controller)
	}
	for _, state := range states {
		st, ok := ctrl.States[state]
		if !ok {
			t.Fatalf("controller %q missing state %q", controller, state)
		}
		if st.Clip != "" && as.Anims[st.Clip] == nil {
			t.Fatalf("controller %q state %q references missing clip %q", controller, state, st.Clip)
		}
	}
}

func RequireClips(t *testing.T, as *kart.Assets, clips ...string) {
	t.Helper()
	for _, clip := range clips {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %q", clip)
		}
	}
}

func RequireAnimatorPaths(t *testing.T, as *kart.Assets) {
	t.Helper()
	for root, ctrlName := range as.Animators {
		if _, ok := as.NodeIndex(root); !ok {
			t.Fatalf("animator root %q for controller %q is missing", root, ctrlName)
		}
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("animator root %q references missing controller %q", root, ctrlName)
		}
		for state, st := range ctrl.States {
			if st.Clip != "" {
				RequireClipPaths(t, as, root, st.Clip, ctrlName+"/"+state)
			}
		}
	}
}

func RequireClipPaths(t *testing.T, as *kart.Assets, root, clip, label string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("%s missing clip %q", label, clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s clip %q curve path %q resolved to missing node %q", label, clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}
