package engine

import (
	"archive/zip"
	"image"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

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
