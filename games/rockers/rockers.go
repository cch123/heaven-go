// Package rockers ports Rockers' call-and-response guitar flow.
package rockers

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	whoJJ = iota
	whoSoshi
	whoBoth
)

const (
	voiceCmon = iota
	voiceLastOne
	voiceNone
)

type riffEvent struct {
	beat, length float64
	respond      bool
	jj, soshi    riffPart
	togetherEnd  bool
}

type bendEvent struct {
	beat, length float64
	respond      bool
	pitchJJ      int
	pitchSoshi   int
}

type riffPart struct {
	pitches    [6]int
	gleeClub   bool
	sample     int
	sampleTone int
}

type intervalEvent struct {
	beat, length       float64
	autoPass, movePass bool
}

type passEvent struct {
	beat       float64
	moveCamera bool
}

type togetherPrepare struct {
	beat, muteBeat, middleBeat float64
	voice                      int
	moveCamera                 bool
}

type cameraMove struct {
	start, from, to float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	jj, soshi rocker

	riffs    []riffEvent
	bends    []bendEvent
	ivals    []intervalEvent
	passes   []passEvent
	together []riffEvent
	preps    []togetherPrepare
	cams     []cameraMove
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "rockers" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rockers"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.jj.init(m, "JJHolder", "JJHolder/StrumEffects", true)
	m.soshi.init(m, "StudentHolder", "StudentHolder/StrumEffects", false)
	m.initScene(0)
	return nil
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	for _, p := range []string{"JJHolder", "StudentHolder", "JJHolder/StrumEffects", "StudentHolder/StrumEffects"} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.ctx.Scene.SetActive("JJHolder/StrumEffects", false)
	m.ctx.Scene.SetActive("StudentHolder/StrumEffects", false)
	m.jj.stopSounds()
	m.soshi.stopSounds()
	m.jj.muted, m.soshi.muted = false, false
	m.jj.strum, m.soshi.strum = false, false
	m.jj.bending, m.soshi.bending = false, false
	m.jj.together, m.soshi.together = false, false
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "rockers/intervalStart":
		m.addInterval(e)
	case "rockers/riff":
		m.riffs = append(m.riffs, parseRiff(e, false))
	case "rockers/bend":
		m.bends = append(m.bends, bendEvent{
			beat: e.Beat, length: defaultLength(e.Length, 1), respond: boolParamDefault(e, "respond", true),
			pitchJJ: int(e.Float("1JJ", 1)), pitchSoshi: int(e.Float("1S", 1)),
		})
	case "rockers/prepare":
		who := int(e.Float("who", whoJJ))
		m.ctx.At(e.Beat, func() { m.muteWho(who) })
	case "rockers/unPrepare":
		who := int(e.Float("who", whoJJ))
		m.ctx.At(e.Beat, func() { m.unMuteWho(who) })
	case "rockers/passTurn":
		mv := boolParamDefault(e, "moveCamera", true)
		m.passes = append(m.passes, passEvent{beat: e.Beat, moveCamera: mv})
		if mv {
			m.addCameraMove(e.Beat-1, 2.8)
		}
	case "rockers/count":
		m.scheduleCount(e)
	case "rockers/voiceLine":
		cmon := boolParamDefault(e, "cmon", true)
		name := "LastOne"
		if cmon {
			name = "Cmon"
		}
		m.ctx.SoundAt(e.Beat, name, 1)
	case "rockers/cmon":
		m.defaultCmon(e)
	case "rockers/lastOne":
		m.defaultLastOne(e)
	case "rockers/prepareTogether":
		ev := togetherPrepare{
			beat: e.Beat, muteBeat: e.Float("muteBeat", 2), middleBeat: e.Float("middleBeat", 2),
			voice: int(e.Float("cmon", voiceCmon)), moveCamera: boolParamDefault(e, "moveCamera", true),
		}
		m.preps = append(m.preps, ev)
		if ev.moveCamera {
			m.addCameraMove(ev.beat+ev.middleBeat, 0)
		}
	case "rockers/riffTogether":
		m.together = append(m.together, parseRiff(e, false))
	case "rockers/riffTogetherEnd":
		m.together = append(m.together, parseRiff(e, true))
	}
}

