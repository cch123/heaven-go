// Package cheerreaders 是 Cheer Readers（cheerReaders）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/CheerReaders/CheerReaders.cs + RvlCharacter.cs：
//
//	oneTwoThree：     三排书依次横翻（B/B+1/B+2），玩家 B+2 翻书
//	itsUpToYou：      纵列书翻（B/B+0.75/B+1.5/B+2），玩家 B+2 翻书
//	letsGoReadABunchaBooks：斜向连翻（B+0.75..B+1.75），玩家 B+2 翻书
//	rahRahSisBoomBaBoom：斜向连翻（B..B+2），玩家 B+2.5 翻书（BOOM）
//	okItsOn(Stretch)：哨声 + B+len/2 全员转书（按住南键），B+3len/4 开书
//	                  （松开）露出海报（SpriteMask 书窗），相机冲击变焦
//	yay：             成功后欢呼（白/黑纸花粒子）
//	bop/resetPose/toggleCaption：区间 bop / 姿势复位 / 字幕开关
//
// 黑白交替：每个 cue 中点翻转 shouldBeBlack，全员书色随之轮换。
// 字幕：Code Remix 等 version=0 旧谱面无 toggleCaption 块时自动禁用
// （CheckCaptions 语义），启用路径未实现（官方非 PRACTICE 关卡未用到）。
package cheerreaders

