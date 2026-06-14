package engine

import (
	"bytes"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/riq"
)

// New 创建 App 并加载初始谱面（path 可为空：进入等待拖放的标题屏）。
func New(assetsRoot, riqPath string) (*App, error) {
	if audioCtx == nil {
		audioCtx = audio.NewContext(SampleRate)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	a := &App{
		assetsRoot:   assetsRoot,
		faceBig:      &text.GoTextFace{Source: src, Size: 44},
		faceMid:      &text.GoTextFace{Source: src, Size: 24},
		faceSmall:    &text.GoTextFace{Source: src, Size: 15},
		commonSounds: map[string][]byte{},
		levels:       discoverLevels("levels"),
	}
	a.loadCommonSounds()
	a.resultAssets = loadResultAssets(filepath.Join(assetsRoot, "common", "ratings"))
	a.resultAudio = loadResultAudio(filepath.Join(assetsRoot, "common", "result_sounds"))
	a.libraryAssets = loadLibraryAssets(filepath.Join(assetsRoot, "common", "library"))
	if riqPath != "" {
		r, err := riq.Load(riqPath)
		if err != nil {
			return nil, err
		}
		if err := a.loadRiq(r); err != nil {
			return nil, err
		}
	}
	return a, nil
}
