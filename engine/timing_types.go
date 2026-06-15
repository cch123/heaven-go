package engine

type timingDisplayState struct {
	// Keep the TimingAccuracy display's internal smoothing here instead of on
	// App, so input judgement only reports facts and the HUD owns presentation.
	arrow, target float64
	hits          []timingHit
}

type timingHit struct {
	y      float64 // normalized visual position on the TimingAccuracy bar, after prefab segment scaling.
	signed float64
	rating Judgment
	t      float64
}