import (
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// posterFiles 对应 PosterToChoose 枚举顺序（切片名 "<文件>/TopPart" 等；
// 首个被扫描的文件（Crop Stomp）切片未命名空间化，按回退名解析）。
var posterFiles = []string{
	"DJ School", "Lockstep", "Rhythm Tweezers", "Crop Stomp", "Fillbots",
	"Fillbots Empty", "Frog Hop", "Moai Doo-Wop", "Night Walk GBA",
	"Rhythm Rally", "Space Dance", "Tap Trial 2", "Tap Trial Remix 5",
	"Tap Trial Remix 7",
}

type cueEvt struct {
	beat, length float64
	kind         string
}

type girl struct {
	path      string
	face      string
	white     bool
	open      bool
	noBop     bool
	missed    bool
	spinning  bool
	canOpen   bool
	blushBeat float64
	isPlayer  bool
	lookDown  bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cues []cueEvt // 全部 cue 事件（Automatic 仲裁 + doingCue 用）
	bops []struct {
		beat, length float64
		auto         bool
	}
	black    bool // shouldBeBlack
	yay      bool // shouldYay
	zoomOK   bool // shouldDoSuccessZoom
	lastSolo bool
	canBop   bool
	cueBeat  float64
	cueLen   float64
	doing    bool

	girls  []*girl // 12 NPC（first+second+third row 顺序）
	player *girl

	stopSpin func()

	// okItsOn 冲击变焦（allCameraEvents 语义）
	zooms     []struct{ beat, length float64 }
	zoomIdx   int
	zoomAdd   float64
	lastT     float64
	particles []confetti
	lastBop   float64
}

type confetti struct {
	x, y, vx, vy, born float64
	black              bool
}

func New() engine.Module { return &Module{canBop: true} }

func (m *Module) ID() string { return "cheerReaders" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("cheerReaders"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	mk := func(path string, isPlayer bool) *girl {
		return &girl{path: path, face: path + "/Head/faceSprites", white: true,
			isPlayer: isPlayer, blushBeat: -10}
	}
	for _, key := range []string{"firstRow", "secondRow", "thirdRow"} {
		for _, p := range ctx.Assets.Extra.RefArrays[key] {
			m.girls = append(m.girls, mk(p, false))
		}
	}
	// shouldLookDown：girl 组件序列化值
	for _, c := range ctx.Assets.Extra.Components {
		if c.Path == "" {
			continue
		}
		for _, g := range m.girls {
			if g.path == c.Path && c.Nums["shouldLookDown"] != 0 {
				g.lookDown = true
			}
		}
	}
	m.player = mk(ctx.Role("player"), true)
	m.stopSpin = func() {}
	// 初始姿势：书窗遮罩与 missPoster 关闭
	m.hideMasks()
	ctx.Scene.SetActive(ctx.Role("missPoster"), false)
	return nil
}

func (m *Module) all() []*girl { return append(append([]*girl{}, m.girls...), m.player) }

func (m *Module) hideMasks() {
	sc := m.ctx.Scene
	for _, key := range []string{"topMasks", "middleMasks", "bottomMasks"} {
		for _, p := range m.ctx.Assets.Extra.RefArrays[key] {
			sc.SetActive(p, false)
		}
	}
	sc.SetActive(m.ctx.Role("playerMask"), false)
}

// ---------- RvlCharacter ----------

func (m *Module) play(g *girl, anim string, scaled bool) {
	b := m.ctx.Beat()
	ts := m.ctx.SecPerBeat(b)
	if scaled {
		ts = 0.5
	}
	m.ctx.Scene.PlayState(g.path, anim, b, ts)
}

func (m *Module) playFace(g *girl, anim string, scaled bool) {
	b := m.ctx.Beat()
	ts := m.ctx.SecPerBeat(b)
	if scaled {
		ts = 0.5
	}
	m.ctx.Scene.PlayState(g.face, anim, b, ts)
}

func (g *girl) baseColor(white bool) string {
	if white {
		return "White"
	}
	return "Black"
}

func (m *Module) flipBook(g *girl, hit bool) {
	m.maskOf(g, false)
	g.canOpen = !hit
	g.spinning = false
	if g.white != m.black && hit && g.isPlayer {
		// 书色已正确：仅调整姿势
		m.play(g, "RepositionTo"+g.baseColor(g.white), true)
		g.open, g.noBop = true, false
		return
	}
	if g.white {
		m.play(g, "WhitetoBlack", true)
	} else {
		m.play(g, "BlacktoWhite", true)
	}
	g.white = !g.white
	g.open, g.noBop, g.missed = true, false, !hit
}

func (m *Module) startSpin(g *girl) {
	m.maskOf(g, false)
	g.canOpen = true
	m.play(g, "Spinfrom"+g.baseColor(g.white), true)
	g.open, g.noBop, g.spinning = true, true, true
}

func (m *Module) stopSpinBook(g *girl) {
	if !g.canOpen {
		return
	}
	m.maskOf(g, true)
	m.play(g, "OpenBook", true)
	g.open, g.noBop, g.spinning = true, true, false
}

func (m *Module) missChar(g *girl) {
	g.blushBeat = m.ctx.Beat()
	m.ctx.Scene.SetActive(g.face+"/Blush", true)
	m.ctx.Scene.SetActive(g.face+"/Blush (1)", true)
	m.play(g, "Miss"+g.baseColor(g.white), false)
	g.noBop, g.missed, g.spinning = true, true, false
}

func (m *Module) yayChar(g *girl, speak bool) {
	if speak {
		m.playFace(g, "FaceYay", true)
	}
	m.play(g, g.baseColor(g.white)+"Yay", true)
}

func (m *Module) bopChar(g *girl) {
	if g.noBop {
		return
	}
	if g.open {
		m.play(g, g.baseColor(g.white)+"Bop", true)
	} else {
		m.play(g, "Bop", true)
	}
}

// maskOf：角色的书窗遮罩（player → playerMask，NPC 按行/列）。
func (m *Module) maskOf(g *girl, on bool) {
	sc := m.ctx.Scene
	if g.isPlayer {
		sc.SetActive(m.ctx.Role("playerMask"), on)
		return
	}
	for gi, gg := range m.girls {
		if gg != g {
			continue
		}
		keys := []string{"topMasks", "topMasks", "topMasks", "topMasks",
			"middleMasks", "middleMasks", "middleMasks", "middleMasks",
			"bottomMasks", "bottomMasks", "bottomMasks"}
		idx := []int{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2}
		if gi < len(keys) {
			arr := m.ctx.Assets.Extra.RefArrays[keys[gi]]
			if idx[gi] < len(arr) {
				sc.SetActive(arr[idx[gi]], on)
			}
		}
	}
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "cheerReaders/bop":
		auto := boolParam(e, "toggle2")
		m.bops = append(m.bops, struct {
			beat, length float64
			auto         bool
		}{b, e.Length, auto})
		if boolParam(e, "toggle") {
			for i := 0.0; i < e.Length; i++ {
				bb := b + i
				ctx.At(bb, func() { m.bopAll() })
			}
		}
	case "cheerReaders/oneTwoThree":
		m.cues = append(m.cues, cueEvt{b, e.Length, "123"})
	case "cheerReaders/itsUpToYou":
		m.cues = append(m.cues, cueEvt{b, e.Length, "uty"})
	case "cheerReaders/letsGoReadABunchaBooks":
		m.cues = append(m.cues, cueEvt{b, e.Length, "books"})
	case "cheerReaders/rahRahSisBoomBaBoom":
		m.cues = append(m.cues, cueEvt{b, e.Length, "rah"})
	case "cheerReaders/okItsOn", "cheerReaders/okItsOnStretch":
		m.cues = append(m.cues, cueEvt{b, e.Length, "ok"})
		if boolParam(e, "impactZoom") {
			m.zooms = append(m.zooms, struct{ beat, length float64 }{b, e.Length})
		}
	case "cheerReaders/yay":
		solo := int(e.Float("solo", 2))
		ctx.At(b, func() { m.doYay(solo) })
	case "cheerReaders/resetPose":
		ctx.At(b, func() { m.resetPose() })
	case "cheerReaders/toggleCaption":
		if boolParam(e, "captions") {
			log.Printf("cheerReaders: toggleCaption 启用字幕未实现（官方非 PRACTICE 关未用）")
		}
	}
}

// Ready：cue 的具体调度（Automatic 仲裁需要全量 cue 列表）。
func (m *Module) Ready() {
	for _, c := range m.cues {
		c := c
		switch c.kind {
		case "123":
			m.cue123(c)
		case "uty":
			m.cueUpToYou(c)
		case "books":
			m.cueBooks(c)
		case "rah":
			m.cueRah(c)
		case "ok":
			m.cueOkItsOn(c)
		}
	}
}

func (m *Module) autoSolo(c cueEvt, win float64) int {
	overlap := false
	covering := false
	for _, o := range m.cues {
		if o == c {
			continue
		}
		if o.beat > c.beat && o.beat < c.beat+win {
			overlap = true
		}
		if o.beat < c.beat && o.beat+o.length > c.beat {
			overlap, covering = true, true
		}
	}
	if !overlap {
		return 2
	}
	if !covering {
		m.lastSolo = false
	}
	who := 0
	if m.lastSolo {
		who = 1
	}
	m.lastSolo = !m.lastSolo
	return who
}

// soloOf：事件参数（存于 cue 调度时的原始实体——OnEvent 顺序与 Ready
// 一致，这里按 riq 重查）。
func (m *Module) soloOf(c cueEvt, win float64, raw int) int {
	if raw == 3 {
		return m.autoSolo(c, win)
	}
	if raw == 0 {
		m.lastSolo = true
	} else if raw == 1 {
		m.lastSolo = false
	}
	return raw
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

// setIsDoingCue：表情复位 + cue 区间 + 中点翻转黑白。
func (m *Module) setCue(c cueEvt, switchColor bool) {
	ctx := m.ctx
	ctx.At(c.beat, func() {
		if !m.doing {
			m.yay = false
		}
		for _, g := range m.all() {
			m.playFace(g, "FaceIdle", false) // ResetFace（IsAnimationNotPlaying 近似恒触发）
		}
		m.doing, m.cueBeat, m.cueLen = true, c.beat, c.length-1
		m.canBop = false
	})
	if switchColor {
		ctx.At(c.beat+c.length*0.5, func() { m.black = !m.black })
	}
}

func (m *Module) faceAll(solo int, anim string) {
	switch solo {
	case 0:
		m.playFace(m.player, anim, true)
	case 1:
		for _, g := range m.girls {
			m.playFace(g, anim, true)
		}
	case 2:
		m.playFace(m.player, anim, true)
		for _, g := range m.girls {
			m.playFace(g, anim, true)
		}
	}
}

// soloRaw 从 riq 重查 cue 实体的 solo 参数。
func (m *Module) soloRaw(c cueEvt) int {
	for i := range m.ctx.Entities() {
		e := &m.ctx.Entities()[i]
		if e.Beat == c.beat && e.Game() == "cheerReaders" {
			return int(e.Float("solo", 3))
		}
	}
	return 3
}

func (m *Module) rows() [][]*girl {
	return [][]*girl{m.girls[0:4], m.girls[4:8], m.girls[8:11]}
}

func (m *Module) cue123(c cueEvt) {
	ctx := m.ctx
	b := c.beat
	solo := m.soloOf(c, 3, m.soloRaw(c))
	m.setCue(c, true)
	ctx.SoundAt(b, "bookHorizontal", 1)
	ctx.SoundAt(b+1, "bookHorizontal", 1)
	voice := [3]string{"oneTwoThreeS1", "oneTwoThreeS2", "oneTwoThreeS3"}
	gvoice := [3]string{"onegirls", "twogirls", "threegirls"}
	for i := 0; i < 3; i++ {
		if solo == 0 || solo == 2 {
			ctx.SoundAt(b+float64(i), "Solo/123/"+voice[i], 1)
		}
		if solo == 1 || solo == 2 {
			ctx.SoundAt(b+float64(i), "Girls/123/"+gvoice[i], 1)
		}
	}
	faces := [3]string{"FaceOne", "FaceTwo", "FaceThree"}
	for i := 0; i < 3; i++ {
		i := i
		ctx.At(b+float64(i), func() {
			for _, g := range m.rows()[i] {
				m.flipBook(g, true)
			}
			m.faceAll(solo, faces[i])
		})
	}
	ctx.At(b+2.5, func() {
		if !m.doing {
			m.canBop = true
		}
	})
	ctx.ScheduleInput(b+2, m.justFlip(false), m.missFlip)
}

func (m *Module) cueUpToYou(c cueEvt) {
	ctx := m.ctx
	b := c.beat
	solo := m.soloOf(c, 3, m.soloRaw(c))
	m.setCue(c, true)
	for _, t := range []float64{0, 0.75, 1.5} {
		ctx.SoundAt(b+t, "bookVertical", 1)
	}
	sv := []struct {
		t    float64
		s, g string
		off  float64
	}{
		{0, "itsUpToYouS1", "itgirls", 0},
		{0.5, "itsUpToYouS2", "sgirls", 0},
		{0.75, "itsUpToYouS3", "upgirls", 0},
		{1.5, "itsUpToYouS4", "togirls", 0},
		{2, "itsUpToYouS5", "yougirls", 0},
	}
	for _, v := range sv {
		if solo == 0 || solo == 2 {
			ctx.SoundAt(b+v.t, "Solo/UpToYou/"+v.s, 1)
		}
		if solo == 1 || solo == 2 {
			ctx.SoundAt(b+v.t, "Girls/UpToYou/"+v.g, 1)
		}
	}
	cols := []struct {
		t    float64
		col  int
		face string
		rows int
	}{
		{0, 0, "FaceIts", 3}, {0.75, 1, "FaceUp", 3}, {1.5, 2, "FaceTo", 3}, {2, 3, "FaceYou", 2},
	}
	for _, cv := range cols {
		cv := cv
		ctx.At(b+cv.t, func() {
			r := m.rows()
			for ri := 0; ri < cv.rows; ri++ {
				if cv.col < len(r[ri]) {
					m.flipBook(r[ri][cv.col], true)
				}
			}
			m.faceAll(solo, cv.face)
		})
	}
	ctx.At(b+2.5, func() {
		if !m.doing {
			m.canBop = true
		}
	})
	ctx.ScheduleInput(b+2, m.justFlip(false), m.missFlip)
}

func (m *Module) cueBooks(c cueEvt) {
	ctx := m.ctx
	b := c.beat
	solo := m.soloOf(c, 3, m.soloRaw(c))
	m.setCue(c, true)
	ctx.SoundAt(b, "letsGoRead", 1)
	for _, t := range []float64{0.75, 1, 1.25, 1.5, 1.75} {
		ctx.SoundAt(b+t, "bookDiagonal", 1)
	}
	for i := 1; i <= 9; i++ {
		t := 0.25 * float64(i)
		if i == 9 {
			t = 2.5
		} else if i == 8 {
			t = 2
		}
		if solo == 0 || solo == 2 {
			ctx.SoundAt(b+t, "Solo/LetsGoRead/bunchaBooksS"+string(rune('0'+i)), 1)
		}
		if solo == 1 || solo == 2 {
			ctx.SoundAt(b+t, "Girls/LetsGoRead/bunchaBooksgirls"+string(rune('0'+i)), 1)
		}
	}
	// 斜向连翻 + 表情节点
	type flip struct {
		t     float64
		cells [][2]int // (row, col)
		face  string
	}
	flips := []flip{
		{0, nil, "FaceIts"},
		{0.75, [][2]int{{0, 0}}, ""},
		{1, [][2]int{{0, 1}, {1, 0}}, "FaceIts"},
		{1.25, [][2]int{{0, 2}, {1, 1}, {2, 0}}, ""},
		{1.5, [][2]int{{0, 3}, {1, 2}, {2, 1}}, "FaceTo"},
		{1.75, [][2]int{{1, 3}, {2, 2}}, ""},
		{2, nil, "FaceTo"},
	}
	for _, f := range flips {
		f := f
		ctx.At(b+f.t, func() {
			r := m.rows()
			for _, cell := range f.cells {
				if cell[1] < len(r[cell[0]]) {
					m.flipBook(r[cell[0]][cell[1]], true)
				}
			}
			if f.face != "" {
				m.faceAll(solo, f.face)
			}
		})
	}
	ctx.At(b+2.5, func() {
		if !m.doing {
			m.canBop = true
		}
	})
	ctx.ScheduleInput(b+2, m.justFlip(false), m.missFlip)
}

func (m *Module) cueRah(c cueEvt) {
	ctx := m.ctx
	b := c.beat
	solo := m.soloOf(c, 4, m.soloRaw(c))
	m.setCue(c, true)
	consecutive := false
	for i := range m.ctx.Entities() {
		e := &m.ctx.Entities()[i]
		if e.Beat == c.beat && e.Datamodel == "cheerReaders/rahRahSisBoomBaBoom" {
			consecutive = boolParam(e, "consecutive")
		}
	}
	if !consecutive {
		ctx.SoundAt(b, "bookDiagonal", 1)
	}
	for _, t := range []float64{0.5, 1, 1.5, 2} {
		ctx.SoundAt(b+t, "bookDiagonal", 1)
	}
	for i := 1; i <= 6; i++ {
		t := 0.5 * float64(i-1)
		soff, goff := 0.0, 0.0
		if i == 3 {
			soff, goff = 0.081, 0.116
		}
		if solo == 0 || solo == 2 {
			ctx.SoundAtOff(b+t, "Solo/RRSBBB/rahRahSisBoomBaBoomS"+string(rune('0'+i)), 1, soff)
		}
		if solo == 1 || solo == 2 {
			ctx.SoundAtOff(b+t, "Girls/RRSBBB/rahRahSisBoomBaBoomgirls"+string(rune('0'+i)), 1, goff)
		}
	}
	type flip struct {
		t     float64
		cells [][2]int
		face  string
	}
	flips := []flip{
		{0, [][2]int{{0, 0}}, "FaceIts"},
		{0.5, [][2]int{{0, 1}, {1, 0}}, "FaceTo"},
		{1, [][2]int{{0, 2}, {1, 1}, {2, 0}}, "FaceIts"},
		{1.5, [][2]int{{0, 3}, {1, 2}, {2, 1}}, "FaceTwo"},
		{2, [][2]int{{1, 3}, {2, 2}}, "FaceTo"},
	}
	for _, f := range flips {
		f := f
		ctx.At(b+f.t, func() {
			r := m.rows()
			for _, cell := range f.cells {
				if cell[1] < len(r[cell[0]]) {
					m.flipBook(r[cell[0]][cell[1]], true)
				}
			}
			if f.face == "FaceTwo" {
				m.faceAll(solo, "FaceTwo") // OneTwoThree(2)
			} else if f.face != "" {
				m.faceAll(solo, f.face)
			}
		})
	}
	ctx.At(b+2.5, func() {
		switch solo {
		case 0:
			m.playFace(m.player, "FaceBoom", true)
		case 1:
			for _, g := range m.girls {
				m.playFace(g, "FaceBoomNPC", true)
			}
		case 2:
			m.playFace(m.player, "FaceBoom", true)
			for _, g := range m.girls {
				m.playFace(g, "FaceBoomNPC", true)
			}
		}
	})
	ctx.At(b+3.5, func() {
		if !m.doing {
			m.canBop = true
		}
	})
	ctx.ScheduleInput(b+2.5, m.justFlip(true), m.missFlip)
}

func (m *Module) cueOkItsOn(c cueEvt) {
	ctx := m.ctx
	b := c.beat
	solo := m.soloOf(c, c.length, m.soloRaw(c))
	m.setCue(c, false)
	q := c.length * 0.25
	whistle, happy := true, true
	poster := 14
	for i := range m.ctx.Entities() {
		e := &m.ctx.Entities()[i]
		if e.Beat == c.beat && e.Game() == "cheerReaders" {
			whistle = boolParam(e, "toggle")
			happy = boolParam(e, "happy")
			poster = int(e.Float("poster", 14))
		}
	}
	if whistle {
		ctx.SoundAt(b, "whistle1", 1)
		ctx.SoundAt(b+q, "whistle2", 1)
	}
	sv := []struct {
		t    float64
		s, g string
	}{
		{0, "okItsOnS1", "okItsOngirls1"},
		{q, "okItsOnS2", "okItsOngirls2"},
		{2 * q, "okItsOnS3", "okItsOngirls3"},
		{2*q + 0.75, "okItsOnS4", "okItsOngirls4"},
		{3 * q, "okItsOnS5", "okItsOngirls5"},
	}
	for _, v := range sv {
		if solo == 0 || solo == 2 {
			ctx.SoundAt(b+v.t, "Solo/OKItsOn/"+v.s, 1)
		}
		if solo == 1 || solo == 2 {
			ctx.SoundAt(b+v.t, "Girls/OKItsOn/"+v.g, 1)
		}
	}
	ctx.At(b, func() { m.faceAll(solo, "FaceTo") })
	ctx.At(b+q, func() { m.faceAll(solo, "FaceIts") })
	ctx.At(b+2*q, func() {
		for _, g := range m.girls {
			m.startSpin(g)
		}
		m.faceAll(solo, "FaceTo")
	})
	ctx.At(b+3*q, func() {
		m.setPoster(poster)
		for _, g := range m.girls {
			m.stopSpinBook(g)
		}
		m.faceAll(solo, "FaceTo")
		if happy {
			switch solo {
			case 0:
				for _, g := range m.girls {
					m.playFace(g, "FaceItsOnNPC", false)
				}
			case 1:
				m.playFace(m.player, "FaceItsOnHappy", false)
			}
		}
	})
	ctx.At(b+3*q+2, func() {
		if !happy {
			return
		}
		if !m.doing {
			for _, g := range m.girls {
				m.playFace(g, "FaceItsOnNPC", false)
			}
			m.playFace(m.player, "FaceItsOnHappy", false)
		}
	})
	// 输入：B+len/2 按住（南键=动作 1），B+3len/4 松开
	ctx.ScheduleInputAction(b+2*q, 1,
		func(st float64, _ engine.Judgment) { m.justHold(st) },
		m.missFlip)
	ctx.ScheduleInputRelease(b+3*q,
		func(st float64, _ engine.Judgment) { m.justRelease(st) },
		func() {
			if m.player.spinning { // IsHittable 语义：未在转书则该判定不生效
				m.missFlip()
			}
		})
}

func (m *Module) setPoster(idx int) {
	if idx >= len(posterFiles) || idx < 0 {
		idx = rand.Intn(len(posterFiles))
	}
	sc := m.ctx.Scene
	file := posterFiles[idx]
	pick := func(part string) string {
		if _, ok := m.ctx.Assets.Sheet.Sprites[file+"/"+part]; ok {
			return file + "/" + part
		}
		return part // Crop Stomp（首扫描文件）未命名空间化
	}
	sc.SetSpriteOver(m.ctx.Role("topPoster"), pick("TopPart"))
	sc.SetSpriteOver(m.ctx.Role("middlePoster"), pick("MiddlePart"))
	sc.SetSpriteOver(m.ctx.Role("bottomPoster"), pick("BottomPart"))
	sc.SetSpriteOver(m.ctx.Role("missPoster"), pick("Miss"))
}

// ---------- 判定 ----------

func (m *Module) justFlip(boom bool) func(float64, engine.Judgment) {
	return func(state float64, _ engine.Judgment) {
		m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
		if state >= 1 || state <= -1 {
			m.ctx.Sound("doingoing")
			m.flipBook(m.player, true)
			return
		}
		m.flipBook(m.player, true)
		m.yay = true
		if boom {
			m.ctx.Sound("bookBoom")
		} else {
			m.ctx.Sound("bookPlayer")
		}
	}
}

func (m *Module) justHold(state float64) {
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
	m.startSpin(m.player)
	if state >= 1 || state <= -1 {
		m.ctx.Sound("doingoing")
		m.stopSpin()
		m.stopSpin = m.ctx.SoundLoop("bookSpinLoop")
		return
	}
	m.ctx.Sound("bookSpin")
	m.stopSpin()
	m.stopSpin = m.ctx.SoundLoop("bookSpinLoop")
}

func (m *Module) justRelease(state float64) {
	m.stopSpin()
	m.stopSpin = func() {}
	if state >= 1 || state <= -1 {
		m.ctx.Sound("doingoing")
		m.stopSpinBook(m.player)
		m.zoomOK = false
		m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), true)
		return
	}
	m.ctx.Sound("bookOpen")
	m.stopSpinBook(m.player)
	m.yay = true
	m.zoomOK = true
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
}

