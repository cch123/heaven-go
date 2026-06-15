package engine

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func (a *App) advancePlayTimeline(beat float64) {
	for a.swIdx < len(a.switches) && a.switches[a.swIdx].beat <= beat {
		a.setActive(a.switches[a.swIdx].id, a.switches[a.swIdx].beat)
		a.swIdx++
	}
	for a.actIdx < len(a.actions) && a.actions[a.actIdx].beat <= beat {
		a.actions[a.actIdx].fn()
		a.actIdx++
	}
}

func (a *App) updateTimingArrow() {
	dt := 1.0 / float64(ebiten.TPS())
	a.tdArrow += (a.tdTarget - a.tdArrow) * math.Min(4*dt, 1)
}

func (a *App) updatePlayMusicVolume(beat float64) {
	// 音量时间轴（riq__VolumeChange）再乘游戏局部 ducking；
	// Tunnel/FadeMinigameVolume 这类事件不改写谱面 volume 曲线。
	a.player.SetVolume(a.bm.VolumeAt(beat) * a.MusicFadeAt(beat))
}
