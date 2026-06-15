package drummerduel

import (
	"math"

	"hsdemo/engine"
)

var cheerPatterns = map[int][]string{
	1: {"Cheer1R", "Cheer1R", "Cheer1L", "Cheer1L"},
	2: {"Cheer2Ra", "Cheer2Rb", "Cheer2La", "Cheer2Lb"},
	3: {"Cheer1R", "Cheer1L", "Cheer1R", "Cheer1L"},
	4: {"Cheer1M", "Cheer1M", "Cheer1M", "Cheer1M"},
}

func (m *Module) scheduleBops() {
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.goBopCheer = ev.cheerAuto
			m.goBopDrummer = ev.drummerAuto
			m.allowBopCheer = true
		})
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() {
				if ev.cheer {
					m.cheerBop(bb)
				}
				if ev.drummer {
					m.drummerBop(bb)
				}
			})
		}
	}
}

func (m *Module) scheduleInterval(ev intervalEvt) {
	allDon := m.intervalAllDon(ev.beat, ev.beat+ev.length)
	m.ctx.At(ev.beat, func() {
		m.allDon = allDon
		m.isRight = true
	})
	if ev.camMove {
		m.ctx.At(ev.beat-1, func() { m.moveCamera(ev.beat-1, autoCamLength, camLeft, 0) })
	}
	m.ctx.At(ev.beat-1, func() {
		m.playReferee("Left", ev.beat-1)
		m.playReferee("HeadNormal", ev.beat-1)
	})
	m.ctx.At(ev.beat, func() { m.playReferee("HeadNormal", ev.beat) })

	m.scheduleCheerPattern(ev.beat, ev.length, ev.pattern, false)
	m.ctx.At(ev.beat+ev.length-1, func() { m.cheerAll(m.cheerLeft, "Just", ev.beat+ev.length-1) })

	hits := m.hitsBetween(ev.beat, ev.beat+ev.length)
	for i, hit := range hits {
		hit := hit
		next := math.Inf(1)
		if i+1 < len(hits) {
			next = hits[i+1].beat
		}
		voice := hitVoice(hit.beat, hit.hitSound, next)
		m.ctx.SoundAt(hit.beat, "drumLeftHit", 1)
		m.ctx.SoundAt(hit.beat, voice, 1)
		m.ctx.At(hit.beat, func() { m.drumHit(hit.beat, false) })
	}
	if ev.auto {
		pass := passEvt{beat: ev.beat + ev.length, length: 1}
		m.ctx.At(pass.beat, func() { m.passTurn(pass, ev) })
	}
}

func (m *Module) passTurnStandalone(pass passEvt) {
	interval, ok := m.lastIntervalBefore(pass.beat)
	if !ok {
		return
	}
	m.passTurn(pass, interval)
}

func (m *Module) passTurn(pass passEvt, interval intervalEvt) {
	intervalEnd := interval.beat + interval.length
	intervalLen := pass.beat - interval.beat
	if intervalLen <= 0 {
		intervalLen = interval.length
	}
	m.isRight = true
	if interval.camMove {
		m.moveCamera(pass.beat, autoCamLength, camRight, 0)
	}
	m.ctx.Sound("passToRight")
	m.playReferee("Right", pass.beat)
	m.hasMissed = false
	m.isDrumming = true

	hits := m.hitsBetween(interval.beat, intervalEnd)
	for i, hit := range hits {
		hit := hit
		relative := hit.beat - interval.beat
		target := pass.beat + pass.length + relative
		next := math.Inf(1)
		if i+1 < len(hits) {
			next = hits[i+1].beat - interval.beat + pass.beat + pass.length
		}
		voice := hitVoice(target, hit.hitSound, next)
		front := voice == "drummerKo"
		m.ctx.SoundAt(target, voice, 1)
		m.ctx.ScheduleInputAny(target,
			func(state float64, _ engine.Judgment) { m.drummerJust(state, front) },
			func() { m.hasMissed = true },
		)
	}

	successBeat := intervalEnd + intervalLen
	m.scheduleDrummerSuccess(successBeat, interval.successVoice)
	m.scheduleCheerPattern(pass.beat+pass.length, interval.length, interval.pattern, true)
}

func (m *Module) scheduleDrummerSuccess(beat float64, successVoice bool) {
	m.ctx.At(beat, func() {
		if !m.hasMissed {
			m.playReferee("HeadGood", beat)
			m.cheerAll(m.cheerRight, "Just", beat)
		} else {
			m.playReferee("HeadBad", beat)
			m.ctx.At(beat+0.5, func() { m.cheerAll(m.cheerRight, "Miss", beat+0.5) })
		}
		m.playReferee("Prepare", beat)
		if successVoice {
			if m.hasMissed {
				m.ctx.Sound("passToLeftBad")
				m.ctx.SoundAt(beat+0.5, "miss", 1)
			} else {
				m.ctx.Sound("passToLeft")
			}
		}
	})
	m.ctx.At(beat+1, func() { m.isDrumming = false })
}

