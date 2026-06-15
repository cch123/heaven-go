package engine

import (
	"log"
	"sort"

	"hsdemo/riq"
)

// ---------- 谱面装载 ----------

func (a *App) loadRiq(r *riq.Riq) error {
	player, err := decodeMusicPlayer(r)
	if err != nil {
		return err
	}
	a.resetLoadedRiq(r, player)
	if err := a.loadModulesFor(a.collectUsedGames()); err != nil {
		return err
	}
	a.dispatchBeatmapEvents()
	if err := a.finishLoadedTimeline(); err != nil {
		return err
	}

	log.Printf("riq loaded: %q by %q, %d entities, games=%v unported=%v",
		a.bm.Prop("remixtitle"), a.bm.Prop("remixauthor"), len(a.bm.Entities), keys(a.modules), a.unported)
	return nil
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortActions(as []beatAction) {
	sort.SliceStable(as, func(i, j int) bool { return as[i].beat < as[j].beat })
}
