package engine

import (
	"encoding/binary"
	"io"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// SoundLoopHandle represents a looping SoundByte instance whose pitch can be
// changed while it is playing. Unity's Sound.BendUp/BendDown modifies
// AudioSource.pitch in place; recreating the player would reset the waveform
// phase and produces an audible click in Rockers' guitar loops.
type SoundLoopHandle struct {
	player *audio.Player
	reader *pitchLoopReader
}

func (h *SoundLoopHandle) Stop() {
	if h == nil || h.player == nil {
		return
	}
	_ = h.player.Close()
	h.player = nil
}

func (h *SoundLoopHandle) StopFunc() func() {
	return func() { h.Stop() }
}

func (h *SoundLoopHandle) SetPitch(pitch float64) {
	if h == nil || h.reader == nil {
		return
	}
	h.reader.SetPitch(pitch)
}

func (h *SoundLoopHandle) RampPitch(target, seconds float64) {
	if h == nil || h.reader == nil {
		return
	}
	h.reader.RampPitch(target, seconds)
}

type pitchLoopReader struct {
	mu sync.Mutex

	pcm    []byte
	frames int
	pos    float64

	pitch       float64
	rampFrom    float64
	rampTo      float64
	rampFrames  float64
	rampElapsed float64
}

func newPitchLoopReader(pcm []byte, pitch float64) *pitchLoopReader {
	return &pitchLoopReader{
		pcm:    pcm,
		frames: len(pcm) / 4,
		pitch:  clampLoopPitch(pitch),
	}
}

func (r *pitchLoopReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r == nil || r.frames == 0 {
		for i := range p {
			p[i] = 0
		}
		return len(p), io.EOF
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	frameBytes := (len(p) / 4) * 4
	for off := 0; off < frameBytes; off += 4 {
		pitch := r.nextPitchLocked()
		r.writeFrameLocked(p[off : off+4])
		r.pos += pitch
		if r.pos >= float64(r.frames) || r.pos < 0 {
			r.pos = math.Mod(r.pos, float64(r.frames))
			if r.pos < 0 {
				r.pos += float64(r.frames)
			}
		}
	}
	for i := frameBytes; i < len(p); i++ {
		p[i] = 0
	}
	return len(p), nil
}

func (r *pitchLoopReader) SetPitch(pitch float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pitch = clampLoopPitch(pitch)
	r.rampFrames = 0
	r.rampElapsed = 0
}

func (r *pitchLoopReader) RampPitch(target, seconds float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	target = clampLoopPitch(target)
	if seconds <= 0 {
		r.pitch = target
		r.rampFrames = 0
		r.rampElapsed = 0
		return
	}
	r.rampFrom = r.currentPitchLocked()
	r.rampTo = target
	r.rampFrames = seconds * SampleRate
	r.rampElapsed = 0
}

func (r *pitchLoopReader) currentPitchLocked() float64 {
	if r.rampFrames <= 0 {
		return r.pitch
	}
	u := r.rampElapsed / r.rampFrames
	if u < 0 {
		u = 0
	} else if u > 1 {
		u = 1
	}
	return r.rampFrom + (r.rampTo-r.rampFrom)*u
}

func (r *pitchLoopReader) nextPitchLocked() float64 {
	pitch := r.currentPitchLocked()
	if r.rampFrames > 0 {
		r.rampElapsed++
		if r.rampElapsed >= r.rampFrames {
			r.pitch = r.rampTo
			r.rampFrames = 0
			r.rampElapsed = 0
		}
	}
	return pitch
}

func (r *pitchLoopReader) writeFrameLocked(dst []byte) {
	j := int(r.pos)
	if j >= r.frames {
		j = r.frames - 1
	}
	next := j + 1
	if next >= r.frames {
		next = 0
	}
	frac := r.pos - float64(j)
	for ch := 0; ch < 2; ch++ {
		a := pcmSample16(r.pcm, j, ch)
		b := pcmSample16(r.pcm, next, ch)
		v := a + (b-a)*frac
		if v > math.MaxInt16 {
			v = math.MaxInt16
		} else if v < math.MinInt16 {
			v = math.MinInt16
		}
		binary.LittleEndian.PutUint16(dst[ch*2:], uint16(int16(math.Round(v))))
	}
}

func pcmSample16(pcm []byte, frame, ch int) float64 {
	off := frame*4 + ch*2
	return float64(int16(binary.LittleEndian.Uint16(pcm[off : off+2])))
}

func clampLoopPitch(pitch float64) float64 {
	if math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		return 1
	}
	if pitch < 0.01 {
		return 0.01
	}
	return pitch
}
