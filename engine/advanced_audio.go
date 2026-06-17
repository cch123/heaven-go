package engine

import (
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"hsdemo/kart"
	"hsdemo/riq"
)

func (a *App) scheduleAdvancedCustomSFX(e *riq.Entity) {
	name := e.Str("sfxName", "")
	if name == "" {
		return
	}
	pcm, ok := a.customSFX(name)
	if !ok {
		log.Printf("engine: advanced custom sfx %q not found in RIQ Resources/Sounds", name)
		return
	}
	a.scheduleAdvancedPCM(e, pcm)
}

func (a *App) scheduleAdvancedSFX(e *riq.Entity) {
	game := dropdownCurrent(e, "game")
	name := dropdownCurrent(e, "sfxName")
	if name == "" {
		return
	}
	switch strings.ToLower(game) {
	case "", "common":
		if pcm, ok := a.commonSounds[strings.ToLower(name)]; ok {
			a.scheduleAdvancedPCM(e, pcm)
		}
	case "custom":
		if pcm, ok := a.customSFX(name); ok {
			a.scheduleAdvancedPCM(e, pcm)
		}
	default:
		if pcm, ok := a.gameSound(game, name); ok {
			a.scheduleAdvancedPCM(e, pcm)
		}
	}
}

func (a *App) scheduleAdvancedPCM(e *riq.Entity, pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	vol := e.Float("volume", 1)
	pitch := advancedPitch(e)
	pan := e.Float("panning", 0)
	offsetSec := e.Float("offset", 0) / 1000
	startBeat := a.beatWithSFXOffset(e.Beat, offsetSec)
	if boolParam(e, "loop") {
		var handle *SoundLoopHandle
		a.at(startBeat, func() { handle = a.playAdvancedLoop(pcm, vol, pitch, pan) })
		a.at(e.Beat+e.Length, func() {
			if handle != nil {
				handle.Stop()
			}
		})
		return
	}
	a.at(startBeat, func() { playAdvancedPCM(pcm, vol, pitch, pan) })
}

func (a *App) beatWithSFXOffset(beat, offsetSec float64) float64 {
	if a.bm == nil || offsetSec == 0 {
		return beat
	}
	t := a.bm.BeatToTime(beat) - offsetSec
	return a.bm.TimeToBeat(t)
}

func (a *App) customSFX(name string) ([]byte, bool) {
	if a.customSfx == nil {
		return nil, false
	}
	if pcm, ok := a.customSfx[name]; ok {
		return pcm, true
	}
	pcm, ok := a.customSfx[strings.ToLower(name)]
	return pcm, ok
}

func (a *App) gameSound(game, name string) ([]byte, bool) {
	if a.gameSfxPCM == nil {
		a.gameSfxPCM = map[string][]byte{}
	}
	key := strings.ToLower(game + "/" + name)
	if pcm, ok := a.gameSfxPCM[key]; ok {
		return pcm, true
	}
	dir := filepath.Join(a.assetsRoot, game, "sounds")
	for _, ext := range []string{".wav", ".ogg", ".mp3"} {
		p := filepath.Join(dir, name+ext)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		pcm, err := decodeAssetSound(raw, ext)
		if err != nil {
			log.Printf("engine: advanced game sfx %s decode failed: %v", p, err)
			return nil, false
		}
		a.gameSfxPCM[key] = pcm
		return pcm, true
	}
	return nil, false
}

func decodeAssetSound(raw []byte, ext string) ([]byte, error) {
	switch strings.ToLower(ext) {
	case ".wav", ".ogg":
		return kart.DecodePCM(raw, ext, SampleRate)
	case ".mp3":
		stream, err := decodeAudioStream(raw, riq.AudioMP3, "advanced mp3 sfx")
		if err != nil {
			return nil, err
		}
		return io.ReadAll(stream)
	default:
		return nil, nil
	}
}

func playAdvancedPCM(pcm []byte, vol, pitch, pan float64) {
	pcm = transformAdvancedPCM(pcm, pitch, pan)
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

func (a *App) playAdvancedLoop(pcm []byte, vol, pitch, pan float64) *SoundLoopHandle {
	pcm = transformAdvancedPCM(pcm, 1, pan)
	reader := newPitchLoopReader(pcm, pitch)
	p, err := audioCtx.NewPlayer(reader)
	if err != nil {
		return &SoundLoopHandle{}
	}
	p.SetVolume(vol)
	p.Play()
	return &SoundLoopHandle{player: p, reader: reader}
}

func transformAdvancedPCM(pcm []byte, pitch, pan float64) []byte {
	if pitch <= 0 || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		pitch = 1
	}
	if pitch != 1 {
		pcm = kart.ResamplePCM(pcm, pitch)
	}
	if pan != 0 {
		pcm = kart.PanPCM(pcm, pan)
	}
	return pcm
}

func advancedPitch(e *riq.Entity) float64 {
	if !boolParam(e, "useSemitones") {
		return e.Float("pitch", 1)
	}
	cents := int(e.Float("semitones", 0))*100 + int(e.Float("cents", 0))
	return math.Pow(2, float64(cents)/1200)
}

func dropdownCurrent(e *riq.Entity, key string) string {
	v, ok := e.Data[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	values, ok := m["Values"].([]any)
	if !ok {
		return ""
	}
	idx := int(0)
	if f, ok := m["value"].(float64); ok {
		idx = int(f)
	}
	if idx < 0 || idx >= len(values) {
		return ""
	}
	if s, ok := values[idx].(string); ok {
		return s
	}
	return ""
}
