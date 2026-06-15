// Package firstcontact ports Second Contact's Bob-speak / interpreter
// call-and-response flow from Assets/Scripts/Games/FirstContact.
package firstcontact

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "firstContact"

const (
	alienRoot      = "Alien"
	translatorRoot = "Translator"
	missionRoot    = "MissionControl"
	liveRoot       = "Live"
	crowdRoot      = "Background/CrowdOfAliens"

	alienBoxSprite     = "interpreterTextboxes_0"
	translateBoxSprite = "interpreterTextboxes_1"
	manIconSprite      = "textIcnSDF_1"
)

type speakEvt struct {
	beat     float64
	spaceNum int
	ddd      bool
	newline  bool
	dialogue string
}

type intervalEvt struct {
	beat, length float64
	outDialogue  string
	auto         bool
}

type turnoverEvt struct {
	beat, length float64
}

type missionEvt struct {
	beat, length float64
	stay         bool
}

type lookEvt struct {
	beat               float64
	alien, interpreter int
}

type liveEvt struct {
	beat   float64
	onBeat bool
}

type callToken struct {
	spaces int
	ddd    bool
}

type textRun struct {
	text string
	col  color.RGBA
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	speaks    []speakEvt
	intervals []intervalEvt
	turnovers []turnoverEvt
	missions  []missionEvt
	looks     []lookEvt
	lives     []liveEvt

	rng *rand.Rand

	currentVoice int
	hasMissed    bool
	noHitOnce    bool
	isSpeaking   bool

	missionActive bool
	onOutDialogue string
	callTokens    []callToken
	respRuns      []textRun
	respList      []speakEvt
	callDiagIndex int

	alienTextVisible     bool
	translateVisible     bool
	translateFailVisible bool

	liveOffset float64
	lastLive   int

	fontFace font.Face
	textImgs map[string]*ebiten.Image
}

