// hsdemo：加载 .riq 谱面并游玩其中 karateman/hit 事件的最小 demo，
// 使用从 Heaven Studio 工程提取的真实资产（图集 / 骨架动画 / 音效）。
//
// 数据流：
//
//	demo.riq ──riq.Load──> Beatmap{tempo map, entities}
//	assets/karateman/ ──kart.Load──> 图集 + Joe 骨架 + 动画曲线 + 音效 PCM
//	     音频字节 ──解码──> audio.Player ──> conductor（采样时钟+平滑）
//	     entities ──预计算 spawn/hit 时刻──> 判定 / 渲染（单位空间→proj→屏幕）
//
// 操作：Space/J/鼠标左键 出拳；Tab 调试叠层；R 重开；Esc 退出。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"path/filepath"

	"hsdemo/conductor"
	"hsdemo/engine"
	"hsdemo/games/airrally"
	"hsdemo/games/basketballgirls"
	"hsdemo/games/bluebear"
	"hsdemo/games/bouncyroad"
	"hsdemo/games/catchytune"
	"hsdemo/games/cheerreaders"
	"hsdemo/games/cointoss"
	"hsdemo/games/drummingpractice"
	"hsdemo/games/kitties"
	"hsdemo/games/lockstep"
	"hsdemo/games/marchingorders"
	"hsdemo/games/meatgrinder"
	"hsdemo/games/munchymonk"
	"hsdemo/games/seesaw"
	"hsdemo/games/sneakyspirits"
	"hsdemo/games/somen"
	"hsdemo/games/spacedance"
	"hsdemo/games/tambourine"
	"hsdemo/games/taptrial"
	"hsdemo/games/totemclimb"
	"hsdemo/games/trickclass"
	"hsdemo/games/wizardswaltz"
	"hsdemo/kart"
	"hsdemo/riq"
	"hsdemo/synth"
)

const (
	screenW = 960
	screenH = 540

	sampleRate = synth.SampleRate

	// 判定窗口（秒）。参考原版 Minigame.cs 的量级，demo 取整。
	aceWindow  = 0.06
	goodWindow = 0.12

	// 舞台（单位空间，y 向上，原点 = Joe 骨架根；
	// 判定点/地面/抛物线参数来自 stage.json，即原版 KarateManPot 的序列化字段）
	joeScreenX = 310.0 // Joe 根的屏幕 x
	groundY    = 470.0 // stage 地面（FloorY）对应的屏幕 y
	rigFitH    = 330.0 // 骨架包围盒映射到的屏幕高度（像素）

	inSpinDeg = 125.0 // 入场自转角速度（度/拍，原版 KarateManPot.cs）
	outSpin   = -9.0  // 被击飞后的自转角速度（rad/s）
	camD      = 10.0  // 相机到舞台平面的距离（GameCamera.defaultPosition z=-10）
)

type gameState int

const (
	stateTitle gameState = iota
	statePlay
	stateResult
)

type judgment int

const (
	judgeNone judgment = iota
	judgeAce
	judgeGood
	judgeMiss
)

// pot 是一个待击打目标。原版语义：事件 beat 是抛出拍，判定在 beat+1，
// 全程飞行 2 拍（抛物线，顶点过后在判定点被击中）。
type pot struct {
	throwBeat float64
	spawnT    float64 // = BeatToTime(throwBeat)
	hitT      float64 // = BeatToTime(throwBeat+1)
	rot0      float64 // 初始随机朝向（原版 Awake 里 Random.Range(0,360)）
	thrown    bool    // 已播放抛出音效
	result    judgment
	judgeT    float64
}

type Game struct {
	bm     *riq.Beatmap
	cond   *conductor.Conductor
	player *audio.Player
	actx   *audio.Context

	as   *kart.Assets
	joe  *kart.RigInst
	proj kart.Aff // 单位空间 → 屏幕
	unit float64  // 像素/单位（proj 的缩放）

	pots    []*pot
	endTime float64

	state    gameState
	punchT   float64
	lastBeat int
	lastMsg  string
	msgT     float64

	combo, maxCombo  int
	aces, goods      int
	misses           int
	score            int
	debug            bool
	skippedDatamodel map[string]int

	faceBig, faceMid, faceSmall *text.GoTextFace
}

