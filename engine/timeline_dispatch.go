package engine

import (
	"strings"

	"hsdemo/riq"
)

func (a *App) dispatchBeatmapEvents() {
	for i := range a.bm.Entities {
		a.dispatchBeatmapEvent(&a.bm.Entities[i])
	}
}

func (a *App) dispatchBeatmapEvent(e *riq.Entity) {
	switch {
	case strings.HasPrefix(e.Datamodel, "gameManager/switchGame/"):
		// collectUsedGames already records switch events before modules load.
	case e.Datamodel == "gameManager/end":
		// Heaven Studio treats the first end marker as the playable remix end.
		// Some v1 community levels keep edited-out events after that marker; a
		// later stray end must not push result timing into those leftovers.
		if !a.hasEndBeat || e.Beat < a.endBeat {
			a.endBeat = e.Beat
			a.hasEndBeat = true
		}
	case e.Datamodel == "gameManager/skill star":
		a.starBeat = e.Beat + e.Length
	case e.Datamodel == "gameManager/toggle inputs":
		on := boolParam(e, "toggle")
		b := e.Beat
		a.at(b, func() { a.inputOn = on })
	case e.Datamodel == "vfx/flash":
		a.flashes = append(a.flashes, flashEvt{
			beat: e.Beat, length: e.Length,
			c0: colorParam(e, "colorA"), c1: colorParam(e, "colorB"),
		})
	case e.Datamodel == "vfx/scale view":
		a.viewScales = append(a.viewScales, viewScaleEvt{
			beat: e.Beat, length: e.Length,
			x: e.Float("valA", 1), y: e.Float("valB", 1),
			ease: int(e.Float("ease", 0)),
			axis: int(e.Float("axis", 0)),
		})
	case e.Datamodel == "vfx/move camera" || e.Datamodel == "gameManager/move camera":
		a.camEvts = append(a.camEvts, camEvt{
			beat: e.Beat, length: e.Length,
			target: [3]float64{e.Float("valA", 0), e.Float("valB", 0), -e.Float("valC", 10)},
			ease:   int(e.Float("ease", 0)),
			axis:   int(e.Float("axis", 0)),
		})
	case isCountInEvent(e.Datamodel):
		a.scheduleCountIn(e.Datamodel, e.Beat, e.Length, e.Data)
	case e.Datamodel == "vfx/filter":
		a.flt.add(e)
	case e.Datamodel == "vfx/display textbox":
		a.tbx.add(e)
	case e.Game() == "ppe":
		a.fx.add(e)
	case e.Game() == "gameManager" || e.Game() == "vfx" || e.Game() == "global":
		// Unsupported global events are intentionally ignored until ported.
	default:
		if m, ok := a.modules[e.Game()]; ok {
			m.OnEvent(e)
		}
	}
}
