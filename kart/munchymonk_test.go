package kart

import (
	"math"
	"testing"
)

func TestMunchyMonkWristSlapReturnsToArmIdle(t *testing.T) {
	as, err := Load("../assets/munchyMonk", 44100)
	if err != nil {
		t.Fatal(err)
	}
	s := NewScene(as)
	const arms = "MonkHolder/ArmsHolder/Arms"

	s.PlayDefaultState(arms, 0, 1)
	s.Sample(0)
	idle0, ok := s.NodeWorld("MonkHolder/ArmsHolder/Arms/Arm/Lines")
	if !ok {
		t.Fatal("missing Arm/Lines node")
	}
	idle1, ok := s.NodeWorld("MonkHolder/ArmsHolder/Arms/Arm/Lines (1)")
	if !ok {
		t.Fatal("missing Arm/Lines (1) node")
	}

	s.PlayState(arms, "WristSlap", 1, 0.5)
	s.Sample(1.7)
	if got := s.Current(arms); got != "Arm/ArmIdle" {
		t.Fatalf("WristSlap did not return to ArmIdle clip, got %q", got)
	}
	assertSameWorld(t, "Arm/Lines", idle0, mustWorld(t, s, "MonkHolder/ArmsHolder/Arms/Arm/Lines"))
	assertSameWorld(t, "Arm/Lines (1)", idle1, mustWorld(t, s, "MonkHolder/ArmsHolder/Arms/Arm/Lines (1)"))
}

func TestMunchyMonkDumplingSmearAppearFadesOut(t *testing.T) {
	as, err := Load("../assets/munchyMonk", 44100)
	if err != nil {
		t.Fatal(err)
	}
	s := NewScene(as)
	const smear = "DumplingStuff/DumplingSmear1"
	idx := mustIndex(t, s, smear)

	s.Sample(0)
	if got := s.state[idx].color[3]; got != 0 {
		t.Fatalf("default smear alpha = %v, want 0", got)
	}

	s.PlayState(smear, "SmearAppear", 1, 1)
	s.Sample(1)
	if got := s.state[idx].color[3]; got != 1 {
		t.Fatalf("SmearAppear first frame alpha = %v, want 1", got)
	}

	s.Sample(1.1)
	if got := s.state[idx].color[3]; got != 0 {
		t.Fatalf("SmearAppear should fade back to alpha 0, got %v", got)
	}
	if got := s.Current(smear); got != "Smears/SmearIdle" {
		t.Fatalf("SmearAppear did not return to idle clip, got %q", got)
	}
}

func mustIndex(t *testing.T, s *SceneInst, path string) int {
	t.Helper()
	idx, ok := s.Index(path)
	if !ok {
		t.Fatalf("missing node %s", path)
	}
	return idx
}

func mustWorld(t *testing.T, s *SceneInst, path string) Aff {
	t.Helper()
	w, ok := s.NodeWorld(path)
	if !ok {
		t.Fatalf("missing node %s", path)
	}
	return w
}

func assertSameWorld(t *testing.T, label string, want, got Aff) {
	t.Helper()
	const eps = 1e-9
	if math.Abs(want.A-got.A) > eps || math.Abs(want.B-got.B) > eps ||
		math.Abs(want.C-got.C) > eps || math.Abs(want.D-got.D) > eps ||
		math.Abs(want.Tx-got.Tx) > eps || math.Abs(want.Ty-got.Ty) > eps {
		t.Fatalf("%s world changed after WristSlap: want %+v got %+v", label, want, got)
	}
}
