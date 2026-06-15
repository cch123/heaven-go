package engine

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
)

// pitchPCMReader resamples 16-bit stereo PCM at runtime while preserving a
// seekable source-time position. It backs chart music pitch changes such as
// Conductor.SetMinigamePitch; the loop-specific SoundByte reader remains in
// sound_loop.go because loop sources intentionally wrap at EOF.
type pitchPCMReader struct {
	mu sync.Mutex

	pcm    []byte
	frames int
	pos    float64

	pitch float64
}

func newPitchPCMReader(pcm []byte, pitch float64) *pitchPCMReader {
	return &pitchPCMReader{
		pcm:    pcm,
		frames: len(pcm) / 4,
		pitch:  clampLoopPitch(pitch),
	}
}

func (r *pitchPCMReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r == nil || r.frames == 0 {
		return 0, io.EOF
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	frameBytes := (len(p) / 4) * 4
	written := 0
	for off := 0; off < frameBytes; off += 4 {
		if r.pos >= float64(r.frames) {
			return written, io.EOF
		}
		r.writeFrameLocked(p[off : off+4])
		written += 4
		r.pos += r.pitch
	}
	for i := frameBytes; i < len(p); i++ {
		p[i] = 0
		written++
	}
	return written, nil
}

func (r *pitchPCMReader) Seek(offset int64, whence int) (int64, error) {
	if r == nil {
		return 0, io.ErrUnexpectedEOF
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = int64(r.pos) * 4
	case io.SeekEnd:
		base = int64(r.frames) * 4
	default:
		return int64(r.pos) * 4, io.ErrUnexpectedEOF
	}
	next := base + offset
	if next < 0 {
		next = 0
	}
	max := int64(r.frames) * 4
	if next > max {
		next = max
	}
	next -= next % 4
	r.pos = float64(next / 4)
	return next, nil
}

func (r *pitchPCMReader) SetPitch(pitch float64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pitch = clampLoopPitch(pitch)
}

func (r *pitchPCMReader) PositionSeconds() float64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pos / SampleRate
}

func (r *pitchPCMReader) writeFrameLocked(dst []byte) {
	j := int(r.pos)
	if j >= r.frames {
		j = r.frames - 1
	}
	next := j + 1
	if next >= r.frames {
		next = r.frames - 1
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