func (m *Module) scheduleCheerPattern(beat, span float64, pattern int, turnover bool) {
	p, ok := cheerPatterns[pattern]
	if !ok || pattern < 1 {
		return
	}
	count := int(span)
	if !turnover {
		count--
	}
	side := m.cheerLeft
	if turnover {
		side = m.cheerRight
	}
	for i := 0; i < count; i++ {
		anim := p[i%len(p)]
		b := beat + float64(i)
		m.ctx.At(b, func() { m.cheerAll(side, anim, b) })
	}
}

func (m *Module) scheduleChant(ev chantEvt) {
	switch ev.typ {
	case 0:
		m.ctx.SoundAt(ev.beat, "one", 1)
		m.ctx.SoundAt(ev.beat+1, "two", 1)
	case 1:
		m.ctx.SoundAt(ev.beat, "grunt", 1)
	case 2:
		m.ctx.SoundAt(ev.beat, "angry", 1)
	}
}

func (m *Module) scheduleEndPose(beat float64) {
	m.ctx.At(beat, func() {
		m.allowBopCheer = false
		m.goBopDrummer = false
		m.playReferee("HeadNormal", beat)
		m.playReferee("Finish", beat)
		m.cheerAll(m.cheerLeft, "Cheer1M", beat)
		m.cheerAll(m.cheerRight, "Cheer1M", beat)
	})
	m.ctx.At(beat+2, func() {
		m.playReferee("Finish", beat+2)
		m.cheerAll(m.cheerLeft, "Cheer1M", beat+2)
		m.cheerAll(m.cheerRight, "Cheer1M", beat+2)
	})
	m.ctx.At(beat+3, func() {
		m.allowBopCheer = true
		m.goBopDrummer = true
	})
}

func (m *Module) drumHit(beat float64, player bool) {
	m.isRight = !m.isRight
	drummer, taiko := m.drummerL, m.taikoL
	if player {
		drummer, taiko = m.drummerR, m.taikoR
	}
	isFront := !m.isRight
	m.playDrummer(drummer, "Hit", beat)
	m.playTaiko(taiko, "Hit", beat)
	if m.allDon {
		if isFront {
			m.playDrummer(drummer, m.armState("ArmF"), beat)
		} else {
			m.playDrummer(drummer, m.armState("ArmB"), beat)
		}
		return
	}
	if !player {
		if isIntegralBeat(beat) {
			m.playDrummer(drummer, m.armState("ArmF"), beat)
		} else {
			m.playDrummer(drummer, m.armState("ArmB"), beat)
		}
	}
}

func (m *Module) drummerJust(state float64, front bool) {
	beat := m.ctx.Beat()
	if state >= 1 || state <= -1 {
		m.ctx.Sound("nearMiss")
		m.playDrummer(m.drummerR, "FaceWhiff", beat)
		m.playTaiko(m.taikoR, "Whiff", beat)
		if m.isAngry {
			m.isWhiffing = true
			m.applyDrummerHeadColors()
		}
	} else {
		m.restoreColorsFromWhiff()
		m.playDrummer(m.drummerR, m.faceState(), beat)
		m.playTaiko(m.taikoR, "Hit", beat)
	}
	m.ctx.Sound("drumRightHit")
	if m.allDon {
		m.isRight = !m.isRight
		m.drumHit(beat, true)
		return
	}
	m.playDrummer(m.drummerR, "Hit", beat)
	if front {
		m.playDrummer(m.drummerR, m.armState("ArmF"), beat)
	} else {
		m.playDrummer(m.drummerR, m.armState("ArmB"), beat)
	}
}

func (m *Module) updateBeatPulse(beat float64) {
	p := int(math.Floor(beat + 1e-6))
	if p <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= p; b++ {
		if b >= 0 {
			bb := float64(b)
			if m.goBopCheer && m.allowBopCheer {
				m.cheerBop(bb)
			}
			if m.goBopDrummer {
				m.drummerBop(bb)
			}
		}
	}
	m.lastPulse = p
}

func (m *Module) cheerBop(beat float64) {
	if !m.statePlaying(m.referee, beat, "Fail", "Finish", "Left", "Prepare", "Right") {
		m.playReferee("Bop", beat)
	}
	for _, p := range append(append([]string{}, m.cheerLeft...), m.cheerRight...) {
		if !m.statePlaying(p, beat, "Just", "Miss", "Cheer1L", "Cheer1R", "Cheer1M", "Cheer2La", "Cheer2Lb", "Cheer2Ra", "Cheer2Rb") {
			m.ctx.Scene.PlayState(p, "Bop", beat, 0.5)
		}
	}
}

