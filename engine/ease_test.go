package engine

import (
	"math"
	"testing"
)

func TestEaseSpringMatchesUnityFormula(t *testing.T) {
	got := Ease(32, 10, 20, 0.5)
	want := 10 + 10*((math.Sin(0.5*math.Pi*(0.2+2.5*0.5*0.5*0.5))*math.Pow(1-0.5, 2.2)+0.5)*(1+1.2*(1-0.5)))
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("Spring midpoint = %.15f, want %.15f", got, want)
	}
	if math.Abs(got-15) < 1e-6 {
		t.Fatalf("Spring midpoint unexpectedly fell back to linear: %.15f", got)
	}
}

func TestEaseCoversHeavenStudioEnumRange(t *testing.T) {
	easeWarned = map[int]bool{}
	for kind := 0; kind <= 43; kind++ {
		_ = Ease(kind, -2, 5, 0.37)
	}
	if len(easeWarned) != 0 {
		t.Fatalf("official easing values fell back to linear: %#v", easeWarned)
	}
}