func newGame(r *riq.Riq, assetsDir string) (*Game, error) {
	actx := audio.NewContext(sampleRate)

	var (
		stream io.Reader
		err    error
	)
	br := bytes.NewReader(r.Audio)
	switch r.AudioFormat {
	case riq.AudioWAV:
		stream, err = wav.DecodeWithSampleRate(sampleRate, br)
	case riq.AudioOGG:
		stream, err = vorbis.DecodeWithSampleRate(sampleRate, br)
	case riq.AudioMP3:
		stream, err = mp3.DecodeWithSampleRate(sampleRate, br)
	default:
		return nil, fmt.Errorf("unsupported audio format in riq (file %s)", r.AudioName)
	}
	if err != nil {
		return nil, fmt.Errorf("decode audio: %w", err)
	}

	player, err := actx.NewPlayer(stream)
	if err != nil {
		return nil, fmt.Errorf("create player: %w", err)
	}
	player.SetVolume(0.85)

	as, err := kart.Load(assetsDir, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("load assets: %w", err)
	}

	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}

	g := &Game{
		bm:               r.Beatmap,
		cond:             conductor.New(r.Beatmap, player),
		player:           player,
		actx:             actx,
		as:               as,
		joe:              kart.NewRig(as),
		lastBeat:         -1,
		skippedDatamodel: map[string]int{},
		faceBig:          &text.GoTextFace{Source: src, Size: 44},
		faceMid:          &text.GoTextFace{Source: src, Size: 24},
		faceSmall:        &text.GoTextFace{Source: src, Size: 15},
	}

	// 镜头标定：骨架包围盒拟合到 rigFitH 像素；舞台地面（stage.FloorY）贴 groundY
	_, minY, _, maxY := g.joe.BBox()
	g.unit = rigFitH / (maxY - minY)
	g.proj = kart.Translate(joeScreenX, groundY+g.unit*as.Stage.FloorY).Mul(kart.Scale(g.unit, -g.unit))

	g.buildPots()
	return g, nil
}

// buildPots 把谱面实体翻译为游戏对象；非 karateman/hit 的事件计数后忽略。
func (g *Game) buildPots() {
	g.pots = g.pots[:0]
	lastBeat := 0.0
	for i := range g.bm.Entities {
		e := &g.bm.Entities[i]
		if e.Beat+e.Length > lastBeat {
			lastBeat = e.Beat + e.Length
		}
		if e.Datamodel != "karateman/hit" {
			g.skippedDatamodel[e.Datamodel]++
			continue
		}
		g.pots = append(g.pots, &pot{
			throwBeat: e.Beat,
			spawnT:    g.bm.BeatToTime(e.Beat),
			hitT:      g.bm.BeatToTime(e.Beat + 1),
			rot0:      rand.Float64() * 2 * math.Pi,
		})
	}
	sort.Slice(g.pots, func(i, j int) bool { return g.pots[i].hitT < g.pots[j].hitT })
	g.endTime = g.bm.BeatToTime(lastBeat + 5)
}

func (g *Game) resetRun() error {
	if err := g.cond.Reset(); err != nil {
		return err
	}
	for _, p := range g.pots {
		p.result, p.judgeT, p.thrown = judgeNone, 0, false
	}
	g.combo, g.maxCombo, g.aces, g.goods, g.misses, g.score = 0, 0, 0, 0, 0, 0
	g.lastMsg, g.msgT, g.punchT, g.lastBeat = "", 0, 0, -1
	g.state = stateTitle
	return nil
}

func (g *Game) playSound(name string) {
	if pcm, ok := g.as.Sounds[name]; ok {
		g.actx.NewPlayerFromBytes(pcm).Play()
	}
}

// ---------- Update ----------

