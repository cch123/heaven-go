// Package bonodori ports The Bon Odori's don/pan call timing, Donpan/Judge
// animator states, lyric line events, bow lockout, and overlay fades from
// Assets/Scripts/Games/BonOdori/BonOdori.cs.
package bonodori

import (
	"image/color"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	panTypePan = iota
	panTypePa
	panTypePanHold
)

const (
	clapSide = iota
	clapFront
)

const (
	opShowText = iota
	opDeleteText
	opScrollText
)

var (
	transparent = [4]float64{0, 0, 0, 0}
	defaultDim  = [4]float64{0, 0, 0, 0.4666}
)

type bopEvt struct {
	beat, length float64
	toggle, auto bool
}

type clapEvt struct {
	beat                       float64
	variation, speak, clapType int
	muted                      bool
	semitone                   int
}

type donEvt struct {
	beat                       float64
	variation, speak, semitone int
}

type textOp struct {
	kind         int
	beat, length float64
	lines        [5]string
	flags        [5]bool
}

type overlayEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type bowEvt struct {
	beat, length float64
}

type scrollLine struct {
	active       bool
	beat, length float64
	raw          string
}

type lyricLine struct {
	runs   []kart.TextRun
	breaks []int
	plain  string
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	darkPlane string
	judge     string
	judgeFace string
	texts     []string
	textsBlue []string
	donpans   []string

	bops     []bopEvt
	claps    []clapEvt
	dons     []donEvt
	textOps  []textOp
	overlays []overlayEvt
	bows     []bowEvt

	originalText [5]string
	scrolls      [5]scrollLine

	clapTypeString string
	noBopPlayer    bool
	noBopDonpans   bool
	noBopPBeats    []float64
	noBopDBeats    []float64
	lastPulse      int
}

func New() engine.Module {
	return &Module{clapTypeString: "ClapFront", lastPulse: -1 << 30}
}

