package engine

import (
	"fmt"
	"sort"
)

func (a *App) finishLoadedTimeline() error {
	a.ensureEndBeat()
	for _, m := range a.modules {
		m.Ready()
	}
	sortActions(a.actions)
	sort.Slice(a.inputs, func(i, j int) bool { return a.inputs[i].Beat < a.inputs[j].Beat })
	a.fx.sortAll()
	if a.fx.active() {
		if err := a.fx.ensure(); err != nil {
			return fmt.Errorf("compile ppe shader: %w", err)
		}
	}

	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	return nil
}

func (a *App) ensureEndBeat() {
	if a.hasEndBeat || a.bm == nil || len(a.bm.Entities) == 0 {
		return
	}
	last := a.bm.Entities[len(a.bm.Entities)-1]
	a.endBeat = last.Beat + last.Length + 4
}