func (g *Game) Update() error {
	if engine.HandleFullscreenShortcut() {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.debug = !g.debug
	}

	switch g.state {
	case stateTitle:
		if punchPressed() {
			g.cond.Play()
			g.state = statePlay
		}
	case statePlay:
		g.cond.Update()
		g.updatePlay()
	case stateResult:
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			return g.resetRun()
		}
	}
	return nil
}

func punchPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

func (g *Game) updatePlay() {
	t := g.cond.Time()

	// 节拍 bop：出拳动画结束后空闲时，每拍播一次 Beat
	if b := int(math.Floor(g.cond.Beat())); b != g.lastBeat && b >= 0 {
		g.lastBeat = b
		if t-g.punchT > g.jabDur() {
			g.joe.Play("Beat", t)
		}
	}

	if punchPressed() {
		g.punchT = t
		g.joe.Play("Jab", t)
		g.judgePunch(t)
	}

	for _, p := range g.pots {
		if !p.thrown && t >= p.spawnT {
			p.thrown = true
			g.playSound("objectOut")
		}
		// 超窗未击 → miss
		if p.result == judgeNone && t > p.hitT+goodWindow {
			p.result = judgeMiss
			p.judgeT = t
			g.misses++
			g.combo = 0
			g.setMsg("MISS...")
			g.playSound("karate_through")
		}
	}

	if t > g.endTime {
		g.cond.Pause()
		g.state = stateResult
	}
}

func (g *Game) jabDur() float64 {
	if a, ok := g.as.Anims["Jab"]; ok {
		return a.Duration
	}
	return 0.28
}

// judgePunch 找时间上最近的未判定 pot，落窗则判定。
func (g *Game) judgePunch(t float64) {
	var best *pot
	bestDiff := math.Inf(1)
	for _, p := range g.pots {
		if p.result != judgeNone {
			continue
		}
		d := math.Abs(t - p.hitT)
		if d < bestDiff {
			best, bestDiff = p, d
		}
	}
	if best == nil || bestDiff > goodWindow {
		g.playSound("swingNoHit") // 空挥
		return
	}
	best.judgeT = t
	g.combo++
	if g.combo > g.maxCombo {
		g.maxCombo = g.combo
	}
	if bestDiff <= aceWindow {
		best.result = judgeAce
		g.aces++
		g.score += 100
		g.setMsg("ACE!!")
		g.playSound("potHit")
	} else {
		best.result = judgeGood
		g.goods++
		g.score += 50
		g.setMsg("OK")
		g.playSound("punchKickHit1")
	}
}

func (g *Game) setMsg(s string) {
	g.lastMsg = s
	g.msgT = g.cond.Time()
}

// ---------- Draw ----------

var (
	colBG     = color.RGBA{0xfb, 0xca, 0x3e, 255} // 原版 KarateMan 默认底色
	colBGDeep = color.RGBA{0xe8, 0xb2, 0x2a, 255}
	colGround = color.RGBA{0xd8, 0x9f, 0x1d, 255}
	colAce    = color.RGBA{40, 150, 70, 255}
	colGood   = color.RGBA{30, 90, 180, 255}
	colMiss   = color.RGBA{200, 40, 40, 255}
	colText   = color.RGBA{60, 40, 10, 255}
	colDim    = color.RGBA{120, 95, 40, 255}
)

func (g *Game) Draw(screen *ebiten.Image) {
	beat := g.cond.Beat()
	pulse := 0.0
	if g.state == statePlay {
		frac := beat - math.Floor(beat)
		pulse = math.Max(0, 1-frac*4)
	}
	screen.Fill(lerpColor(colBG, colBGDeep, pulse*0.5))
	vector.DrawFilledRect(screen, 0, groundY, screenW, screenH-groundY, colGround, false)

	t := g.cond.Time()
	g.joe.Sample(t)
	g.joe.Draw(screen, g.proj)
	g.drawPots(screen, t)

	switch g.state {
	case stateTitle:
		g.drawTitle(screen)
	case statePlay:
		g.drawHUD(screen, beat)
	case stateResult:
		g.drawResult(screen)
	}

	if g.debug {
		g.drawDebug(screen, beat)
	}
}

