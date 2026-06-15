package engine

type scoreRuntimeState struct {
	scores []resultScoreInput

	starBeat float64
	starGot  bool

	aces, justs, ngs, misses, whiffs int
}
