// Package clappytrio ports The Clappy Trio's dynamic lion line, clap relay,
// sign movement/text, and emotion bops.
package clappytrio

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var bgColor = color.NRGBA{R: 0xde, G: 0xff, B: 0xff, A: 0xff}

const (
	minLions = 3
	maxLions = 8

	lionStartX = -3.066667
	lionMaxW   = 12.266668
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
	disableEmo   bool
}

type clapEvt struct {
	beat, length float64
}

type prepareEvt struct {
	beat float64
	typ  int
}

type signEvt struct {
	beat, length float64
	ease         int
	down         bool
}

type textEvt struct {
	beat   float64
	custom bool
	text   string
}

type countEvt struct {
	beat  float64
	count int
}

type lion struct {
	inst *kart.Instance
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	lionT *kart.Template
	lions []*lion
	faces []string

	baseLionPath string
	signPath     string
	trioTextPath string
	customPath   string

	bops     []bopEvt
	claps    []clapEvt
	prepares []prepareEvt
	signs    []signEvt
	texts    []textEvt
	counts   []countEvt

	lionCount int
	endBeat   float64

	misses    int
	shouldBop bool
	canBop    bool
	doEmotion bool
	emoCount  int

	clapStarted bool
	canHit      bool
	signState   string
}

func New() engine.Module {
	return &Module{lionCount: 3, shouldBop: true, canBop: true, doEmotion: true}
}

func (m *Module) ID() string { return "clappyTrio" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("clappyTrio"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	if refs := game.RefArrays["Lion"]; len(refs) > 0 {
		m.baseLionPath = refs[0]
	} else {
		m.baseLionPath = "Lion"
	}
	m.signPath = roleOr(ctx, "signAnim", "Sign")
	m.trioTextPath = roleOr(ctx, "textTrioTiming", "Sign/SignContents/trioTiming")
	m.customPath = roleOr(ctx, "textCustom", "Sign/SignContents/customText")
	m.faces = append(m.faces, game.SpriteArrays["faces"]...)
	if len(m.faces) == 0 {
		m.faces = []string{"head_1", "head_2", "head_3", "head_4", "head_5"}
	}
	m.lionT = kart.NewTemplate(ctx.Assets, m.baseLionPath)
	ctx.Scene.SetActive(m.baseLionPath, false)
	m.setLionCount(m.lionCount, 0)
	m.changeText(false, "Trio Timing!")
	ctx.Scene.PlayDefaultState(m.signPath, 0, ctx.SecPerBeat(0))
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "clappyTrio/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			bop:        boolDefault(e, "bop", true),
			auto:       boolParam(e, "autoBop"),
			disableEmo: boolParam(e, "emo"),
		})
	case "clappyTrio/clap":
		m.claps = append(m.claps, clapEvt{beat: b, length: e.Length})
	case "clappyTrio/prepare":
		typ := 0
		if boolParam(e, "toggle") {
			typ = 3
		}
		m.prepares = append(m.prepares, prepareEvt{beat: b, typ: typ})
	case "clappyTrio/sign":
		m.signs = append(m.signs, signEvt{
			beat: b, length: e.Length,
			ease: int(e.Float("ease", 0)),
			down: boolDefault(e, "down", true),
		})
	case "clappyTrio/sign text":
		m.texts = append(m.texts, textEvt{
			beat: b, custom: boolParam(e, "textToggle"),
			text: e.Str("text", "Trio Timing!"),
		})
	case "clappyTrio/change lion count":
		m.counts = append(m.counts, countEvt{beat: b, count: int(e.Float("valA", 3))})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.claps, func(i, j int) bool { return m.claps[i].beat < m.claps[j].beat })
	sort.SliceStable(m.prepares, func(i, j int) bool { return m.prepares[i].beat < m.prepares[j].beat })
	sort.SliceStable(m.signs, func(i, j int) bool { return m.signs[i].beat < m.signs[j].beat })
	sort.SliceStable(m.texts, func(i, j int) bool { return m.texts[i].beat < m.texts[j].beat })
	sort.SliceStable(m.counts, func(i, j int) bool { return m.counts[i].beat < m.counts[j].beat })

	for _, ev := range m.counts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setLionCount(ev.count, ev.beat) })
	}
	for _, ev := range m.prepares {
		ev := ev
		m.ctx.At(ev.beat, func() { m.prepare(ev.typ, ev.beat) })
	}
	for _, ev := range m.texts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.changeText(ev.custom, ev.text) })
	}
	for _, ev := range m.signs {
		ev := ev
		if ev.ease != 1 {
			m.ctx.SoundAt(ev.beat, "sign", 1)
		}
	}
	for _, ev := range m.claps {
		ev := ev
		m.ctx.At(ev.beat, func() { m.clap(ev.beat, ev.length, ev.beat) })
	}
	m.scheduleBops()
}