// potFlight 复刻原版 KarateManPot.ProgressToFlyPosition：
// x/z 在起点/终点（判定点 ± StartOffset）间线性，y 走归一化抛物线，
// 抛物线在判定时刻（throwBeat+1，progress=0.5）恰好经过拳头位置。
// z 跨度 ±9：罐子从镜头近处（大）飞向场景深处（小），判定时回到舞台平面。
func (g *Game) potFlight(p *pot, beat float64) (x, y, z, rot float64) {
	st := &g.as.Stage
	elapsed := beat - p.throwBeat
	progress := math.Max(elapsed/2, 0)
	rotCap := 2 * (1 - st.Slip) // 落地（slip 点）后停止自转
	if progress > 1-st.Slip {
		progress = 1 - st.Slip
	}

	pHit := progress + (st.HitOffset - 0.5)
	flyH := pHit * (pHit - 1) / (st.HitOffset * (st.HitOffset - 1))

	startX := st.HitPos[0] + st.StartOffset[0]
	endX := st.HitPos[0] - st.StartOffset[0]
	x = startX + (endX-startX)*progress

	rise := math.Min(math.Max(elapsed, 0), 1)
	y = st.FloorY + (st.HitPos[1]-st.FloorY+st.StartOffset[1]*(1-rise))*flyH
	if progress >= 0.5 && y < st.FloorY {
		y = st.FloorY
	}

	z = st.StartOffsetZ * (1 - 2*progress) // start: +z0 → end: -z0

	rot = p.rot0 + inSpinDeg*math.Pi/180*math.Min(math.Max(elapsed, 0), rotCap)
	return
}

func (g *Game) drawPots(screen *ebiten.Image, t float64) {
	beat := g.cond.Beat()
	st := &g.as.Stage
	for _, p := range g.pots {
		switch p.result {
		case judgeNone, judgeMiss:
			if t < p.spawnT {
				continue
			}
			alpha := 1.0
			if p.result == judgeMiss {
				alpha = 1 - (t-p.judgeT)/0.6 // 漏拍后淡出
				if alpha <= 0 {
					continue
				}
			}
			x, y, z, rot := g.potFlight(p, beat)
			// 透视近似：以判定点为视轴，按深度缩放位置与大小
			s := camD / (camD + z)
			if s <= 0 || s > 12 {
				continue // 几乎贴脸/在相机后，视锥外
			}
			drawX := st.HitPos[0] + (x-st.HitPos[0])*s
			drawY := st.HitPos[1] + (y-st.HitPos[1])*s
			g.drawPotShadow(screen, drawX, s)
			world := kart.Translate(drawX, drawY).Mul(kart.Rotate(rot)).Mul(kart.Scale(s, s))
			g.as.DrawSprite(screen, "karateman_pot", world, g.proj, false, float32(alpha))
		case judgeAce, judgeGood:
			dt := t - p.judgeT
			if dt > 1.0 {
				continue
			}
			// 击飞：向左上抛物线 + 自转
			x := st.HitPos[0] - 14*dt
			y := st.HitPos[1] + 10*dt - 14*dt*dt
			world := kart.Translate(x, y).Mul(kart.Rotate(p.rot0 + outSpin*dt))
			g.as.DrawSprite(screen, "karateman_pot", world, g.proj, false, 1)
			// 命中闪光
			if dt < 0.15 {
				s := 1 + 3*dt
				fx := kart.Translate(st.HitPos[0]+0.3, st.HitPos[1]).Mul(kart.Scale(s, s))
				g.as.DrawSprite(screen, "karateman_hiteffect_0", fx, g.proj, false, float32(1-dt/0.15))
			}
		}
	}
}

