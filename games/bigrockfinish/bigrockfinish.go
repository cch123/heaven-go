// Package bigrockfinish ports Big Rock Finish's four-hit guitar finish,
// layered ghost/crowd animations, spotlight/flash cues, drums, pitched guitar
// sustain, and audience reactions.
package bigrockfinish

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
	animScale       = 0.5
	guitarFadeSec   = 0.1
	guitarWhiffFade = 0.25
	actionFlick     = 1
)

const (
	drumKick = iota
	drumSnare
	drumTomLeft
	drumTomRight
	drumHiHat
	drumCymbal
)

const (
	spotRed = iota
	spotGreen
	spotBlue
	spotNone
)

type bopEvt struct {
	beat, length                 float64
	booboo, ecto, crowd, bopAuto bool
	bop                          bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	player, ecto, spooky string
	handL, handR         string
	audience             string
	spotlight, flash     string
	bass, cymbal         string
	tomL, tomR           string
	snare, hihat         string
	unlitArea            string

	bops []bopEvt

	ghostCanBop, ectoCanBop, crowdCanBop bool
	playerPrepare, ectoPrepare           bool
	playerStrum, ectoStrum               bool
	crowdReact, crowdIsReacting          bool
	flashOn                              bool
	bopAuto                              bool

	pitches    [4]int
	whiffPitch int
	timeScale  float64
	misses     int
	lastPulse  int
}

func New() engine.Module {
	return &Module{
		ghostCanBop: true, ectoCanBop: true, crowdCanBop: true,
		crowdReact: true, flashOn: true, timeScale: 1,
		lastPulse: math.MinInt,
	}
}