func (m *Module) ID() string { return "bonOdori" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("bonOdori"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.darkPlane = roleOr(ctx, "darkPlane", "Square")
	m.judge = roleOr(ctx, "Judge", "Judge")
	m.judgeFace = roleOr(ctx, "JudgeFace", "Judge/Head/Face")
	m.texts = append([]string(nil), ctx.Assets.Extra.RefArrays["Texts"]...)
	m.textsBlue = append([]string(nil), ctx.Assets.Extra.RefArrays["TextsBlue"]...)
	m.donpans = append([]string(nil), ctx.Assets.Extra.RefArrays["Donpans"]...)
	m.clearAllText()
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
	switch e.Datamodel {
	case "bonOdori/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			toggle: boolDefault(e, "toggle", true),
			auto:   boolParam(e, "auto"),
		})
	case "bonOdori/pan":
		m.claps = append(m.claps, clapEvt{
			beat: b, speak: intParam(e, "type", panTypePan),
			variation: panVariation(e), muted: boolParam(e, "mute"),
			clapType: intParam(e, "clapType", clapSide),
			semitone: intParam(e, "semitone", 0),
		})
	case "bonOdori/don":
		m.dons = append(m.dons, donEvt{
			beat: b, speak: intParam(e, "type", panTypePan),
			variation: donVariation(e), semitone: intParam(e, "semitone", 0),
		})
	case "bonOdori/show text":
		m.textOps = append(m.textOps, textOp{kind: opShowText, beat: b, lines: lineParams(e)})
	case "bonOdori/delete text":
		m.textOps = append(m.textOps, textOp{kind: opDeleteText, beat: b, flags: lineFlags(e)})
	case "bonOdori/scroll text":
		m.textOps = append(m.textOps, textOp{kind: opScrollText, beat: b, length: e.Length, flags: lineFlags(e)})
	case "bonOdori/bow":
		m.bows = append(m.bows, bowEvt{beat: b, length: e.Length})
	case "bonOdori/overlay":
		m.overlays = append(m.overlays, overlayEvt{
			beat: b, length: e.Length,
			from: colorParam(e, "colorStart", transparent),
			to:   colorParam(e, "colorEnd", defaultDim),
			ease: intParam(e, "ease", 0),
		})
	case "bonOdori/toggle bg":
		// Current Heaven Studio migrates this hidden legacy event to overlay.
		// BonOdori.DarkBG itself is empty, so un-migrated old charts intentionally no-op.
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.claps, func(i, j int) bool { return m.claps[i].beat < m.claps[j].beat })
	sort.SliceStable(m.dons, func(i, j int) bool { return m.dons[i].beat < m.dons[j].beat })
	sort.SliceStable(m.textOps, func(i, j int) bool { return m.textOps[i].beat < m.textOps[j].beat })
	sort.SliceStable(m.overlays, func(i, j int) bool { return m.overlays[i].beat < m.overlays[j].beat })
	sort.SliceStable(m.bows, func(i, j int) bool { return m.bows[i].beat < m.bows[j].beat })

	for _, ev := range m.bops {
		ev := ev
		if !ev.auto && ev.toggle {
			for i := 0; float64(i) < ev.length-1e-6; i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.bop(b) })
			}
		}
	}
	for _, ev := range m.claps {
		ev := ev
		m.ctx.At(ev.beat-0.1, func() { m.clapTypeString = clapState(ev.clapType) })
		m.ctx.At(ev.beat, func() { m.clap(ev) })
	}
	for _, ev := range m.dons {
		ev := ev
		m.ctx.At(ev.beat, func() { m.playDon(ev) })
	}
	for _, op := range m.textOps {
		op := op
		m.ctx.At(op.beat, func() { m.applyTextOp(op, op.beat) })
	}
	for _, ev := range m.bows {
		ev := ev
		m.ctx.At(ev.beat, func() { m.bow(ev) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for path := range m.ctx.Assets.Animators {
		m.ctx.Scene.PlayDefaultState(path, beat, sec)
	}
	m.noBopPlayer, m.noBopDonpans = false, false
	m.noBopPBeats, m.noBopDBeats = nil, nil
	m.clapTypeString = m.clapTypeAt(beat)
	m.restoreTextAt(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Sound("clap")
	m.playDonpan(0, m.clapTypeString, beat, 0.5)
	if m.clapTypeString == "ClapFront" {
		m.blockPlayerBop(beat, beat+2)
	}
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse != m.lastPulse {
		m.lastPulse = pulse
		if m.autoBopAt(float64(pulse)) {
			m.bop(float64(pulse))
		}
	}
	m.updateScrolls(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x31, 0x2b, 0x9f, 0xff})
	m.ctx.Scene.SetColorOver(m.darkPlane, m.overlayAt(beat))
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) clap(ev clapEvt) {
	if !ev.muted {
		m.ctx.SoundPitch(speakClip("pan", ev.speak, ev.variation), 1, semitonePitch(ev.semitone))
	}
	m.ctx.SoundVol("clap2", 0.5)
	m.noBopDBeats = append(m.noBopDBeats, ev.beat)
	for i := 1; i < len(m.donpans); i++ {
		m.playDonpan(i, m.clapTypeString, ev.beat, 0.5)
	}
	if m.clapTypeString == "ClapFront" {
		m.blockDonpanBop(ev.beat+0.05, ev.beat+1.01)
	}
	m.ctx.ScheduleInput(ev.beat, func(state float64, _ engine.Judgment) {
		m.clapSuccess(ev, state)
	}, func() {
		// C# passes Empty as the no-input callback; missing a pan cue does not
		// trigger the Judge sad face unless an actual wrong input produced Miss.
	})
}

func (m *Module) clapSuccess(ev clapEvt, state float64) {
	m.playDonpan(0, m.clapTypeString, m.ctx.Beat(), 0.5)
	if state <= -1 || state >= 1 {
		m.ctx.Sound("common_nearMiss")
		return
	}
	m.ctx.Sound("clap")
	closest := ev.beat
	if len(m.noBopDBeats) > 0 {
		closest = closestBeat(m.noBopDBeats, m.ctx.Beat())
	}
	m.noBopPBeats = append(m.noBopPBeats, closest)
	if m.clapTypeString == "ClapFront" {
		m.blockPlayerBop(closest+0.05, closest+1.01)
	}
}

func (m *Module) playDon(ev donEvt) {
	m.ctx.SoundPitch(speakClip("don", ev.speak, ev.variation), 1, semitonePitch(ev.semitone))
	m.clapTypeString = m.clapTypeAfter(ev.beat)
}

func (m *Module) bop(beat float64) {
	if !m.noBopPlayer && !containsBeat(m.noBopPBeats, beat) {
		if !playingAny(m.ctx.Scene, m.donpans[0], beat, "ClapSide", "ClapFront") {
			m.playDonpan(0, "Bop", beat, 0.5)
		}
	}
	if !m.noBopDonpans && !containsBeat(m.noBopDBeats, beat) {
		for i := 1; i < len(m.donpans); i++ {
			if !playingAny(m.ctx.Scene, m.donpans[i], beat, "ClapSide", "ClapFront") {
				m.playDonpan(i, "Bop", beat, 0.5)
			}
		}
	}
	m.ctx.Scene.PlayState(m.judge, "Bop", beat, 0.5)
}

func (m *Module) bow(ev bowEvt) {
	m.noBopPlayer, m.noBopDonpans = true, true
	for i := range m.donpans {
		m.playDonpan(i, "Bow", ev.beat, 1)
	}
	m.ctx.At(ev.beat+ev.length, func() {
		m.noBopPlayer, m.noBopDonpans = false, false
		if !containsBeat(m.noBopPBeats, ev.beat+ev.length) {
			m.playDonpan(0, "NeutralBopped", ev.beat+ev.length, 1)
		}
		if !containsBeat(m.noBopDBeats, ev.beat+ev.length) {
			for i := 1; i < len(m.donpans); i++ {
				m.playDonpan(i, "NeutralBopped", ev.beat+ev.length, 1)
			}
		}
		if m.autoBopAt(ev.beat + ev.length) {
			m.bop(ev.beat + ev.length)
		}
	})
}

func (m *Module) playDonpan(idx int, state string, beat, scale float64) {
	if idx < 0 || idx >= len(m.donpans) || m.donpans[idx] == "" {
		return
	}
	m.ctx.Scene.PlayState(m.donpans[idx], state, beat, scale)
}

func (m *Module) blockPlayerBop(from, until float64) {
	m.noBopPlayer = true
	m.ctx.At(until, func() {
		if len(m.noBopPBeats) == 0 || m.noBopPBeats[len(m.noBopPBeats)-1] <= from {
			m.noBopPlayer = false
		}
	})
}

func (m *Module) blockDonpanBop(from, until float64) {
	m.noBopDonpans = true
	m.ctx.At(until, func() {
		if len(m.noBopDBeats) == 0 || m.noBopDBeats[len(m.noBopDBeats)-1] <= from {
			m.noBopDonpans = false
		}
	})
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) clapTypeAt(beat float64) string {
	state := "ClapFront"
	for _, ev := range m.claps {
		if ev.beat-0.1 > beat {
			break
		}
		state = clapState(ev.clapType)
	}
	return state
}

func (m *Module) clapTypeAfter(beat float64) string {
	for _, ev := range m.claps {
		if ev.beat >= beat {
			return clapState(ev.clapType)
		}
	}
	return m.clapTypeString
}

func (m *Module) overlayAt(beat float64) [4]float64 {
	col := transparent
	for _, ev := range m.overlays {
		if ev.beat > beat {
			break
		}
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			col = ev.to
			continue
		}
		t := (beat - ev.beat) / ev.length
		for i := 0; i < 4; i++ {
			col[i] = engine.Ease(ev.ease, ev.from[i], ev.to[i], t)
		}
	}
	return col
}

func (m *Module) clearAllText() {
	for i := 0; i < 5; i++ {
		m.originalText[i] = ""
		m.scrolls[i] = scrollLine{}
		m.setLine(i, "", nil)
	}
}

func (m *Module) applyTextOp(op textOp, beat float64) {
	switch op.kind {
	case opShowText:
		for i, raw := range op.lines {
			if raw == "" || raw == defaultLineHelp {
				continue
			}
			m.originalText[i] = raw
			m.scrolls[i] = scrollLine{}
			m.setLine(i, raw, nil)
		}
	case opDeleteText:
		for i, ok := range op.flags {
			if !ok {
				continue
			}
			m.originalText[i] = ""
			m.scrolls[i] = scrollLine{}
			m.setLine(i, "", nil)
		}
	case opScrollText:
		for i, ok := range op.flags {
			if !ok || m.originalText[i] == "" {
				continue
			}
			m.scrolls[i] = scrollLine{active: true, beat: op.beat, length: op.length, raw: m.originalText[i]}
			m.renderScrollLine(i, beat)
		}
	}
}

func (m *Module) restoreTextAt(beat float64) {
	m.clearAllText()
	for _, op := range m.textOps {
		if op.beat > beat {
			break
		}
		m.applyTextOp(op, beat)
	}
	m.updateScrolls(beat)
}

func (m *Module) updateScrolls(beat float64) {
	for i := range m.scrolls {
		if m.scrolls[i].active {
			m.renderScrollLine(i, beat)
		}
	}
}

func (m *Module) renderScrollLine(i int, beat float64) {
	sc := m.scrolls[i]
	if !sc.active {
		return
	}
	if sc.length <= 0 || beat >= sc.beat+sc.length {
		m.scrolls[i].active = false
		m.setLine(i, sc.raw, ptrFloat(m.lyricWidth(i, sc.raw)))
		return
	}
	if beat < sc.beat {
		m.setLine(i, sc.raw, ptrFloat(0))
		return
	}
	t := (beat - sc.beat) / sc.length
	m.setLine(i, sc.raw, ptrFloat(m.scrollReveal(i, sc.raw, t)))
}

func (m *Module) setLine(i int, raw string, blueReveal *float64) {
	if i < len(m.texts) {
		_ = m.ctx.Assets.SetTextRuns(m.texts[i], parseLyric(raw, false).runs)
		m.ctx.Scene.SetColorOver(m.texts[i], [4]float64{1, 1, 1, 1})
	}
	if i < len(m.textsBlue) {
		reveal := 0.0
		if blueReveal != nil {
			reveal = *blueReveal
		}
		_ = m.ctx.Assets.SetTextRunsClipped(m.textsBlue[i], parseLyric(raw, true).runs, reveal)
		m.ctx.Scene.SetColorOver(m.textsBlue[i], [4]float64{1, 1, 1, 1})
	}
}

func (m *Module) lyricWidth(i int, raw string) float64 {
	if i >= len(m.textsBlue) || raw == "" {
		return 0
	}
	line := parseLyric(raw, true)
	width, _, err := m.ctx.Assets.MeasureTextRuns(m.textsBlue[i], line.runs)
	if err != nil {
		return 0
	}
	return width
}

func (m *Module) scrollReveal(i int, raw string, t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return m.lyricWidth(i, raw)
	}
	if i >= len(m.textsBlue) || raw == "" {
		return 0
	}
	line := parseLyric(raw, true)
	width, charX, err := m.ctx.Assets.MeasureTextRuns(m.textsBlue[i], line.runs)
	if err != nil {
		return 0
	}
	edges := []float64{0}
	for _, pos := range line.breaks {
		switch {
		case pos < 0:
			edges = append(edges, 0)
		case pos < len(charX):
			edges = append(edges, charX[pos])
		default:
			edges = append(edges, width)
		}
	}
	edges = append(edges, width)
	span := len(edges) - 1
	scaled := t * float64(span)
	idx := int(math.Floor(scaled))
	if idx >= span {
		return edges[span]
	}
	u := scaled - float64(idx)
	return edges[idx] + (edges[idx+1]-edges[idx])*u
}