func New() engine.Module {
	return &Module{
		rng:           rand.New(rand.NewSource(0x53434f4e)),
		onOutDialogue: "YOU SUCK AT CHARTING",
		lastLive:      -1 << 30,
		textImgs:      map[string]*ebiten.Image{},
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(ctx.AssetsRoot(), "common", "textbox_font.otf"))
	if err != nil {
		return err
	}
	f, err := opentype.Parse(raw)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 30, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return err
	}
	m.fontFace = face
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.resetScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case gameID + "/beat intervals":
		m.intervals = append(m.intervals, intervalEvt{
			beat: e.Beat, length: e.Length,
			outDialogue: e.Str("dialogue", "REPLACE THIS"),
			auto:        boolDefault(e, "auto", true),
		})
	case gameID + "/alien speak":
		m.speaks = append(m.speaks, speakEvt{
			beat: e.Beat, spaceNum: int(e.Float("spaceNum", 0)),
			ddd: boolParam(e, "dotdotdot"), newline: boolParam(e, "newline"),
			dialogue: e.Str("dialogue", ""),
		})
	case gameID + "/alien turnover":
		m.turnovers = append(m.turnovers, turnoverEvt{beat: e.Beat, length: defaultLength(e.Length, 1)})
	case gameID + "/alien success":
		b := e.Beat
		m.ctx.At(b, func() { m.alienSuccess(b) })
	case gameID + "/mission control":
		m.missions = append(m.missions, missionEvt{
			beat: e.Beat, length: e.Length, stay: boolParam(e, "toggle"),
		})
	case gameID + "/look at":
		m.looks = append(m.looks, lookEvt{
			beat: e.Beat, alien: int(e.Float("type", 0)), interpreter: int(e.Float("type2", 0)),
		})
	case gameID + "/live bar beat":
		m.lives = append(m.lives, liveEvt{beat: e.Beat, onBeat: boolDefault(e, "toggle", true)})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.speaks, func(i, j int) bool { return m.speaks[i].beat < m.speaks[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.turnovers, func(i, j int) bool { return m.turnovers[i].beat < m.turnovers[j].beat })
	sort.SliceStable(m.missions, func(i, j int) bool { return m.missions[i].beat < m.missions[j].beat })
	sort.SliceStable(m.looks, func(i, j int) bool { return m.looks[i].beat < m.looks[j].beat })
	sort.SliceStable(m.lives, func(i, j int) bool { return m.lives[i].beat < m.lives[j].beat })

	for _, iv := range m.intervals {
		iv := iv
		m.scheduleInterval(iv)
		if iv.auto {
			m.schedulePassTurn(iv.beat+iv.length, iv.beat, iv.beat+iv.length, 1)
		}
	}
	for _, to := range m.turnovers {
		to := to
		if iv, ok := m.previousInterval(to.beat); ok {
			m.schedulePassTurn(to.beat, iv.beat, iv.beat+iv.length, to.length)
		}
	}
	for _, ev := range m.missions {
		ev := ev
		m.ctx.At(ev.beat, func() { m.missionControlDisplay(ev.beat, ev.stay, ev.length) })
	}
	for _, ev := range m.looks {
		ev := ev
		m.ctx.At(ev.beat, func() { m.lookAtDirection(ev.alien, ev.interpreter, ev.beat) })
	}
	for _, ev := range m.lives {
		ev := ev
		m.ctx.At(ev.beat, func() { m.liveBarBeat(ev.onBeat) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.restoreLiveBar(beat)
	m.restoreIntervalText(beat)
	m.restoreMissionControl(beat)
	m.lastLive = int(math.Floor(beat - m.liveOffset))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, _ int) {
	switch {
	case m.isSpeaking:
		if m.noHitOnce || m.callDiagIndex == 0 {
			m.failContact()
			return
		}
		m.playPlayerA()
		m.trailingContact()
		m.ctx.ScoreMiss()
	case !m.noHitOnce && !m.missionActive:
		m.ctx.Scene.PlayState(translatorRoot, "translator_eh", beat, 0.5)
		m.ctx.SoundPitch("ALIEN_PLAYER_MISS2_A", 1, semitonePitch(m.rng.Intn(3)-2))
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat - m.liveOffset + 1e-6))
	if pulse > m.lastLive {
		m.ctx.Scene.PlayState(liveRoot, "liveBar", beat, m.ctx.SecPerBeat(beat))
		m.lastLive = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0xff, 0x89, 0xd8, 0xff})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawTextboxes(screen)
}

func (m *Module) scheduleInterval(iv intervalEvt) {
	m.ctx.At(iv.beat, func() {
		m.onOutDialogue = iv.outDialogue
		m.callTokens = nil
		m.respRuns = nil
		m.respList = nil
		m.callDiagIndex = 0
		m.alienTextVisible = false
		m.translateVisible = false
		m.translateFailVisible = false
		m.ctx.Scene.PlayState(translatorRoot, "translator_lookAtAlien", iv.beat, 0.5)
	})
	for _, sp := range m.speaksBetween(iv.beat, iv.beat+iv.length) {
		sp := sp
		m.ctx.At(sp.beat, func() {
			m.alienSpeak(sp, m.ctx.GameAt(sp.beat) == gameID)
		})
	}
}

func (m *Module) schedulePassTurn(beat, intervalBeat, endBeat, length float64) {
	inputs := m.speaksBetween(intervalBeat, endBeat)
	m.ctx.At(beat, func() {
		m.isSpeaking = true
		m.hasMissed = false
		m.respList = append(m.respList[:0], inputs...)
		m.callDiagIndex = 0
		m.alienTextVisible = false
		if m.ctx.GameAt(beat) == gameID {
			m.ctx.Sound("turnover")
			m.ctx.Scene.PlayState(alienRoot, "alien_point", beat, 0.5)
		}
	})
	m.ctx.At(beat+length/2, func() {
		if m.ctx.GameAt(beat+length/2) == gameID {
			m.ctx.Scene.PlayState(alienRoot, "alien_idle", beat+length/2, 0.5)
		}
	})
	for _, sp := range inputs {
		sp := sp
		target := beat + length + sp.beat - intervalBeat
		m.ctx.ScheduleInputAny(target, func(state float64, _ engine.Judgment) {
			m.alienTapping(state, target)
		}, func() {
			m.alienOnMiss(target)
		}).CanHit = func() bool {
			return !(m.hasMissed || m.noHitOnce)
		}
	}
}

func (m *Module) alienSpeak(sp speakEvt, active bool) {
	m.alienTextVisible = true
	m.callTokens = append(m.callTokens, callToken{spaces: sp.spaceNum * 2, ddd: sp.ddd})
	if !active {
		return
	}
	voice := m.rng.Intn(10) + 1
	if voice == m.currentVoice {
		voice++
		if voice > 10 {
			voice = 1
		}
	}
	m.currentVoice = voice
	m.ctx.SoundPitch("Bob"+itoa(voice), 1, centsPitch(float64(-100+m.rng.Intn(100))))
	m.ctx.Sound("BobB")
	m.ctx.Scene.PlayState(alienRoot, "alien_talk", sp.beat, 0.5)
	if m.rng.Intn(5) == 0 {
		m.ctx.Scene.PlayState(translatorRoot, "translator_lookAtAlien_nod", sp.beat, 0.5)
	}
}

func (m *Module) alienSuccess(beat float64) {
	anim := "alien_success"
	if m.hasMissed || m.noHitOnce {
		anim = "alien_fail"
		m.ctx.SoundAt(beat, "fail", 1)
		m.ctx.SoundAt(beat, "shakeHead", 1)
		m.ctx.SoundAt(beat+0.5, "shakeHead", 1)
	} else {
		m.ctx.SoundAt(beat, "successCrowd", 1)
		m.ctx.SoundAt(beat, "nod", 1)
		m.ctx.SoundAt(beat+0.5, "nod", 1)
		m.ctx.SoundAtPitchPan(beat+0.5, "successExtra"+itoa(m.rng.Intn(2)+1), 1, centsPitch(float64(m.rng.Intn(100)-50)), 0)
		m.ctx.SoundAtPitchPan(beat+0.5+m.rng.Float64(), "whistle", 0.4+m.rng.Float64()*0.6, centsPitch(float64(m.rng.Intn(150)-50)), 0)
	}
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(alienRoot, anim, beat, 0.5) })
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.PlayState(alienRoot, anim, beat+0.5, 0.5) })
	m.ctx.At(beat+1, func() {
		m.translateVisible = false
		m.translateFailVisible = false
	})
	m.isSpeaking = false
	m.hasMissed = false
	m.noHitOnce = false
}

