package engine

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type menuRecord struct {
	Favorite bool   `json:"favorite,omitempty"`
	Rank     string `json:"rank,omitempty"`
	Perfect  bool   `json:"perfect,omitempty"`
	Played   int    `json:"played,omitempty"`
}

func newMenuRuntimeState(levels []menuLevel) menuRuntimeState {
	records := loadMenuRecords()
	applyMenuRecords(levels, records)
	st := menuRuntimeState{
		allLevels:      levels,
		libraryRecords: records,
		menuSort:       menuSortTitle,
	}
	st.rebuildMenuLevels()
	return st
}

func canonicalMenuLevelKey(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func loadMenuRecords() map[string]menuRecord {
	path := menuRecordsPath()
	if path == "" {
		return map[string]menuRecord{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]menuRecord{}
	}
	var out map[string]menuRecord
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Printf("engine: read library state %s: %v", path, err)
		return map[string]menuRecord{}
	}
	if out == nil {
		out = map[string]menuRecord{}
	}
	return out
}

func saveMenuRecords(records map[string]menuRecord) {
	path := menuRecordsPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("engine: create library state dir: %v", err)
		return
	}
	raw, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		log.Printf("engine: encode library state: %v", err)
		return
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		log.Printf("engine: write library state %s: %v", path, err)
	}
}

func menuRecordsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "heaven-go", "library.json")
}

func applyMenuRecords(levels []menuLevel, records map[string]menuRecord) {
	for i := range levels {
		levels[i].applyRecord(records[levels[i].key])
	}
}

func (l *menuLevel) applyRecord(rec menuRecord) {
	l.favorite = rec.Favorite
	l.perfect = rec.Perfect
	l.rank = menuRankFromString(rec.Rank)
}

func (m *menuRuntimeState) rebuildMenuLevels() {
	query := strings.TrimSpace(strings.ToLower(m.menuQuery))
	m.levels = m.levels[:0]
	for _, level := range m.allLevels {
		if m.favoritesOnly && !level.favorite {
			continue
		}
		if query != "" && !strings.Contains(menuLevelSearchText(level), query) {
			continue
		}
		m.levels = append(m.levels, level)
	}
	sort.SliceStable(m.levels, func(i, j int) bool {
		return lessMenuLevel(m.levels[i], m.levels[j], m.menuSort)
	})
	if m.menuSel >= len(m.levels) {
		m.menuSel = len(m.levels) - 1
	}
	if m.menuSel < 0 {
		m.menuSel = 0
	}
	if len(m.levels) == 0 {
		m.menuScroll = 0
	}
}

func menuLevelSearchText(level menuLevel) string {
	parts := []string{level.displayName(), level.fileName, level.author, level.desc, level.path}
	parts = append(parts, level.games...)
	return strings.ToLower(strings.Join(parts, "\n"))
}

func lessMenuLevel(a, b menuLevel, mode menuSortMode) bool {
	switch mode {
	case menuSortBPM:
		if a.bpm != b.bpm {
			return a.bpm < b.bpm
		}
	case menuSortRank:
		if menuRankScore(a) != menuRankScore(b) {
			return menuRankScore(a) > menuRankScore(b)
		}
	}
	aa := strings.ToLower(a.displayName())
	bb := strings.ToLower(b.displayName())
	if aa != bb {
		return aa < bb
	}
	return a.path < b.path
}

func menuRankScore(level menuLevel) int {
	if level.perfect {
		return 4
	}
	return int(level.rank)
}

func menuRankString(rank menuRank) string {
	switch rank {
	case menuRankTryAgain:
		return "tryAgain"
	case menuRankOK:
		return "ok"
	case menuRankSuperb:
		return "superb"
	default:
		return ""
	}
}

func menuRankFromString(s string) menuRank {
	switch strings.ToLower(s) {
	case "tryagain", "try_again", "ng":
		return menuRankTryAgain
	case "ok":
		return menuRankOK
	case "superb", "hi":
		return menuRankSuperb
	default:
		return menuRankUnplayed
	}
}

func menuRankFromResult(rank resultRank) menuRank {
	switch rank {
	case resultRankHi:
		return menuRankSuperb
	case resultRankOk:
		return menuRankOK
	default:
		return menuRankTryAgain
	}
}

func (m menuSortMode) label() string {
	switch m {
	case menuSortBPM:
		return "BPM"
	case menuSortRank:
		return "Rank"
	default:
		return "Title"
	}
}

func (r menuRank) label(perfect bool) string {
	if perfect {
		return "Perfect"
	}
	switch r {
	case menuRankTryAgain:
		return "Try Again"
	case menuRankOK:
		return "OK"
	case menuRankSuperb:
		return "Superb"
	default:
		return "Unplayed"
	}
}
