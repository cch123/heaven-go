package engine

type timingHit struct {
	y      float64 // normalized visual position on the TimingAccuracy bar, after prefab segment scaling.
	signed float64
	rating Judgment
	t      float64
}