func (g *Game) drawPotShadow(screen *ebiten.Image, x, s float64) {
	st := &g.as.Stage
	y := st.HitPos[1] + (st.FloorY+0.05-st.HitPos[1])*s // 地面点随同一透视缩放
	world := kart.Translate(x, y).Mul(kart.Scale(s, s))
	g.as.DrawSprite(screen, "karateman_object_shadow", world, g.proj, false, 0.6)
}

func (g *Game) drawHUD(screen *ebiten.Image, beat float64) {
	t := g.cond.Time()

	if g.lastMsg != "" && t-g.msgT < 0.6 {
		c := colText
		switch g.lastMsg {
		case "ACE!!":
			c = colAce
		case "OK":
			c = colGood
		case "MISS...":
			c = colMiss
		}
		g.text(screen, g.lastMsg, g.faceBig, joeScreenX+150, 150, c, true)
	}

	cur := int(math.Floor(beat)) % 4
	for i := 0; i < 4; i++ {
		c := color.RGBA{0xc9, 0x9c, 0x20, 255}
		if i == cur && beat >= 0 {
			c = colText
		}
		vector.DrawFilledCircle(screen, float32(screenW/2-45+i*30), 34, 8, c, true)
	}

	prog := math.Min(t/g.endTime, 1)
	vector.DrawFilledRect(screen, 0, 0, float32(screenW*prog), 4, colAce, false)

	g.text(screen, fmt.Sprintf("SCORE %d", g.score), g.faceMid, 20, 20, colText, false)
	if g.combo >= 2 {
		g.text(screen, fmt.Sprintf("%d COMBO", g.combo), g.faceMid, screenW/2, screenH-46, colText, true)
	}
	g.text(screen, fmt.Sprintf("BPM %.0f", g.bm.BPMAt(beat)), g.faceSmall, screenW-110, 20, colDim, false)
}

func (g *Game) drawTitle(screen *ebiten.Image) {
	g.text(screen, "HEAVEN GO DEMO", g.faceBig, screenW/2, 110, colText, true)
	g.text(screen, "Karate Man (riq runtime port)", g.faceMid, screenW/2, 170, colDim, true)
	g.text(screen, fmt.Sprintf("chart: %d hits | %.0f BPM start | offset %.2fs",
		len(g.pots), g.bm.Tempos[0].BPM, g.bm.Offset), g.faceSmall, screenW/2, 212, colDim, true)
	g.text(screen, "Space / J / Click — punch on the beat", g.faceMid, screenW/2, screenH-110, colText, true)
	g.text(screen, "press to start", g.faceMid, screenW/2, screenH-72, colAce, true)
}

func (g *Game) drawResult(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 140}, false)

	total := len(g.pots)
	rank := "TRY AGAIN"
	c := color.RGBA{255, 120, 120, 255}
	switch {
	case g.misses == 0 && g.goods <= total/8:
		rank, c = "SUPERB!!", color.RGBA{140, 255, 170, 255}
	case g.misses <= total/6:
		rank, c = "OK", color.RGBA{140, 200, 255, 255}
	}
	white := color.RGBA{235, 235, 245, 255}
	grey := color.RGBA{170, 170, 185, 255}
	g.text(screen, rank, g.faceBig, screenW/2, 160, c, true)
	g.text(screen, fmt.Sprintf("ACE %d   OK %d   MISS %d", g.aces, g.goods, g.misses),
		g.faceMid, screenW/2, 240, white, true)
	g.text(screen, fmt.Sprintf("score %d   max combo %d", g.score, g.maxCombo),
		g.faceMid, screenW/2, 280, grey, true)
	g.text(screen, "R — restart    Esc — quit", g.faceMid, screenW/2, 360, white, true)
}