func (m *Module) Ready() {
	sort.Slice(m.riffs, func(i, j int) bool { return m.riffs[i].beat < m.riffs[j].beat })
	sort.Slice(m.bends, func(i, j int) bool { return m.bends[i].beat < m.bends[j].beat })
	sort.Slice(m.ivals, func(i, j int) bool { return m.ivals[i].beat < m.ivals[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.preps, func(i, j int) bool { return m.preps[i].beat < m.preps[j].beat })
	sort.Slice(m.together, func(i, j int) bool { return m.together[i].beat < m.together[j].beat })
	sort.Slice(m.cams, func(i, j int) bool { return m.cams[i].start < m.cams[j].start })
	pos := 0.0
	for i := range m.cams {
		m.cams[i].from = pos
		pos = m.cams[i].to
	}
	for _, iv := range m.ivals {
		m.startInterval(iv)
		if iv.autoPass {
			m.passTurn(iv.beat+iv.length, iv.movePass, iv)
		}
	}
	for _, pass := range m.passes {
		m.passTurn(pass.beat, pass.moveCamera, m.lastIntervalBefore(pass.beat))
	}
	for _, prep := range m.preps {
		m.prepareTogether(prep)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.initScene(beat)
}

func (m *Module) Whiff(beat float64) {
	m.soshi.mute(true, false)
}

func (m *Module) Update(_, _ float64) {
	if m.ctx.PressedNow() && !m.ctx.ExpectingPressNow() {
		m.soshi.mute(true, false)
	}
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		m.soshi.unHold(false)
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{R: 0xeb, G: 0x4c, B: 0x94, A: 0xff})
	cam := m.ctx.CameraAt(beat)
	m.ctx.Scene.SetCamera(cam[0]+m.cameraX(beat), cam[1], cam[2])
	m.ctx.Scene.Sample(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) addInterval(e *riq.Entity) {
	length := defaultLength(e.Length, 8)
	ev := intervalEvent{
		beat: e.Beat, length: length,
		autoPass: boolParamDefault(e, "auto", true), movePass: boolParamDefault(e, "movePass", true),
	}
	m.ivals = append(m.ivals, ev)
	if boolParamDefault(e, "moveCamera", true) {
		m.addCameraMove(e.Beat-1, -2.76)
	}
	if ev.autoPass {
		passBeat := ev.beat + ev.length
		if ev.movePass {
			m.addCameraMove(passBeat-1, 2.8)
		}
	}
}

func (m *Module) startInterval(iv intervalEvent) {
	for _, r := range m.riffsBetween(iv.beat, iv.beat+iv.length) {
		r := r
		m.ctx.At(r.beat, func() {
			if m.ctx.GameAt(r.beat) == m.ID() {
				m.jj.strumStrings(r.jj.gleeClub, r.jj.pitches, sampleAt(r.jj.sample), r.jj.sampleTone, !r.respond, false, false)
			}
		})
		m.ctx.At(r.beat+r.length, func() {
			if m.jj.strum {
				m.jj.mute(true, false)
			}
		})
	}
	for _, b := range m.bendsBetween(iv.beat, iv.beat+iv.length) {
		b := b
		m.ctx.At(b.beat, func() {
			if m.ctx.GameAt(b.beat) == m.ID() {
				m.jj.bendUp(b.pitchJJ)
			}
		})
		m.ctx.At(b.beat+b.length, func() {
			if m.jj.bending {
				m.jj.bendDown()
			}
		})
	}
	m.ctx.At(iv.beat, func() {
		if m.jj.together || m.soshi.together {
			m.jj.returnBack()
			m.soshi.returnBack()
		}
	})
}

func (m *Module) passTurn(passBeat float64, moveCamera bool, iv intervalEvent) {
	if iv.length <= 0 {
		return
	}
	m.ctx.At(passBeat, func() { m.jj.unHold(false) })
	for _, r := range m.riffsBetween(iv.beat, iv.beat+iv.length) {
		if !r.respond {
			continue
		}
		r := r
		target := passBeat + (r.beat - iv.beat)
		m.scheduleSoshiRiff(target, r.length, r.soshi, false)
	}
	for _, b := range m.bendsBetween(iv.beat, iv.beat+iv.length) {
		if !b.respond {
			continue
		}
		b := b
		target := passBeat + (b.beat - iv.beat)
		m.ctx.ScheduleInputActionReleaseCond(target, 4,
			func() bool { return m.ctx.GameAt(target) == m.ID() },
			func(_ float64, _ engine.Judgment) { m.soshi.bendDown() },
			func() { m.jj.miss() },
		)
	}
}

func (m *Module) scheduleSoshiRiff(target, length float64, part riffPart, jump bool) {
	m.ctx.ScheduleInputReleaseCond(target,
		func() bool { return m.ctx.GameAt(target) == m.ID() },
		func(_ float64, j engine.Judgment) {
			m.soshi.strumStrings(part.gleeClub, part.pitches, sampleAt(part.sample), part.sampleTone, false, jump, j == engine.JudgeNG)
		},
		func() { m.jj.miss() },
	)
	m.ctx.ScheduleInputNoScore(target+length, func(_ float64, _ engine.Judgment) {
		m.soshi.mute(true, false)
	}, nil)
}

func (m *Module) muteWho(who int) {
	if who == whoJJ || who == whoBoth {
		m.jj.mute(true, false)
	}
	if (who == whoSoshi || who == whoBoth) && m.ctx.App.Autoplay {
		m.soshi.mute(true, false)
	}
}

func (m *Module) unMuteWho(who int) {
	if who == whoJJ || who == whoBoth {
		m.jj.unHold(true)
	}
	if (who == whoSoshi || who == whoBoth) && m.ctx.App.Autoplay {
		m.soshi.unHold(true)
	}
}

func (m *Module) scheduleCount(e *riq.Entity) {
	n := int(e.Float("count", 1))
	offset := 0.0
	switch n {
	case 1:
		offset = 0.028
	case 2, 3:
		offset = 0.033
	case 4:
		offset = 0.034
	}
	m.ctx.SoundAtOff(e.Beat, "count/count"+itoa1(n), 1, offset)
}

func (m *Module) defaultCmon(e *riq.Entity) {
	beat := e.Beat
	m.ctx.SoundAt(beat, "Cmon", 1)
	if boolParamDefault(e, "moveCamera", true) {
		m.addCameraMove(beat+2, 0)
	}
	jjSamples := [4]int{int(e.Float("JJ1", sampleChordG5)), int(e.Float("JJ2", sampleChordG5)), int(e.Float("JJ3", sampleChordG5)), int(e.Float("JJ4", sampleChordA))}
	jjPitch := [4]int{int(e.Float("pJJ1", 0)), int(e.Float("pJJ2", 0)), int(e.Float("pJJ3", 0)), int(e.Float("pJJ4", 0))}
	sSamples := [4]int{int(e.Float("S1", sampleChordG)), int(e.Float("S2", sampleChordG)), int(e.Float("S3", sampleChordG)), int(e.Float("S4", sampleChordA))}
	sPitch := [4]int{int(e.Float("pS1", 0)), int(e.Float("pS2", 0)), int(e.Float("pS3", 0)), int(e.Float("pS4", 0))}
	m.ctx.At(beat+2, func() { m.jj.prepareTogether(true); m.soshi.prepareTogether(m.ctx.App.Autoplay) })
	offsets := []struct{ start, mute float64 }{{3, 4}, {4.5, 5.5}, {6, 6.5}, {7, 10}}
	for i, off := range offsets {
		i, off := i, off
		m.ctx.At(beat+off.start, func() {
			var p [6]int
			m.jj.strumStrings(false, p, sampleAt(jjSamples[i]), jjPitch[i], false, i == 3, false)
		})
		m.ctx.At(beat+off.mute, func() { m.jj.mute(true, false) })
		part := riffPart{sample: sSamples[i], sampleTone: sPitch[i]}
		m.scheduleSoshiRiff(beat+off.start, off.mute-off.start, part, i == 3)
	}
}

func (m *Module) defaultLastOne(e *riq.Entity) {
	beat := e.Beat
	m.ctx.SoundAt(beat, "LastOne", 1)
	if boolParamDefault(e, "moveCamera", true) {
		m.addCameraMove(beat+2, 0)
	}
	jjSamples := [3]int{int(e.Float("JJ1", sampleChordAsus4)), int(e.Float("JJ2", sampleChordAsus4)), int(e.Float("JJ3", sampleChordAsus4))}
	jjPitch := [3]int{int(e.Float("pJJ1", 0)), int(e.Float("pJJ2", 0)), int(e.Float("pJJ3", 0))}
	sSamples := [3]int{int(e.Float("S1", sampleChordDmaj9)), int(e.Float("S2", sampleChordDmaj9)), int(e.Float("S3", sampleChordDmaj9))}
	sPitch := [3]int{int(e.Float("pS1", 0)), int(e.Float("pS2", 0)), int(e.Float("pS3", 0))}
	m.ctx.At(beat+2, func() { m.jj.prepareTogether(true); m.soshi.prepareTogether(m.ctx.App.Autoplay) })
	offsets := []struct{ start, mute float64 }{{3, 3.5}, {4.5, 5}, {6, 6.5}}
	for i, off := range offsets {
		i, off := i, off
		m.ctx.At(beat+off.start, func() {
			var p [6]int
			m.jj.strumStrings(false, p, sampleAt(jjSamples[i]), jjPitch[i], false, false, false)
		})
		m.ctx.At(beat+off.mute, func() { m.jj.mute(true, false) })
		part := riffPart{sample: sSamples[i], sampleTone: sPitch[i]}
		m.scheduleSoshiRiff(beat+off.start, off.mute-off.start, part, false)
	}
}

func (m *Module) prepareTogether(ev togetherPrepare) {
	if ev.voice != voiceNone {
		name := "LastOne"
		if ev.voice == voiceCmon {
			name = "Cmon"
		}
		m.ctx.SoundAt(ev.beat, name, 1)
	}
	m.ctx.At(ev.beat+ev.middleBeat, func() {
		force := ev.middleBeat == ev.muteBeat
		m.jj.prepareTogether(force)
		m.soshi.prepareTogether(force && m.ctx.App.Autoplay)
	})
	if ev.middleBeat != ev.muteBeat {
		m.ctx.At(ev.beat+ev.muteBeat, func() { m.muteWho(whoBoth) })
	}
	for _, r := range m.togetherBetween(ev.beat, m.ctx.NextSwitchBeat(ev.beat)) {
		r := r
		m.ctx.At(r.beat, func() {
			m.jj.strumStrings(r.jj.gleeClub, r.jj.pitches, sampleAt(r.jj.sample), r.jj.sampleTone, false, r.togetherEnd, false)
		})
		m.ctx.At(r.beat+r.length, func() { m.jj.mute(true, false) })
		m.scheduleSoshiRiff(r.beat, r.length, r.soshi, r.togetherEnd)
		if r.togetherEnd {
			break
		}
	}
}

func (m *Module) addCameraMove(start, target float64) {
	m.cams = append(m.cams, cameraMove{start: start, to: target})
}

func (m *Module) cameraX(beat float64) float64 {
	x := 0.0
	for _, mv := range m.cams {
		if beat < mv.start {
			break
		}
		if beat <= mv.start+1 {
			return engine.Ease(4, mv.from, mv.to, beat-mv.start)
		}
		x = mv.to
	}
	return x
}

func (m *Module) riffsBetween(start, end float64) []riffEvent {
	out := []riffEvent{}
	for _, r := range m.riffs {
		if r.beat >= start && r.beat < end {
			out = append(out, r)
		}
	}
	return out
}

func (m *Module) bendsBetween(start, end float64) []bendEvent {
	out := []bendEvent{}
	for _, b := range m.bends {
		if b.beat >= start && b.beat < end {
			out = append(out, b)
		}
	}
	return out
}

func (m *Module) togetherBetween(start, end float64) []riffEvent {
	out := []riffEvent{}
	for _, r := range m.together {
		if r.beat > start && r.beat < end {
			out = append(out, r)
		}
	}
	return out
}

func (m *Module) lastIntervalBefore(beat float64) intervalEvent {
	var out intervalEvent
	for _, iv := range m.ivals {
		if iv.beat <= beat {
			out = iv
		}
	}
	return out
}

func parseRiff(e *riq.Entity, togetherEnd bool) riffEvent {
	return riffEvent{
		beat: e.Beat, length: defaultLength(e.Length, 1), respond: boolParamDefault(e, "respond", true),
		jj: riffPart{
			pitches:    pitches(e, "JJ"),
			gleeClub:   boolParam(e, "gcJJ"),
			sample:     int(e.Float("sampleJJ", sampleChordA)),
			sampleTone: int(e.Float("pitchSampleJJ", 0)),
		},
		soshi: riffPart{
			pitches:    pitches(e, "S"),
			gleeClub:   boolParam(e, "gcS"),
			sample:     int(e.Float("sampleS", sampleChordA)),
			sampleTone: int(e.Float("pitchSampleS", 0)),
		},
		togetherEnd: togetherEnd,
	}
}

func pitches(e *riq.Entity, suffix string) [6]int {
	return [6]int{
		int(e.Float("1"+suffix, -1)), int(e.Float("2"+suffix, -1)), int(e.Float("3"+suffix, -1)),
		int(e.Float("4"+suffix, -1)), int(e.Float("5"+suffix, -1)), int(e.Float("6"+suffix, -1)),
	}
}

func defaultLength(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func itoa1(v int) string {
	if v < 0 {
		return "0"
	}
	if v < 10 {
		return string(rune('0' + v))
	}
	return "10"
}