func (m *Module) missFlip() {
	ctx := m.ctx
	ctx.Scene.SetActive(ctx.Role("playerMask"), false)
	ctx.Scene.SetActive(ctx.Role("missPoster"), false)
	ctx.Sound("doingoing")
	m.stopSpin()
	m.stopSpin = func() {}
	m.missChar(m.player)
	m.zoomOK = false
	for _, g := range m.girls {
		face := "FaceNPCLook1"
		if g.lookDown {
			face = "FaceNPCLook2"
		}
		m.playFace(g, face, false)
	}
}

// ---------- yay / 杂项 ----------

func (m *Module) doYay(solo int) {
	if !m.yay {
		return
	}
	m.spawnConfetti(m.black)
	m.hideMasks()
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
	switch solo {
	case 0:
		m.ctx.Sound("Solo/yayS")
		m.yayChar(m.player, true)
		for _, g := range m.girls {
			m.yayChar(g, true)
		}
	case 1:
		m.ctx.Sound("Girls/yayGirls")
		for _, g := range m.girls {
			m.yayChar(g, true)
		}
		m.yayChar(m.player, false)
	default:
		m.ctx.Sound("All/yay")
		for _, g := range m.girls {
			m.yayChar(g, true)
		}
		m.yayChar(m.player, true)
	}
}