func (m *Module) scheduleBops() {
	for i, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.doEmotion = !ev.disableEmo
			m.shouldBop = ev.auto
		})
		if ev.bop {
			for k := 0; k < int(ev.length); k++ {
				if k == 0 && ev.auto {
					continue
				}
				bb := ev.beat + float64(k)
				m.ctx.At(bb, func() { m.bop(bb) })
				if k == int(ev.length)-1 {
					m.ctx.At(bb, func() { m.misses = 0 })
				}
			}
		}
		if ev.auto {
			end := m.autoBopEnd(i)
			for bb := ev.beat; bb < end-1e-6; bb++ {
				b := bb
				m.ctx.At(b, func() {
					if m.shouldBop && m.canBop {
						m.bop(b)
					}
				})
			}
		}
	}
}

func (m *Module) autoBopEnd(i int) float64 {
	ev := m.bops[i]
	end := math.Max(ev.beat+ev.length, m.endBeat+4)
	if sw := m.ctx.NextSwitchBeat(ev.beat); !math.IsInf(sw, 1) {
		end = sw
	}
	for j := i + 1; j < len(m.bops); j++ {
		if m.bops[j].beat > ev.beat && m.bops[j].beat < end {
			return m.bops[j].beat
		}
	}
	return end
}

func (m *Module) OnSwitch(beat float64) {
	for _, ev := range m.counts {
		if ev.beat > beat {
			break
		}
		m.setLionCount(ev.count, beat)
	}
	for _, ev := range m.texts {
		if ev.beat > beat {
			break
		}
		m.changeText(ev.custom, ev.text)
	}
}

func (m *Module) Whiff(beat float64) {
	m.playerClap(false, beat)
}

