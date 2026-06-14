package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

func discoverLevels(dir string) []menuLevel {
	paths, err := filepath.Glob(filepath.Join(dir, "*.riq"))
	if err != nil {
		return nil
	}
	sort.Strings(paths)
	out := make([]menuLevel, 0, len(paths))
	for _, p := range paths {
		out = append(out, inspectMenuLevel(p))
	}
	return out
}

func inspectMenuLevel(p string) menuLevel {
	fileName := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	level := menuLevel{
		path:     p,
		fileName: fileName,
		title:    fileName,
		bpm:      120,
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		return level
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			files[f.Name] = f
		}
	}
	if raw, ok := readZipFile(files, "remix.json"); ok {
		applyV1MenuMetadata(&level, raw)
	} else if chartName, ok := findZipChart(files); ok {
		if raw, ok := readZipFile(files, chartName); ok {
			applyV2MenuMetadata(&level, raw)
		}
	}
	level.customIcon = readLibraryLevelIcon(files)
	return level
}

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

func findZipChart(files map[string]*zip.File) (string, bool) {
	if _, ok := files["Charts/chart0.json"]; ok {
		return "Charts/chart0.json", true
	}
	names := make([]string, 0)
	for name := range files {
		if strings.HasPrefix(name, "Charts/") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	return names[0], true
}

func readZipFile(files map[string]*zip.File, name string) ([]byte, bool) {
	f, ok := findZipFile(files, name)
	if !ok {
		return nil, false
	}
	rc, err := f.Open()
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return b, err == nil
}

func readLibraryLevelIcon(files map[string]*zip.File) *ebiten.Image {
	for _, name := range []string{
		"Resources/Images/LibraryIcon/LibraryLevelIcon.png",
		"Resources/Images/LibraryIcon/LibraryLevelIcon.jpg",
		"Resources/Images/LibraryIcon/LibraryLevelIcon.jpeg",
	} {
		f, ok := findZipFile(files, name)
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		img, _, err := image.Decode(rc)
		rc.Close()
		if err != nil {
			log.Printf("engine: decode library icon %s: %v", name, err)
			continue
		}
		return ebiten.NewImageFromImage(img)
	}
	return nil
}

func findZipFile(files map[string]*zip.File, name string) (*zip.File, bool) {
	if f, ok := files[name]; ok {
		return f, true
	}
	for path, f := range files {
		if strings.EqualFold(path, name) {
			return f, true
		}
	}
	return nil, false
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

func (l menuLevel) displayName() string {
	if l.title != "" {
		return l.title
	}
	return l.fileName
}
