package engine

import "sort"

// MusicFadeAt returns the minigame-local music volume multiplier. It is
// separate from riq__VolumeChange so games like Tunnel can duck the song
// without rewriting the chart's authored volume events.
func (a *App) MusicFadeAt(beat float64) float64 {
	vol := 1.0
	for _, e := range a.musicFades {
		if beat < e.beat {
			break
		}
		if e.length > 0 && beat < e.beat+e.length {
			u := (beat - e.beat) / e.length
			return e.from + (e.to-e.from)*u
		}
		vol = e.to
	}
	return vol
}

func (a *App) fadeMusicVolume(beat, length, target float64) {
	from := a.MusicFadeAt(beat)
	ev := musicFadeEvt{beat: beat, length: length, from: from, to: target}
	i := sort.Search(len(a.musicFades), func(i int) bool { return a.musicFades[i].beat > beat })
	a.musicFades = append(a.musicFades, musicFadeEvt{})
	copy(a.musicFades[i+1:], a.musicFades[i:])
	a.musicFades[i] = ev
}