func (m *Module) Update(t, beat float64) {
	m.updateSign(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	m.ctx.SampleScene(beat)
	for _, l := range m.lions {
		l.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) setLionCount(n int, beat float64) {
	if n < minLions {
		n = minLions
	} else if n > maxLions {
		n = maxLions
	}
	m.lionCount = n
	m.lions = m.lions[:0]
	for i := 0; i < n; i++ {
		in := m.lionT.NewInstance()
		in.Offset[0] = lionStartX + (lionMaxW/float64(n+1))*float64(i+1)
		in.Offset[1] = 0
		in.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
		l := &lion{inst: in}
		m.lions = append(m.lions, l)
		m.setFace(i, 0)
	}
}

func (m *Module) prepare(face int, beat float64) {
	for i := range m.lions {
		m.setFace(i, face)
		m.lions[i].inst.PlayState("", "Prepare", beat, m.ctx.SecPerBeat(beat))
	}
	m.ctx.Sound("ready")
	m.canBop = false
}

func (m *Module) clap(beat, length, gameSwitchBeat float64) {
	m.clapStarted = true
	m.canHit = true
	m.canBop = false
	count := len(m.lions)
	for i := 0; i < count-1; i++ {
		idx := i
		bb := beat + length*float64(i)
		if bb < gameSwitchBeat {
			m.setFace(idx, 4)
			m.lions[idx].inst.PlayFrozen("", "Clap", 1)
			continue
		}
		do := func() {
			m.setFace(idx, 4)
			m.lions[idx].inst.PlayState("", "Clap", bb, m.ctx.SecPerBeat(bb))
		}
		sound := "leftClap"
		if i > 0 {
			sound = "middleClap"
		}
		if bb <= m.ctx.Beat()+1e-6 {
			m.ctx.Sound(sound)
			do()
		} else {
			m.ctx.SoundAt(bb, sound, 1)
			m.ctx.At(bb, do)
		}
	}
	end := beat + length*float64(count) - 0.1
	if end <= m.ctx.Beat()+1e-6 {
		m.canBop = true
	} else {
		m.ctx.At(end, func() { m.canBop = true })
	}
	target := beat + length*float64(count-1)
	m.ctx.ScheduleInput(target,
		func(state float64, _ engine.Judgment) { m.hitPlayerClap(state, target) },
		func() { m.missPlayerClap() })
}

func (m *Module) hitPlayerClap(state, beat float64) {
	if !m.canHit {
		m.playerClap(false, beat)
		return
	}
	if state >= 1 || state <= -1 {
		m.playerClap(false, beat)
		return
	}
	m.playerClap(true, beat)
}

func (m *Module) missPlayerClap() {
	m.misses++
	m.emoCount = 2
	if m.clapStarted {
		m.canHit = false
	}
}

func (m *Module) playerClap(just bool, beat float64) {
	if len(m.lions) == 0 {
		return
	}
	m.emoCount = 2
	pidx := len(m.lions) - 1
	player := m.lions[pidx]
	player.inst.SetActive("Hands/ClapEffect", just)
	if just {
		m.ctx.Sound("rightClap")
	} else {
		m.ctx.PlayCommon("miss")
		m.misses++
		if m.clapStarted {
			m.canHit = false
		}
	}
	m.clapStarted = false
	m.setFace(pidx, 4)
	player.inst.PlayState("", "Clap", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) bop(beat float64) {
	switch {
	case m.doEmotion && m.emoCount > 0 && m.misses == 0:
		for i := range m.lions {
			m.setFace(i, 1)
		}
		m.emoCount--
	case m.doEmotion && m.emoCount > 0 && m.misses > 0 && m.hasClapBefore(beat):
		for i := range m.lions {
			if i == len(m.lions)-1 {
				m.setFace(i, 0)
			} else {
				m.setFace(i, 2)
			}
		}
		m.emoCount--
	default:
		for i := range m.lions {
			m.setFace(i, 0)
		}
	}
	for _, l := range m.lions {
		l.inst.PlayState("", "Bop", beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) hasClapBefore(beat float64) bool {
	for _, c := range m.claps {
		if c.beat < beat {
			return true
		}
	}
	return false
}

func (m *Module) setFace(idx, face int) {
	if idx < 0 || idx >= len(m.lions) || len(m.faces) == 0 {
		return
	}
	if face < 0 {
		face = 0
	} else if face >= len(m.faces) {
		face = len(m.faces) - 1
	}
	m.lions[idx].inst.SetSprite("head_1", m.faces[face])
}

func (m *Module) changeText(custom bool, text string) {
	m.ctx.Scene.SetActive(m.trioTextPath, !custom)
	m.ctx.Scene.SetActive(m.customPath, custom)
	if custom {
		_ = m.ctx.Assets.SetText(m.customPath, text)
	}
}

func (m *Module) updateSign(beat float64) {
	clip, norm, moving, final := m.signAt(beat)
	if moving {
		m.ctx.Scene.PlayNormalized(m.signPath, clip, norm)
		m.signState = "moving:" + clip
		return
	}
	if final != "" && final != m.signState {
		if final == "Enter" || final == "Exit" {
			m.ctx.Scene.PlayFrozen(m.signPath, final, 1)
		} else {
			m.ctx.Scene.PlayState(m.signPath, final, beat, m.ctx.SecPerBeat(beat))
		}
		m.signState = final
	}
}

func (m *Module) signAt(beat float64) (clip string, norm float64, moving bool, final string) {
	final = "SignIdle"
	for _, ev := range m.signs {
		if beat < ev.beat {
			break
		}
		clip = "Exit"
		if ev.down {
			clip = "Enter"
		}
		if ev.length > 0 && beat < ev.beat+ev.length {
			u := (beat - ev.beat) / ev.length
			return clip, engine.Ease(ev.ease, 0, 1, u), true, ""
		}
		final = clip
	}
	return "", 0, false, final
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}