func (m *Module) ID() string { return "bigRockFinish" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("bigRockFinish"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.player = roleOr(ctx, "playerGhost", "PlayerGhostHolder")
	m.ecto = roleOr(ctx, "greenGhost", "GreenGhostHolder")
	m.spooky = roleOr(ctx, "drummerGhost", "RedGhostHolder")
	m.handL = roleOr(ctx, "ghostHandL", "RedGhostHolder/Ghost/ArmLeft")
	m.handR = roleOr(ctx, "ghostHandR", "RedGhostHolder/Ghost/ArmRight")
	m.audience = roleOr(ctx, "audience", "AudienceHolder")
	m.spotlight = roleOr(ctx, "spotlightMask", "BackgroundHolder/SpotlightMask")
	m.flash = roleOr(ctx, "flash", "Flash")
	m.bass = roleOr(ctx, "Bass", "DrumHolder/Bass")
	m.cymbal = roleOr(ctx, "Cymbal", "DrumHolder/Cymbal")
	m.tomL = roleOr(ctx, "TomL", "DrumHolder/TomLeft")
	m.tomR = roleOr(ctx, "TomR", "DrumHolder/TomRight")
	m.snare = roleOr(ctx, "Snare", "DrumHolder/Snare")
	m.hihat = roleOr(ctx, "Hihat", "DrumHolder/Hi-Hat")
	m.unlitArea = roleOr(ctx, "UnlitArea", "BackgroundHolder/Black")

	m.initAnimators(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) initAnimators(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, root := range []string{
		m.player, m.ecto, m.spooky, m.handL, m.handR, m.audience,
		m.spotlight, m.bass, m.cymbal, m.tomL, m.tomR, m.snare, m.hihat,
	} {
		m.ctx.Scene.PlayDefaultState(root, beat, sec)
	}
	m.spotlightAnim(beat, spotNone)
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "bigRockFinish/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			booboo:  boolDefault(e, "booboo", true),
			ecto:    boolDefault(e, "ecto", true),
			crowd:   boolDefault(e, "crowd", true),
			bop:     boolDefault(e, "bop", true),
			bopAuto: boolDefault(e, "auto", false),
		}
		if ev.length <= 0 {
			ev.length = 1
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() { m.applyBop(ev) })
		if ev.bop {
			for i := 0; i < int(ev.length); i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.bopAll() })
			}
		}
	case "bigRockFinish/drum":
		beat := e.Beat
		drum := int(e.Float("drumType", drumKick))
		m.ctx.At(beat, func() { m.playDrums(beat, drum) })
	case "bigRockFinish/spotlight":
		beat := e.Beat
		typ := int(e.Float("type", spotNone))
		m.ctx.At(beat, func() { m.spotlightAnim(beat, typ) })
	case "bigRockFinish/countin1":
		beat, length := e.Beat, e.Length
		if length <= 0 {
			length = 4
		}
		m.countin(beat, length,
			int(e.Float("note1", 0)), int(e.Float("note2", 0)),
			int(e.Float("note3", 0)), int(e.Float("note4", 0)),
			boolDefault(e, "alt", true), int(e.Float("whiff", 0)),
			boolDefault(e, "cheer", true), boolDefault(e, "crowdBop", true),
			boolDefault(e, "flash", true))
	case "bigRockFinish/countin2":
		beat, length := e.Beat, e.Length
		if length <= 0 {
			length = 8
		}
		m.countinLong(beat, length,
			int(e.Float("note1", 0)), int(e.Float("note2", 0)),
			int(e.Float("note3", 0)), int(e.Float("note4", 0)),
			boolDefault(e, "short", false), boolDefault(e, "alt", false),
			int(e.Float("whiff", 0)), boolDefault(e, "cheer", true),
			boolDefault(e, "crowdBop", true), boolDefault(e, "flash", true))
	case "bigRockFinish/thankyou":
		beat := e.Beat
		alt := boolDefault(e, "alt", false)
		m.ctx.At(beat, func() { m.thankYou(beat, alt) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.initAnimators(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Update(_, beat float64) {
	m.updateBeatPulse(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.NRGBA{0x01, 0x9b, 0x95, 0xff})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action != actionFlick {
		return
	}
	m.playerPrepare = false
	m.playerStrum = true
	m.ctx.Scene.PlayState(m.player, "Strum", beat, animScale)
	m.ctx.SoundLoopPitchVolUntil("guitar", semitonePitch(m.whiffPitch), 1, beat, guitarWhiffFade)
	m.ctx.At(beat+1, func() { m.playerStrum = false })
}

func (m *Module) updateBeatPulse(beat float64) {
	p := int(math.Floor(beat + 1e-6))
	if p <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= p; b++ {
		if b >= 0 && m.bopAuto {
			m.bopAll()
		}
	}
	m.lastPulse = p
}

func (m *Module) applyBop(ev bopEvt) {
	m.ghostCanBop = ev.booboo
	m.ectoCanBop = ev.ecto
	m.crowdCanBop = ev.crowd
	m.bopAuto = ev.bopAuto
	m.playerPrepare, m.playerStrum = false, false
	m.ectoPrepare, m.ectoStrum = false, false
}

func (m *Module) bopAll() {
	m.bopBooBoo()
	m.bopEcto()
	m.bopCrowd()
}

func (m *Module) bopBooBoo() {
	if m.ghostCanBop && !m.playerPrepare && !m.playerStrum && m.idle(m.player) {
		m.ctx.Scene.PlayState(m.player, "Beat", m.ctx.Beat(), animScale)
	}
}

func (m *Module) bopEcto() {
	if m.ectoCanBop && !m.ectoPrepare && !m.ectoStrum && m.idle(m.ecto) {
		m.ctx.Scene.PlayState(m.ecto, "Beat", m.ctx.Beat(), animScale)
	}
}

func (m *Module) bopCrowd() {
	if m.crowdCanBop && !m.crowdIsReacting && m.idle(m.audience) {
		m.ctx.Scene.PlayState(m.audience, "Beat", m.ctx.Beat(), animScale)
	}
}

func (m *Module) idle(root string) bool {
	st, playing := m.ctx.Scene.StateInfo(root, m.ctx.Beat())
	return st == "" || st == "Idle" || !playing
}

func (m *Module) countin(beat, length float64, note1, note2, note3, note4 int, alt bool, whiff int, cheer, crowdBop, flash bool) {
	noteTwo := length + length*0.1875
	noteThree := length + length*0.375
	noteFour := length + length*0.5
	m.doAutoPrepare(beat+length+length*0.125, beat+noteTwo+length*0.125, beat+noteThree+length*0.0625)
	m.whiffPitch = whiff

	if alt {
		m.soundAt(beat, "countinOneFast")
		m.soundAt(beat+length*0.25, "countinTwoFast")
		m.soundAt(beat+length*0.5, "countinThreeFast")
		m.soundAt(beat+length*0.75, "comeOn")
	} else {
		m.soundAt(beat, "countinOne")
		m.soundAt(beat+length*0.25, "countinTwo")
		m.soundAt(beat+length*0.5, "countinThree")
		m.soundAt(beat+length*0.75, "countinFour")
	}

	m.scheduleCountinActors(beat, length, length*0.75, note1, note2, note3, note4, cheer, crowdBop, flash)
	m.scheduleStrumInputs(beat, length, noteTwo, noteThree, noteFour)
}

func (m *Module) countinLong(beat, length float64, note1, note2, note3, note4 int, short, alt bool, whiff int, cheer, crowdBop, flash bool) {
	noteTwo := length + length*0.1875
	noteThree := length + length*0.375
	noteFour := length + length*0.5
	m.doAutoPrepare(beat+length+length*0.125, beat+noteTwo+length*0.125, beat+noteThree+length*0.0625)
	m.whiffPitch = whiff
	m.scheduleStrumInputs(beat, length, noteTwo, noteThree, noteFour)

	if alt {
		if !short {
			m.soundAt(beat, "countinOneFast")
			m.soundAt(beat+length*0.25, "countinTwoFast")
		}
		m.soundAt(beat+length*0.5, "countinOneFast")
		m.soundAt(beat+length*0.625, "countinTwoFast")
		m.soundAt(beat+length*0.75, "countinThreeFast")
		m.soundAt(beat+length*0.875, "comeOn")
	} else {
		if !short {
			m.soundAt(beat, "countinOne")
			m.soundAt(beat+length*0.25, "countinTwo")
		}
		m.soundAt(beat+length*0.5, "countinOne")
		m.soundAt(beat+length*0.625, "countinTwo")
		m.soundAt(beat+length*0.75, "countinThree")
		m.soundAt(beat+length*0.875, "countinFour")
	}

	m.scheduleCountinActors(beat, length, length*0.875, note1, note2, note3, note4, cheer, crowdBop, flash)
}

func (m *Module) scheduleCountinActors(beat, length, prepOffset float64, note1, note2, note3, note4 int, cheer, crowdBop, flash bool) {
	interval := beat + length
	noteTwo := length + length*0.1875
	noteThree := length + length*0.375
	noteFour := length + length*0.5

	m.ctx.At(beat+prepOffset, func() {
		m.ctx.Scene.PlayState(m.ecto, "Prepare", beat+prepOffset, animScale)
		m.ectoPrepare = true
		m.doPrepare(beat + prepOffset)
		m.pitches = [4]int{note1, note2, note3, note4}
		m.timeScale = length / 8
		m.crowdReact = cheer
		m.crowdIsReacting = crowdBop
		m.flashOn = flash
	})
	m.ctx.At(interval, func() { m.ectoStrumAt(interval) })
	m.ctx.At(interval+length*0.125, func() { m.ectoPrepareAt(interval + length*0.125) })
	m.ctx.At(beat+noteTwo, func() { m.ectoStrumAt(beat + noteTwo) })
	m.ctx.At(beat+noteTwo+length*0.125, func() { m.ectoPrepareAt(beat + noteTwo + length*0.125) })
	m.ctx.At(beat+noteThree, func() { m.ectoStrumAt(beat + noteThree) })
	m.ctx.At(beat+noteThree+length*0.0625, func() { m.ectoPrepareAt(beat + noteThree + length*0.0625) })
	m.ctx.At(beat+noteFour, func() {
		m.ctx.Scene.PlayState(m.ecto, "Jump", beat+noteFour, animScale)
		m.ectoStrum, m.ectoPrepare = true, false
		m.checkCrowdInputBop()
	})
	m.ctx.At(beat+length*2, func() {
		m.ectoStrum, m.ectoPrepare = false, false
		m.crowdIsReacting = false
		m.playerStrum, m.playerPrepare = false, false
	})
}

func (m *Module) scheduleStrumInputs(beat, length, noteTwo, noteThree, noteFour float64) {
	targets := []float64{beat + length, beat + noteTwo, beat + noteThree, beat + noteFour}
	for i, target := range targets {
		idx := i
		if idx == len(targets)-1 {
			m.ctx.ScheduleInputAction(target, actionFlick,
				func(state float64, _ engine.Judgment) { m.strumSuccess(idx, state) },
				func() { m.strumMissLast() })
			continue
		}
		m.ctx.ScheduleInputAction(target, actionFlick,
			func(state float64, _ engine.Judgment) { m.strumSuccess(idx, state) },
			func() { m.strumMiss() })
	}
}

func (m *Module) doAutoPrepare(beats ...float64) {
	for _, b := range beats {
		beat := b
		m.ctx.At(beat, func() { m.doPrepare(beat) })
	}
}

func (m *Module) doPrepare(beat float64) {
	m.playerPrepare, m.playerStrum = true, false
	m.ctx.Scene.PlayState(m.player, "Prepare", beat, animScale)
}

func (m *Module) ectoStrumAt(beat float64) {
	m.ctx.Scene.PlayState(m.ecto, "Strum", beat, animScale)
	m.ectoStrum, m.ectoPrepare = true, false
	m.checkCrowdInputBop()
}

func (m *Module) ectoPrepareAt(beat float64) {
	m.ctx.Scene.PlayState(m.ecto, "Prepare", beat, animScale)
	m.ectoStrum, m.ectoPrepare = false, true
}

func (m *Module) checkCrowdInputBop() {
	if m.crowdIsReacting {
		m.ctx.Scene.PlayState(m.audience, "Beat", m.ctx.Beat(), animScale)
	}
}

func (m *Module) checkInputFlash() {
	if m.flashOn {
		m.ctx.Scene.Play(m.flash, "Flash/Flash", m.ctx.Beat(), animScale)
	}
}

func (m *Module) strumSuccess(idx int, state float64) {
	m.playerStrum, m.ectoStrum = true, true
	m.playerPrepare, m.ectoPrepare = false, false
	beat := m.ctx.Beat()
	ng := state <= -1 || state >= 1
	m.ctx.Scene.PlayState(m.player, strumState(idx), beat, animScale)

	pitch := semitonePitch(m.pitches[idx])
	if ng {
		pitch += 0.01
	}
	stopBeat := guitarStopBeat(beat, m.timeScale, idx)
	m.playGuitarUntil(pitch, stopBeat)
	if !ng {
		if idx == 3 {
			m.ctx.Sound("yeahB")
		} else {
			m.ctx.Sound("yeahA")
		}
	}
	m.ctx.Sound("cymbal")
	if idx != 1 || !ng {
		m.checkInputFlash()
	}
	if idx == 3 {
		m.finishCrowdReaction(beat)
	}
}

func strumState(idx int) string {
	if idx == 3 {
		return "Jump"
	}
	return "Strum"
}

func (m *Module) playGuitarUntil(pitch, stopBeat float64) {
	m.ctx.SoundLoopPitchVolUntil("guitar", pitch, 1, stopBeat, guitarFadeSec)
}

func guitarStopBeat(hitBeat, timeScale float64, idx int) float64 {
	if idx == 3 {
		return hitBeat + timeScale*4
	}
	return hitBeat + timeScale*0.5
}

func (m *Module) finishCrowdReaction(hitBeat float64) {
	if !m.crowdReact {
		return
	}
	if m.misses == 0 {
		m.soundAt(hitBeat+0.5, "cheering")
		m.ctx.At(hitBeat, func() { m.ctx.Scene.PlayState(m.audience, "Beat", hitBeat, animScale) })
		m.ctx.At(hitBeat+m.timeScale, func() { m.ctx.Scene.PlayState(m.audience, "Cheer", hitBeat+m.timeScale, animScale) })
		m.ctx.At(hitBeat+5*m.timeScale, func() { m.ctx.Scene.PlayState(m.audience, "Idle", hitBeat+5*m.timeScale, animScale) })
		return
	}
	m.soundAt(hitBeat+0.5, "boo")
	m.ctx.At(hitBeat+0.5, func() { m.ctx.Scene.PlayState(m.audience, "Miss", hitBeat+0.5, animScale) })
}

func (m *Module) strumMiss() {
	beat := m.ctx.Beat()
	m.ctx.Scene.PlayState(m.player, "Release", beat, animScale)
	m.misses++
}

func (m *Module) strumMissLast() {
	beat := m.ctx.Beat()
	m.ctx.Scene.PlayState(m.player, "Release", beat, animScale)
	m.misses++
	if m.crowdReact {
		m.ctx.Sound("boo")
		m.ctx.At(beat+0.5, func() { m.ctx.Scene.PlayState(m.audience, "Miss", beat+0.5, animScale) })
	}
}

func (m *Module) thankYou(beat float64, alt bool) {
	if m.misses > 0 {
		m.ctx.SoundAt(beat, "cough", 1)
	} else if alt {
		m.ctx.SoundAt(beat, "thankYouB", 1)
	} else {
		m.ctx.SoundAt(beat, "thankYouA", 1)
	}
	m.misses = 0
}

func (m *Module) soundAt(beat float64, name string) {
	m.ctx.SoundAt(beat, name, 1)
}

func (m *Module) playDrums(beat float64, drum int) {
	switch drum {
	case drumKick:
		m.ctx.Scene.PlayState(m.spooky, "Beat", beat, animScale)
		m.ctx.Scene.PlayState(m.bass, "Hit", beat, animScale)
	case drumSnare:
		m.ctx.Scene.PlayState(m.handL, "Hit01", beat, animScale)
		m.ctx.Scene.PlayState(m.snare, "Hit", beat, animScale)
	case drumTomLeft:
		m.ctx.Scene.PlayState(m.handL, "Hit01", beat, animScale)
		m.ctx.Scene.PlayState(m.tomL, "Hit", beat, animScale)
	case drumTomRight:
		m.ctx.Scene.PlayState(m.handR, "Hit01", beat, animScale)
		m.ctx.Scene.PlayState(m.tomR, "Hit", beat, animScale)
	case drumCymbal:
		m.ctx.Scene.PlayState(m.handR, "Hit02", beat, animScale)
		m.ctx.Scene.PlayState(m.cymbal, "Hit", beat, animScale)
	case drumHiHat:
		m.ctx.Scene.PlayState(m.handL, "Hit02", beat, animScale)
		m.ctx.Scene.PlayState(m.hihat, "Hit", beat, animScale)
	}
}

func (m *Module) spotlightAnim(beat float64, typ int) {
	switch typ {
	case spotRed:
		m.ctx.Scene.PlayState(m.spotlight, "Red", beat, animScale)
		m.ctx.Scene.SetColorOver(m.unlitArea, [4]float64{1, 0, 0, 0.4})
	case spotGreen:
		m.ctx.Scene.PlayState(m.spotlight, "Green", beat, animScale)
		m.ctx.Scene.SetColorOver(m.unlitArea, [4]float64{0, 1, 0, 0.4})
	case spotBlue:
		m.ctx.Scene.PlayState(m.spotlight, "Blue", beat, animScale)
		m.ctx.Scene.SetColorOver(m.unlitArea, [4]float64{0, 0, 1, 0.4})
	default:
		m.ctx.Scene.PlayState(m.spotlight, "None", beat, animScale)
		m.ctx.Scene.SetColorOver(m.unlitArea, [4]float64{0, 0, 0, 0.4})
	}
}

func semitonePitch(semitone int) float64 {
	return math.Exp2(float64(semitone) / 12)
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	d := 0.0
	if def {
		d = 1
	}
	return e.Float(key, d) != 0
}
