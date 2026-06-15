package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/conductor"
	"hsdemo/riq"
)

func (a *App) resetLoadedRiq(r *riq.Riq, player *audio.Player) {
	if a.player != nil {
		a.player.Close()
	}

	a.r, a.bm = r, r.Beatmap
	a.player = player
	a.cond = conductor.New(r.Beatmap, player)
	a.modules = map[string]Module{}
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.scores = nil
	a.flashes = nil
	a.camEvts = nil
	a.musicFades = nil
	a.viewScales = nil
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
	a.unported = nil
	a.starBeat, a.endBeat = -1, 0
	a.resetRunState()
}

func (a *App) collectUsedGames() map[string]bool {
	used := map[string]bool{}
	for i := range a.bm.Entities {
		e := &a.bm.Entities[i]
		if g, ok := strings.CutPrefix(e.Datamodel, "gameManager/switchGame/"); ok {
			a.switches = append(a.switches, gameSwitch{e.Beat, g})
			used[g] = true
			continue
		}
		switch e.Game() {
		case "gameManager", "vfx", "countIn", "global", "ppe":
			continue // ppe is handled by the engine and does not own a module.
		}
		used[e.Game()] = true
	}
	sort.Slice(a.switches, func(i, j int) bool { return a.switches[i].beat < a.switches[j].beat })
	return used
}

func (a *App) loadModulesFor(used map[string]bool) error {
	for id := range used {
		var m Module
		if f, ok := registry[id]; ok {
			m = f()
		} else {
			m = newPlaceholder(id)
			a.unported = append(a.unported, id)
		}
		ctx := &Ctx{App: a, module: m}
		if err := m.Load(ctx); err != nil {
			return fmt.Errorf("load %s assets: %w (run go run ./cmd/extract -game %s first)", id, err, id)
		}
		a.modules[id] = m
	}
	sort.Strings(a.unported)
	return nil
}