func speakClip(prefix string, speak, variation int) string {
	switch speak {
	case panTypePan:
		return prefix + itoa1(variation)
	case panTypePa:
		if prefix == "pan" {
			return "pa" + itoa1(variation)
		}
		return "do" + itoa1(variation)
	default:
		if prefix == "pan" {
			return "pa_n" + itoa1(variation)
		}
		return "do_n" + itoa1(variation)
	}
}

func itoa1(v int) string {
	switch v + 1 {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	default:
		return "1"
	}
}

func panVariation(e *riq.Entity) int {
	switch intParam(e, "type", panTypePan) {
	case panTypePan:
		return intParam(e, "variationPan", 0)
	case panTypePa:
		return intParam(e, "variationPa", 0)
	default:
		return intParam(e, "variationPa_n", 0)
	}
}

func donVariation(e *riq.Entity) int {
	switch intParam(e, "type", panTypePan) {
	case panTypePan:
		return intParam(e, "variationDon", 0)
	case panTypePa:
		return intParam(e, "variationDo", 0)
	default:
		return intParam(e, "variationDo_n", 0)
	}
}

func clapState(v int) string {
	if v == clapSide {
		return "ClapSide"
	}
	return "ClapFront"
}

func lineParams(e *riq.Entity) [5]string {
	return [5]string{
		e.Str("line 1", ""),
		e.Str("line 2", ""),
		e.Str("line 3", ""),
		e.Str("line 4", ""),
		e.Str("line 5", ""),
	}
}

