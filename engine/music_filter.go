package engine

import "math"

// Unity's MainMixer routes chart music through a Music group with Highpass and
// Lowpass effects. Most snapshots leave those filters effectively transparent;
// DJ School's Hold snapshot narrows them into a scratchy radio band.
type musicFilterSnapshot struct {
	highpassHz float64
	lowpassHz  float64
	gainDB     float64
}

type musicFilterState struct {
	current musicFilterSnapshot
	from    musicFilterSnapshot
	target  musicFilterSnapshot

	transitionFrame  int
	transitionFrames int

	highpass biquadFilterState
	lowpass  biquadFilterState
}

type biquadFilterState struct {
	x1 [2]float64
	x2 [2]float64
	y1 [2]float64
	y2 [2]float64
}

type biquadCoeffs struct {
	b0 float64
	b1 float64
	b2 float64
	a1 float64
	a2 float64
}

var mainMusicFilterSnapshot = musicFilterSnapshot{
	highpassHz: 10,
	lowpassHz:  22000,
	gainDB:     0.01,
}

func newMusicFilterState() musicFilterState {
	s := normalizeMusicFilterSnapshot(mainMusicFilterSnapshot)
	return musicFilterState{current: s, from: s, target: s}
}

func (a *App) transitionMusicFilter(highpassHz, lowpassHz, gainDB, seconds float64) {
	if a.music == nil {
		return
	}
	a.music.transitionMusicFilter(highpassHz, lowpassHz, gainDB, seconds)
}

func (a *App) resetMusicFilter() {
	if a.music == nil {
		return
	}
	a.music.resetMusicFilter()
}

func (r *pitchPCMReader) transitionMusicFilter(highpassHz, lowpassHz, gainDB, seconds float64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filter.transitionTo(musicFilterSnapshot{
		highpassHz: highpassHz,
		lowpassHz:  lowpassHz,
		gainDB:     gainDB,
	}, seconds)
}

func (r *pitchPCMReader) resetMusicFilter() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filter.resetToMain()
}

func (f *musicFilterState) resetToMain() {
	*f = newMusicFilterState()
}

func (f *musicFilterState) transitionTo(target musicFilterSnapshot, seconds float64) {
	target = normalizeMusicFilterSnapshot(target)
	f.from = f.current
	f.target = target
	f.transitionFrame = 0
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		f.transitionFrames = 0
		f.current = target
		f.from = target
		return
	}
	f.transitionFrames = int(math.Round(seconds * float64(SampleRate)))
	if f.transitionFrames <= 0 {
		f.current = target
		f.from = target
	}
}

func (f *musicFilterState) processFrame(left, right float64) (float64, float64) {
	s := f.nextSnapshot()
	if s.bypass() {
		f.highpass.trackPassthrough(left, right)
		f.lowpass.trackPassthrough(left, right)
		return left, right
	}

	if s.highpassHz > 20 {
		c := highpassBiquad(s.highpassHz, 1)
		left = f.highpass.process(0, left, c)
		right = f.highpass.process(1, right, c)
	} else {
		f.highpass.trackPassthrough(left, right)
	}

	if s.lowpassHz < float64(SampleRate)*0.49 {
		c := lowpassBiquad(s.lowpassHz, 1)
		left = f.lowpass.process(0, left, c)
		right = f.lowpass.process(1, right, c)
	} else {
		f.lowpass.trackPassthrough(left, right)
	}

	gain := math.Pow(10, s.gainDB/20)
	return left * gain, right * gain
}

func (f *musicFilterState) nextSnapshot() musicFilterSnapshot {
	if f.transitionFrames <= 0 || f.transitionFrame >= f.transitionFrames {
		f.current = f.target
		return f.current
	}
	u := float64(f.transitionFrame) / float64(f.transitionFrames)
	f.transitionFrame++
	f.current = musicFilterSnapshot{
		highpassHz: lerpMusicFilter(f.from.highpassHz, f.target.highpassHz, u),
		lowpassHz:  lerpMusicFilter(f.from.lowpassHz, f.target.lowpassHz, u),
		gainDB:     lerpMusicFilter(f.from.gainDB, f.target.gainDB, u),
	}
	return f.current
}

func (s musicFilterSnapshot) bypass() bool {
	return s.highpassHz <= 20 &&
		s.lowpassHz >= float64(SampleRate)*0.49 &&
		math.Abs(s.gainDB) < 0.02
}

func normalizeMusicFilterSnapshot(s musicFilterSnapshot) musicFilterSnapshot {
	nyquist := float64(SampleRate) / 2
	s.highpassHz = clampFilterHz(s.highpassHz, 0, nyquist-10, mainMusicFilterSnapshot.highpassHz)
	s.lowpassHz = clampFilterHz(s.lowpassHz, 10, nyquist-10, mainMusicFilterSnapshot.lowpassHz)
	if math.IsNaN(s.gainDB) || math.IsInf(s.gainDB, 0) {
		s.gainDB = mainMusicFilterSnapshot.gainDB
	}
	return s
}

func clampFilterHz(v, lo, hi, fallback float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func lowpassBiquad(cutoffHz, q float64) biquadCoeffs {
	omega := 2 * math.Pi * cutoffHz / float64(SampleRate)
	sin, cos := math.Sin(omega), math.Cos(omega)
	alpha := sin / (2 * q)
	b0 := (1 - cos) / 2
	b1 := 1 - cos
	b2 := (1 - cos) / 2
	a0 := 1 + alpha
	a1 := -2 * cos
	a2 := 1 - alpha
	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

func highpassBiquad(cutoffHz, q float64) biquadCoeffs {
	omega := 2 * math.Pi * cutoffHz / float64(SampleRate)
	sin, cos := math.Sin(omega), math.Cos(omega)
	alpha := sin / (2 * q)
	b0 := (1 + cos) / 2
	b1 := -(1 + cos)
	b2 := (1 + cos) / 2
	a0 := 1 + alpha
	a1 := -2 * cos
	a2 := 1 - alpha
	return normalizeBiquad(b0, b1, b2, a0, a1, a2)
}

func normalizeBiquad(b0, b1, b2, a0, a1, a2 float64) biquadCoeffs {
	return biquadCoeffs{
		b0: b0 / a0,
		b1: b1 / a0,
		b2: b2 / a0,
		a1: a1 / a0,
		a2: a2 / a0,
	}
}

func (s *biquadFilterState) process(ch int, x float64, c biquadCoeffs) float64 {
	y := c.b0*x + c.b1*s.x1[ch] + c.b2*s.x2[ch] - c.a1*s.y1[ch] - c.a2*s.y2[ch]
	s.x2[ch], s.x1[ch] = s.x1[ch], x
	s.y2[ch], s.y1[ch] = s.y1[ch], y
	return y
}

func (s *biquadFilterState) trackPassthrough(left, right float64) {
	s.trackChannel(0, left)
	s.trackChannel(1, right)
}

func (s *biquadFilterState) trackChannel(ch int, x float64) {
	s.x2[ch], s.x1[ch] = s.x1[ch], x
	s.y2[ch], s.y1[ch] = s.y1[ch], x
}

func clampPCM16(v float64) float64 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return v
}

func lerpMusicFilter(a, b, u float64) float64 { return a + (b-a)*u }
