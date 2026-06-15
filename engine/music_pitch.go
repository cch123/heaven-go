package engine

// setMinigamePitch mirrors Conductor.SetMinigamePitch: chart music changes
// pitch immediately, and the conductor's beat clock advances at the same rate.
func (a *App) setMinigamePitch(pitch float64) {
	if a.music != nil {
		a.music.SetPitch(pitch)
	}
	if a.cond != nil {
		a.cond.SetMinigamePitch(pitch)
	}
}
