package engine

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

// ---------- riq 拖放导入 ----------

func (a *App) pollDroppedRiq() {
	df := ebiten.DroppedFiles()
	if df == nil {
		return
	}
	entries, err := fs.ReadDir(df, ".")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".riq") {
			continue
		}
		b, err := fs.ReadFile(df, name)
		if err != nil {
			a.loadErr = fmt.Sprintf("read %s failed: %v", name, err)
			return
		}
		r, err := riq.LoadBytes(b)
		if err != nil {
			a.loadErr = fmt.Sprintf("%s is not a valid riq: %v", name, err)
			return
		}
		if err := a.loadRiq(r); err != nil {
			a.loadErr = fmt.Sprintf("load %s failed: %v", filepath.Base(name), err)
			return
		}
		a.loadErr = ""
		a.state = stateTitle
		return
	}
}
