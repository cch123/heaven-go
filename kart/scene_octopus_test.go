package kart

import "testing"

func TestSceneNodeResolvesOctopusCorkAliases(t *testing.T) {
	as, err := Load("../assets/octopusMachine", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	s := NewScene(as)
	p := &scenePlayer{rootPath: "Octopodes/Octopus1"}
	for curvePath, want := range map[string]string{
		"Body/Head/Cork": "Octopodes/Octopus1/Body/Head/Mouth/Cork",
		"CorkString":     "Octopodes/Octopus1/Body/Head/CorkString",
	} {
		idx, ok := s.node(p, curvePath)
		if !ok {
			t.Fatalf("curve path %s did not resolve", curvePath)
		}
		if got := as.Rig.Nodes[idx].Path; got != want {
			t.Fatalf("curve path %s resolved to %s, want %s", curvePath, got, want)
		}
	}
}
