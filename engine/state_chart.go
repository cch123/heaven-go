package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/conductor"
	"hsdemo/riq"
)

// chartRuntimeState is rebuilt whenever a new .riq is loaded.
type chartRuntimeState struct {
	r             *riq.Riq
	bm            *riq.Beatmap
	cond          *conductor.Conductor
	player        *audio.Player
	music         *pitchPCMReader
	customSfx     map[string][]byte
	customSprites map[string]*ebiten.Image
	gameSfxPCM    map[string][]byte
}