func lineFlags(e *riq.Entity) [5]bool {
	return [5]bool{
		boolParam(e, "line 1"),
		boolParam(e, "line 2"),
		boolParam(e, "line 3"),
		boolParam(e, "line 4"),
		boolParam(e, "line 5"),
	}
}

const defaultLineHelp = "Type r| for red text, g| for green text and y| for yellow text. These can be used multiple times in a single line."

func cleanLyric(s string) string {
	return parseLyric(s, false).plain
}

func parseLyric(s string, blue bool) lyricLine {
	line := lyricLine{}
	col := [4]float64{1, 1, 1, 1}
	scale := 1.0
	var buf strings.Builder
	plainPos := 0
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		line.runs = append(line.runs, kart.TextRun{Text: buf.String(), Color: col, Scale: scale})
		buf.Reset()
	}
	for len(s) > 0 {
		switch {
		case strings.HasPrefix(s, "r|"):
			flush()
			col = lyricMarkerColor('r', blue)
			s = s[2:]
			continue
		case strings.HasPrefix(s, "g|"):
			flush()
			col = lyricMarkerColor('g', blue)
			s = s[2:]
			continue
		case strings.HasPrefix(s, "y|"):
			flush()
			col = lyricMarkerColor('y', blue)
			s = s[2:]
			continue
		case strings.HasPrefix(s, "s|"):
			flush()
			scale = 0.9375
			s = s[2:]
			continue
		case strings.HasPrefix(s, "|s"):
			flush()
			scale = 1
			s = s[2:]
			continue
		case strings.HasPrefix(s, "d|"):
			flush()
			line.breaks = append(line.breaks, plainPos)
			s = s[2:]
			continue
		}
		r, sz := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && sz == 0 {
			break
		}
		buf.WriteRune(r)
		line.plain += string(r)
		plainPos++
		s = s[sz:]
	}
	flush()
	return line
}

