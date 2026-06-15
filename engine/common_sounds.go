package engine

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"hsdemo/kart"
)

// loadCommonSounds 加载 assets/common/sounds（可选；缺目录时 countIn 等事件静默跳过）。
func (a *App) loadCommonSounds() {
	dir := filepath.Join(a.assetsRoot, "common", "sounds")
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("engine: 无公共音效目录 %s（countIn 计数音不可用，运行 go run ./cmd/extract -game common）", dir)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		pcm, err := kart.DecodePCM(raw, filepath.Ext(e.Name()), SampleRate)
		if err != nil {
			log.Printf("engine: 公共音效 %s 解码失败: %v", e.Name(), err)
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		a.commonSounds[strings.ToLower(base)] = pcm
	}
}

// PlayCommon 立即播放公共音效（大小写不敏感，Unity Resources.Load 同语义）。
func (a *App) PlayCommon(name string, vol float64) {
	pcm, ok := a.commonSounds[strings.ToLower(name)]
	if !ok {
		return
	}
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

// commonAt 在指定拍播放公共音效。
func (a *App) commonAt(beat float64, name string) {
	a.at(beat, func() { a.PlayCommon(name, 1) })
}