func (m *Module) spawnConfetti(black bool) {
	t := m.lastT
	for i := 0; i < 30; i++ {
		ang := rand.Float64() * 2 * math.Pi
		sp := 4 + rand.Float64()*5
		m.particles = append(m.particles, confetti{
			x: 0, y: 0, vx: math.Cos(ang) * sp, vy: math.Sin(ang)*sp + 3,
			born: t, black: black,
		})
	}
}

func (m *Module) resetPose() {
	m.canBop = true
	for _, g := range m.all() {
		m.play(g, g.baseColor(g.white)+"Idle", false)
		m.playFace(g, "FaceIdle", false)
		g.noBop, g.missed, g.spinning, g.open = false, false, false, false
	}
	m.hideMasks()
}

func (m *Module) bopAll() {
	if !m.canBop {
		return
	}
	for _, g := range m.all() {
		m.bopChar(g)
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, be := range m.bops {
		if be.beat > beat {
			break
		}
		on = be.auto
	}
	return on
}

// ---------- 引擎接口 ----------

func (m *Module) OnSwitch(beat float64) {
	m.canBop = true
	m.hideMasks()
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
}

func (m *Module) Whiff(beat float64) {
	// BasicPress 空按：玩家翻书失败 + miss 音 + ScoreMiss(1)
	m.flipBook(m.player, false)
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
	m.ctx.PlayCommon("miss")
	m.ctx.ScoreMiss()
}

// WhiffAction：南键按下/松开的空按（StartSpin / StopSpin + ScoreMiss）。
func (m *Module) WhiffAction(beat float64, action int) {
	if action != 1 {
		m.Whiff(beat)
		return
	}
	m.ctx.Sound("doingoing")
	m.startSpin(m.player)
	m.ctx.Scene.SetActive(m.ctx.Role("missPoster"), false)
	m.stopSpin()
	m.stopSpin = m.ctx.SoundLoop("bookSpinLoop")
	m.ctx.ScoreMiss()
}

func (m *Module) Update(t, beat float64) {
	m.lastT = t
	// doingCue 区间折叠
	if m.cueLen > 0 {
		norm := (beat - m.cueBeat) / m.cueLen
		if norm >= 0 {
			m.doing = norm <= 1
		}
	}
	// blush 超时 + missed 表情（RvlCharacter.Update）
	for _, g := range m.all() {
		if g.blushBeat > -5 && beat-g.blushBeat > 3 {
			m.ctx.Scene.SetActive(g.face+"/Blush", false)
			m.ctx.Scene.SetActive(g.face+"/Blush (1)", false)
			g.blushBeat = -10
		}
		if !m.doing && g.missed {
			m.playFace(g, "FaceBlush", false)
			g.missed = false
		}
	}
	// 自动 bop（OnBeatPulse + BeatIsInBopRegion）
	if p := math.Floor(beat); p == beat || beat-p < 0.02 {
		if m.autoBopAt(p) && m.lastBop != p {
			m.lastBop = p
			m.bopAll()
		}
	}
	// okItsOn 冲击变焦（Update 的三段 EaseQuint）
	m.updateZoom(beat)
}

func (m *Module) updateZoom(beat float64) {
	if len(m.zooms) == 0 {
		return
	}
	if m.zoomIdx < len(m.zooms) && beat >= m.zooms[m.zoomIdx].beat {
		if m.zoomIdx+1 < len(m.zooms) && beat >= m.zooms[m.zoomIdx+1].beat {
			m.zoomIdx++
		}
	}
	z := m.zooms[min(m.zoomIdx, len(m.zooms)-1)]
	q := z.length * 0.25
	peak := 1.5
	if m.zoomOK {
		peak = 4
	}
	normOut := norm01(beat, z.beat+2*q, q)
	normIn := norm01(beat, z.beat+3*q, 0.1)
	normOutAgain := norm01(beat, z.beat+3*q+0.1, 1.1)
	switch {
	case normOutAgain >= 0:
		if normOutAgain > 1 {
			m.zoomAdd = 0
		} else {
			m.zoomAdd = engine.Ease(13, peak, 0, normOutAgain) // EaseInOutQuint
		}
	case normIn >= 0:
		if normIn > 1 {
			m.zoomAdd = peak
		} else {
			m.zoomAdd = engine.Ease(12, -1, peak, normIn) // EaseOutQuint
		}
	case normOut >= 0:
		if normOut > 1 {
			m.zoomAdd = -1
		} else {
			m.zoomAdd = -engine.Ease(12, 0, 1, normOut)
		}
	}
}

func norm01(beat, start, length float64) float64 {
	if length <= 0 {
		return -1
	}
	return (beat - start) / length
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	cam := m.ctx.CameraAt(beat)
	// GameCamera.AdditionalPosition.z（冲击变焦）
	sc.SetCamera(cam[0], cam[1], cam[2]+m.zoomAdd)
	sc.Sample(beat)
	sc.Draw(screen, m.proj)
	m.drawConfetti(screen, t)
}

// 主题色 "ffffde"
var bgColor = colorRGBA{255, 255, 222, 255}

func (m *Module) drawConfetti(screen *ebiten.Image, t float64) {
	alive := m.particles[:0]
	for _, p := range m.particles {
		age := t - p.born
		if age > 1.2 {
			continue
		}
		alive = append(alive, p)
		x := p.x + p.vx*age
		y := p.y + p.vy*age - 4*age*age
		px := float64(engine.ScreenW)/2 + x*54
		py := float64(engine.ScreenH)/2 - y*54
		c := colorRGBA{250, 250, 250, 255}
		if p.black {
			c = colorRGBA{40, 40, 40, 255}
		}
		// 简易方片粒子（whiteYayParticle/blackYayParticle 等价）
		half := 4.0
		ebitenFillRect(screen, px-half, py-half, half*2, half*2, c)
	}
	m.particles = alive
}

type colorRGBA struct{ R, G, B, A uint8 }

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	return uint32(c.R) * 0x101, uint32(c.G) * 0x101, uint32(c.B) * 0x101, uint32(c.A) * 0x101
}

var whitePix *ebiten.Image

func ebitenFillRect(dst *ebiten.Image, x, y, w, h float64, c colorRGBA) {
	if whitePix == nil {
		whitePix = ebiten.NewImage(1, 1)
		whitePix.Fill(colorRGBA{255, 255, 255, 255})
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w, h)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
	dst.DrawImage(whitePix, op)
}