func (g *Game) drawDebug(screen *ebiten.Image, beat float64) {
	lines := []string{
		fmt.Sprintf("songPos  %8.3fs", g.cond.Time()),
		fmt.Sprintf("audioPos %8.3fs", g.player.Position().Seconds()),
		fmt.Sprintf("drift    %+7.1fms", g.cond.Drift()*1000),
		fmt.Sprintf("beat     %8.3f", beat),
		fmt.Sprintf("unit     %8.2f px", g.unit),
		fmt.Sprintf("tps      %8.1f", ebiten.ActualTPS()),
		fmt.Sprintf("fps      %8.1f", ebiten.ActualFPS()),
	}
	for k, v := range g.skippedDatamodel {
		lines = append(lines, fmt.Sprintf("skipped  %s x%d", k, v))
	}
	for i, s := range lines {
		g.text(screen, s, g.faceSmall, 20, 60+float64(i)*18, colText, false)
	}
}

func (g *Game) text(screen *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color, center bool) {
	if center {
		w, _ := text.Measure(s, face, 0)
		x -= w / 2
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

func (g *Game) Layout(_, _ int) (int, int) { return screenW, screenH }

// ---------- main ----------

// detectGame 根据谱面实体推断主 minigame（取出现次数最多的游戏前缀）。
func detectGame(bm *riq.Beatmap) string {
	counts := map[string]int{}
	for i := range bm.Entities {
		game := bm.Entities[i].Game()
		switch game {
		case "gameManager", "vfx", "countIn", "global":
			continue
		}
		counts[game]++
	}
	best, n := "", 0
	for g, c := range counts {
		if c > n {
			best, n = g, c
		}
	}
	return best
}

func main() {
	path := flag.String("riq", "", ".riq 谱面路径（留空则进入标题屏等待拖放）")
	assetsRoot := flag.String("assets", "assets", "提取资产根目录")
	latency := flag.Float64("latency", 0, "输入延迟校准（毫秒，可在游戏内用 [ ] 微调）")
	autoplay := flag.Bool("autoplay", false, "完美自动打击（调试/验证用）")
	fullscreen := flag.Bool("fullscreen", false, "启动时进入全屏；运行中可用 F11 / Alt+Enter 切换")
	flag.Parse()

	// 已移植的游戏模块
	engine.Register("rhythmSomen", somen.New)
	engine.Register("airRally", airrally.New)
	engine.Register("basketballGirls", basketballgirls.New)
	engine.Register("bouncyRoad", bouncyroad.New)
	engine.Register("catchyTune", catchytune.New)
	engine.Register("coinToss", cointoss.New)
	engine.Register("drummingPractice", drummingpractice.New)
	engine.Register("tambourine", tambourine.New)
	engine.Register("tapTrial", taptrial.New)
	engine.Register("trickClass", trickclass.New)
	engine.Register("meatGrinder", meatgrinder.New)
	engine.Register("totemClimb", totemclimb.New)
	engine.Register("seeSaw", seesaw.New)
	engine.Register("sneakySpirits", sneakyspirits.New)
	engine.Register("blueBear", bluebear.New)
	engine.Register("marchingOrders", marchingorders.New)
	engine.Register("cheerReaders", cheerreaders.New)
	engine.Register("kitties", kitties.New)
	engine.Register("lockstep", lockstep.New)
	engine.Register("spaceDance", spacedance.New)
	engine.Register("munchyMonk", munchymonk.New)
	engine.Register("wizardsWaltz", wizardswaltz.New)

	// karateman 仍走早期 demo 路径（未迁移到 engine）
	if *path != "" {
		if r, err := riq.Load(*path); err == nil && detectGame(r.Beatmap) == "karateman" {
			g, err := newGame(r, filepath.Join(*assetsRoot, "karateman"))
			if err != nil {
				log.Fatal(err)
			}
			engine.ConfigureWindow("Heaven Go — Karate Man (legacy)", *fullscreen)
			ebiten.SetTPS(240)
			if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
				log.Fatal(err)
			}
			return
		}
	}

	app, err := engine.New(*assetsRoot, *path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	app.LatencyMS = *latency
	app.Autoplay = *autoplay

	engine.ConfigureWindow("Heaven Go", *fullscreen)
	// 提高逻辑帧率，把输入采样量化误差压到 ~±2ms（60Hz 下是 ±8ms）
	ebiten.SetTPS(240)

	if err := ebiten.RunGame(app); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