func (m *Module) drummerBop(beat float64) {
	for _, p := range []string{m.drummerL, m.drummerR} {
		if !m.statePlaying(p, beat, "Hit", "ArmF", "HitF", "ArmB", "HitB", "ArmMadF", "ArmMadB") {
			m.playDrummer(p, "Bop", beat)
		}
	}
	if m.isWhiffing {
		m.restoreColorsFromWhiff()
	}
	m.playDrummer(m.drummerR, m.faceState(), beat)
}

func (m *Module) cheerAll(paths []string, anim string, beat float64) {
	for i, p := range paths {
		curr := anim
		if anim == "Cheer1M" {
			switch i {
			case 0:
				curr = "Cheer1R"
			case 1:
				curr = "Cheer1M"
			case 2:
				curr = "Cheer1L"
			}
		}
		m.ctx.Scene.PlayState(p, curr, beat, 0.5)
	}
}

func (m *Module) playReferee(state string, beat float64) {
	if state == "HeadNormal" || state == "HeadGood" || state == "HeadBad" {
		m.ctx.Scene.PlayStateLayer(m.referee+":head", m.referee, state, beat, 0.5)
		return
	}
	m.ctx.Scene.PlayState(m.referee, state, beat, 0.5)
}

func (m *Module) playDrummer(path, state string, beat float64) {
	switch state {
	case "FaceIdle", "FaceAngry", "FaceWhiff":
		m.ctx.Scene.PlayStateLayer(path+":face", path, state, beat, 0.5)
	case "ArmF", "ArmMadF", "HitArmF", "HitF", "HitMadF":
		m.ctx.Scene.PlayStateLayer(path+":armF", path, state, beat, 0.5)
	case "ArmB", "ArmMadB", "HitArmB", "HitB", "HitMadB":
		m.ctx.Scene.PlayStateLayer(path+":armB", path, state, beat, 0.5)
	default:
		m.ctx.Scene.PlayState(path, state, beat, 0.5)
	}
}

func (m *Module) playTaiko(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) statePlaying(path string, beat float64, names ...string) bool {
	state, playing := m.ctx.Scene.StateInfo(path, beat)
	if !playing {
		return false
	}
	for _, n := range names {
		if state == n {
			return true
		}
	}
	return false
}

func (m *Module) setAnger(beat float64, angry bool) {
	m.isAngry = angry
	m.applyDrummerHeadColors()
	m.playDrummer(m.drummerL, m.faceState(), beat)
	m.playDrummer(m.drummerR, m.faceState(), beat)
}

func (m *Module) faceState() string {
	if m.isAngry {
		return "FaceAngry"
	}
	return "FaceIdle"
}

func (m *Module) armState(base string) string {
	if !m.isAngry {
		return base
	}
	switch base {
	case "ArmF":
		return "ArmMadF"
	case "ArmB":
		return "ArmMadB"
	}
	return base
}

func (m *Module) restoreColorsFromWhiff() {
	if !m.isWhiffing {
		return
	}
	m.isWhiffing = false
	m.applyDrummerHeadColors()
}

func (m *Module) setNPCs(cheer, referee, platform bool) {
	m.ctx.Scene.SetActive(m.cheerObj, cheer)
	m.ctx.Scene.SetActive(m.refObj, referee)
	m.ctx.Scene.SetActive(m.platform, platform)
}

func (m *Module) hitsBetween(start, end float64) []hitEvt {
	var out []hitEvt
	for _, hit := range m.hits {
		if hit.beat >= start-1e-6 && hit.beat <= end+1e-6 {
			out = append(out, hit)
		}
	}
	return out
}

func (m *Module) intervalAllDon(start, end float64) bool {
	for _, hit := range m.hitsBetween(start, end) {
		if !isIntegralBeat(hit.beat) {
			return false
		}
	}
	return true
}

func (m *Module) lastIntervalBefore(beat float64) (intervalEvt, bool) {
	var out intervalEvt
	ok := false
	for _, ev := range m.intervals {
		if ev.beat > beat {
			break
		}
		out, ok = ev, true
	}
	return out, ok
}

func hitVoice(beat float64, hitSound int, nextBeat float64) string {
	if hitSound == hitAuto {
		switch {
		case !isIntegralBeat(beat):
			hitSound = hitKo
		case nextBeat-beat < 1:
			hitSound = hitDo
		default:
			hitSound = hitDon
		}
	}
	switch hitSound {
	case hitDo:
		return "drummerDo"
	case hitKo:
		return "drummerKo"
	default:
		return "drummerDon"
	}
}

func isIntegralBeat(beat float64) bool {
	return math.Abs(beat-math.Round(beat)) < 1e-6
}
