package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type menuV1Chart struct {
	Properties   map[string]any `json:"properties"`
	Entities     []menuV1Entity `json:"entities"`
	TempoChanges []menuV1Entity `json:"tempoChanges"`
}

type menuV1Entity struct {
	Datamodel   string         `json:"datamodel"`
	DynamicData map[string]any `json:"dynamicData"`
}

type menuV2Chart struct {
	SongName string            `json:"songname"`
	Entities []menuV2Entity    `json:"entities"`
	Models   map[string]string `json:"models"`
	Types    map[string]string `json:"types"`
}

type menuV2Entity struct {
	Type  int            `json:"type"`
	Model int            `json:"model"`
	Data  map[string]any `json:"data"`
}

func applyV1MenuMetadata(level *menuLevel, raw []byte) {
	var c menuV1Chart
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &c); err != nil {
		log.Printf("engine: parse level metadata %s: %v", level.path, err)
		return
	}
	if title := menuString(c.Properties, "remixtitle"); title != "" {
		level.title = title
	}
	level.author = menuString(c.Properties, "remixauthor")
	level.desc = menuString(c.Properties, "remixdesc")
	if bpm := menuFloat(c.Properties, "remixtempo"); bpm > 0 {
		level.bpm = bpm
	}
	for _, e := range c.TempoChanges {
		if bpm := menuFloat(e.DynamicData, "tempo"); bpm > 0 {
			level.bpm = bpm
			break
		}
	}
	level.games = summarizeMenuGames(v1MenuModels(c.Entities))
}

func v1MenuModels(entities []menuV1Entity) []string {
	models := make([]string, 0, len(entities))
	for _, e := range entities {
		models = append(models, e.Datamodel)
	}
	return models
}

func applyV2MenuMetadata(level *menuLevel, raw []byte) {
	var c menuV2Chart
	if err := json.Unmarshal(raw, &c); err != nil {
		log.Printf("engine: parse level metadata %s: %v", level.path, err)
		return
	}
	if c.SongName != "" {
		level.title = c.SongName
	}
	models := make([]string, 0, len(c.Entities))
	for _, e := range c.Entities {
		model := c.Models[fmt.Sprint(e.Model)]
		typ := c.Types[fmt.Sprint(e.Type)]
		if typ == "riq__TempoChange" {
			if bpm := menuFloat(e.Data, "tempo"); bpm > 0 {
				level.bpm = bpm
			}
			continue
		}
		models = append(models, model)
	}
	level.games = summarizeMenuGames(models)
}

func summarizeMenuGames(models []string) []string {
	switches := make([]string, 0, 4)
	fallback := make([]string, 0, 4)
	seenSwitches := map[string]bool{}
	seenFallback := map[string]bool{}
	for _, model := range models {
		if game, ok := strings.CutPrefix(model, "gameManager/switchGame/"); ok {
			addUniqueGame(&switches, seenSwitches, game)
			continue
		}
		game, _, ok := strings.Cut(model, "/")
		if !ok || ignoredMenuGame(game) {
			continue
		}
		addUniqueGame(&fallback, seenFallback, game)
	}
	if len(switches) > 0 {
		return switches
	}
	return fallback
}

func ignoredMenuGame(game string) bool {
	switch game {
	case "", "gameManager", "global", "vfx", "ppe", "countIn":
		return true
	}
	return false
}

func addUniqueGame(out *[]string, seen map[string]bool, game string) {
	if game == "" || seen[game] {
		return
	}
	seen[game] = true
	*out = append(*out, game)
}

func menuString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func menuFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return 0
}
