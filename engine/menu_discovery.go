package engine

import (
	"archive/zip"
	"path/filepath"
	"sort"
	"strings"
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

func (l menuLevel) displayName() string {
	if l.title != "" {
		return l.title
	}
	return l.fileName
}
