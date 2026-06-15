package engine

import (
	"fmt"
	"sort"
	"strings"
)

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