func lyricMarkerColor(marker rune, blue bool) [4]float64 {
	switch marker {
	case 'r':
		if blue {
			return [4]float64{1, 0, 1, 1}
		}
		return [4]float64{1, 0, 0, 1}
	case 'g':
		if blue {
			return [4]float64{0, 1, 1, 1}
		}
		return [4]float64{0, 1, 0, 1}
	case 'y':
		if blue {
			return [4]float64{1, 1, 1, 1}
		}
		return [4]float64{1, 1, 0, 1}
	default:
		return [4]float64{1, 1, 1, 1}
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}

func closestBeat(xs []float64, target float64) float64 {
	best := xs[0]
	bestD := math.Abs(best - target)
	for _, x := range xs[1:] {
		if d := math.Abs(x - target); d < bestD {
			best, bestD = x, d
		}
	}
	return best
}

func containsBeat(xs []float64, beat float64) bool {
	for _, x := range xs {
		if math.Abs(x-beat) <= 1e-9 {
			return true
		}
	}
	return false
}

func playingAny(sc *kart.SceneInst, path string, beat float64, names ...string) bool {
	st, playing := sc.StateInfo(path, beat)
	if !playing {
		return false
	}
	for _, name := range names {
		if st == name {
			return true
		}
	}
	return false
}

func semitonePitch(semi int) float64 { return math.Exp2(float64(semi) / 12) }

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func boolParam(e *riq.Entity, key string) bool { return boolDefault(e, key, false) }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3])}
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}