func (m *Module) missionControlDisplay(beat float64, stay bool, length float64) {
	m.missionActive = true
	m.ctx.Scene.SetActive(missionRoot, true)
	m.alienTextVisible = false
	m.translateVisible = false
	m.translateFailVisible = false
	state := "missionControl_success"
	if m.hasMissed || m.noHitOnce {
		state = "missionControl_fail"
	}
	m.ctx.Scene.PlayState(missionRoot, state, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(alienRoot, "alien_idle", beat, 0.5)
	m.ctx.Scene.PlayState(translatorRoot, "translator_idle", beat, 0.5)
	if !stay {
		m.ctx.At(beat+length, func() {
			m.missionActive = false
			m.ctx.Scene.SetActive(missionRoot, false)
		})
	}
	m.isSpeaking = false
}

func (m *Module) lookAtDirection(alienLook, translatorLook int, beat float64) {
	if alienLook == 0 {
		m.ctx.Scene.PlayState(alienRoot, "alien_lookAt", beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(alienRoot, "alien_idle", beat, 0.5)
	}
	if translatorLook == 0 {
		m.ctx.Scene.PlayState(translatorRoot, "translator_lookAtAlien", beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(translatorRoot, "translator_idle", beat, 0.5)
	}
}

func (m *Module) liveBarBeat(onBeat bool) {
	if onBeat {
		m.liveOffset = 0
		return
	}
	m.liveOffset = 0.5
}

func (m *Module) failContact() {
	m.ctx.Sound("failContact")
	m.ctx.Scene.PlayState(translatorRoot, "translator_speak", m.ctx.Beat(), 0.5)
	if !m.hasMissed && m.callDiagIndex == 0 {
		m.translateFailVisible = true
		m.respRuns = []textRun{{text: m.onOutDialogue, col: color.RGBA{0x00, 0x00, 0x00, 0xff}}}
		m.ctx.ScoreMiss()
	}
	m.hasMissed = true
}

func (m *Module) trailingContact() {
	// FirstContact.cs calls firstContact/slightlyFail here, but the bundled
	// FirstContact/Sounds folder in HeavenStudio-master contains no such asset.
	// README tracks this as a known resource gap; do not substitute a different
	// fail sound because that would hide the missing official clip.
	if _, ok := m.ctx.Assets.Sounds["slightlyFail"]; ok {
		m.ctx.Sound("slightlyFail")
	}
	m.ctx.Scene.PlayState(translatorRoot, "translator_eh", m.ctx.Beat(), 0.5)
	if !m.hasMissed {
		m.respRuns = append(m.respRuns, textRun{text: " ..? ", col: color.RGBA{0xd9, 0x10, 0x20, 0xff}})
		m.translateVisible = true
	}
	m.hasMissed = true
}

func (m *Module) alienTapping(state float64, beat float64) {
	if m.callDiagIndex >= len(m.respList) {
		return
	}
	sp := m.respList[m.callDiagIndex]
	m.translateVisible = true
	if sp.newline {
		m.respRuns = nil
	}
	if state >= 1 || state <= -1 {
		m.playPlayerA()
		m.trailingContact()
		m.callDiagIndex++
		return
	}
	m.ctx.Scene.PlayState(translatorRoot, "translator_speak", beat, 0.5)
	m.playPlayerA()
	m.ctx.Sound("ALIEN_PLAYER_B")
	m.respRuns = append(m.respRuns, textRun{text: sp.dialogue, col: color.RGBA{0x00, 0x00, 0x00, 0xff}})
	m.callDiagIndex++
}

func (m *Module) alienOnMiss(beat float64) {
	if !m.noHitOnce && !m.hasMissed {
		m.ctx.Sound("alienNoHit")
		m.noHitOnce = true
	}
	if m.callDiagIndex > 0 && !m.hasMissed {
		m.respRuns = append(m.respRuns, textRun{text: " ..? ", col: color.RGBA{0xd9, 0x10, 0x20, 0xff}})
		m.translateVisible = true
		m.hasMissed = true
	}
	m.ctx.Scene.PlayState(alienRoot, "alien_noHit", beat, 0.5)
}

func (m *Module) playPlayerA() {
	m.ctx.SoundPitch("ALIEN_PLAYER_A", 1, semitonePitch(m.rng.Intn(6)-3))
}

func (m *Module) resetScene(beat float64) {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	m.hasMissed = false
	m.noHitOnce = false
	m.isSpeaking = false
	m.missionActive = false
	m.callTokens = nil
	m.respRuns = nil
	m.respList = nil
	m.callDiagIndex = 0
	m.alienTextVisible = false
	m.translateVisible = false
	m.translateFailVisible = false
	m.ctx.Scene.SetActive(missionRoot, false)
	m.ctx.Scene.PlayDefaultState(alienRoot, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(crowdRoot, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(translatorRoot, "translator_idle", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) restoreLiveBar(beat float64) {
	m.liveOffset = 0
	for _, ev := range m.lives {
		if ev.beat > beat {
			break
		}
		if ev.onBeat {
			m.liveOffset = 0
		} else {
			m.liveOffset = 0.5
		}
	}
}

func (m *Module) restoreIntervalText(beat float64) {
	for _, iv := range m.intervals {
		if beat < iv.beat || beat >= iv.beat+iv.length {
			continue
		}
		m.onOutDialogue = iv.outDialogue
		for _, sp := range m.speaksBetween(iv.beat, beat+1e-6) {
			m.alienTextVisible = true
			m.callTokens = append(m.callTokens, callToken{spaces: sp.spaceNum * 2, ddd: sp.ddd})
		}
		return
	}
}

func (m *Module) restoreMissionControl(beat float64) {
	for _, ev := range m.missions {
		if ev.beat > beat {
			break
		}
		active := ev.stay || beat < ev.beat+ev.length
		if !active {
			m.missionActive = false
			m.ctx.Scene.SetActive(missionRoot, false)
			continue
		}
		m.missionActive = true
		m.ctx.Scene.SetActive(missionRoot, true)
		state := "missionControl_success"
		if m.hasMissed || m.noHitOnce {
			state = "missionControl_fail"
		}
		m.ctx.Scene.PlayState(missionRoot, state, ev.beat, m.ctx.SecPerBeat(ev.beat))
	}
}

func (m *Module) previousInterval(beat float64) (intervalEvt, bool) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		if m.intervals[i].beat <= beat {
			return m.intervals[i], true
		}
	}
	return intervalEvt{}, false
}

func (m *Module) speaksBetween(start, end float64) []speakEvt {
	var out []speakEvt
	for _, sp := range m.speaks {
		if sp.beat >= start && sp.beat < end {
			out = append(out, sp)
		}
	}
	return out
}

func (m *Module) drawTextboxes(dst *ebiten.Image) {
	if m.alienTextVisible {
		m.drawBox(dst, alienBoxSprite, 0.77, -2.00)
		m.drawCall(dst)
	}
	if m.translateVisible {
		m.drawBox(dst, translateBoxSprite, 0.77, -2.25)
		m.drawRuns(dst, m.respRuns, 278, 378, 420, 64, "left")
	}
	if m.translateFailVisible {
		m.drawBox(dst, translateBoxSprite, 0.77, -2.25)
		m.drawRuns(dst, m.respRuns, 282, 378, 410, 64, "center")
	}
}

func (m *Module) drawBox(dst *ebiten.Image, sprite string, x, y float64) {
	m.ctx.Assets.DrawSpriteOpts(dst, sprite, kart.Translate(x, y), m.proj, kart.SpriteOpts{
		FlipY: true, Tint: [4]float64{1, 1, 1, 1},
	})
}

func (m *Module) drawCall(dst *ebiten.Image) {
	x, y := 330.0, 355.0
	for _, tok := range m.callTokens {
		x += float64(tok.spaces) * 7
		if tok.ddd {
			img := m.textImage([]textRun{{text: "...", col: color.RGBA{0xff, 0xff, 0xff, 0xff}}}, 54, 34, "left")
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Translate(x, y+5)
			dst.DrawImage(img, op)
			x += 42
		}
		icon := m.ctx.Assets.Sub(manIconSprite)
		if icon != nil {
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Scale(0.42, 0.42)
			op.GeoM.Translate(x, y)
			dst.DrawImage(icon, op)
		}
		x += 34
	}
}

func (m *Module) drawRuns(dst *ebiten.Image, runs []textRun, x, y, w, h float64, align string) {
	if len(runs) == 0 {
		return
	}
	img := m.textImage(runs, int(w), int(h), align)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(x, y)
	dst.DrawImage(img, op)
}

func (m *Module) textImage(runs []textRun, w, h int, align string) *ebiten.Image {
	if m.fontFace == nil || w <= 0 || h <= 0 {
		return nil
	}
	key := textImageKey(runs, w, h, align)
	if img, ok := m.textImgs[key]; ok {
		return img
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), image.Transparent, image.Point{}, draw.Src)
	met := m.fontFace.Metrics()
	ascent := met.Ascent.Ceil()
	lineH := (met.Ascent + met.Descent).Ceil()
	totalW := 0
	for _, r := range runs {
		totalW += font.MeasureString(m.fontFace, r.text).Ceil()
	}
	x := 0
	switch align {
	case "center":
		x = (w - totalW) / 2
	case "right":
		x = w - totalW - 4
	default:
		x = 4
	}
	y := (h-lineH)/2 + ascent
	for _, r := range runs {
		src := image.NewUniform(r.col)
		d := &font.Drawer{Dst: rgba, Src: src, Face: m.fontFace, Dot: fixed.P(x, y)}
		d.DrawString(r.text)
		x += font.MeasureString(m.fontFace, r.text).Ceil()
	}
	img := ebiten.NewImageFromImage(rgba)
	m.textImgs[key] = img
	return img
}

func textImageKey(runs []textRun, w, h int, align string) string {
	var b strings.Builder
	b.WriteString(align)
	b.WriteByte('|')
	b.WriteString(itoa(w))
	b.WriteByte('x')
	b.WriteString(itoa(h))
	for _, r := range runs {
		b.WriteByte('|')
		b.WriteString(r.text)
		b.WriteByte('#')
		b.WriteString(itoa(int(r.col.R)))
		b.WriteByte(',')
		b.WriteString(itoa(int(r.col.G)))
		b.WriteByte(',')
		b.WriteString(itoa(int(r.col.B)))
	}
	return b.String()
}

func semitonePitch(n int) float64      { return math.Pow(2, float64(n)/12) }
func centsPitch(cents float64) float64 { return math.Pow(2, cents/1200) }

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func defaultLength(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
