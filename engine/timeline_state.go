package engine

import (
	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/conductor"
	"hsdemo/riq"
)

func (a *App) resetLoadedRiq(r *riq.Riq, player *audio.Player, music *pitchPCMReader) {
	a.unloadLoadedRiq()

	a.r, a.bm = r, r.Beatmap
	a.player = player
	a.music = music
	a.cond = conductor.New(r.Beatmap, player)
	a.modules = map[string]Module{}
}

func (a *App) unloadLoadedRiq() {
	a.closeChartPlayer()

	a.r = nil
	a.bm = nil
	a.cond = nil
	a.clearTimelineState()
	a.resetRunState()
	a.loadErr = ""
}

func (a *App) closeChartPlayer() {
	if a.player == nil {
		return
	}
	a.player.Close()
	a.player = nil
	a.music = nil
}

func (a *App) clearTimelineState() {
	// These slices and effects are rebuilt from the loaded RIQ. Score/result
	// counters stay in resetRunState because restart keeps the same timeline.
	a.modules = nil
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.flashes = nil
	a.camEvts = nil
	a.musicFades = nil
	a.viewScales = nil
	a.viewBuf = nil
	a.unported = nil
	a.actIdx = 0
	a.starBeat, a.endBeat = -1, 0
	a.hasEndBeat = false
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
}
