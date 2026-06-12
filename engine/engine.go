// engine.go：App 外壳——谱面加载/重载（含拖放导入）、音频、conductor、
// 时间轴执行、输入判定、游戏切换、HUD/时机条/flash、结算。
package engine

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/conductor"
	"hsdemo/riq"
)

const (
	ScreenW = 960
	ScreenH = 540

	SampleRate = 44100

	// 判定窗口（秒），对应 Minigame.cs 的 ace/just/ngTimeBase
	WinAce  = 0.01
	WinJust = 0.05
	WinNG   = 0.10
)

type gameState int

const (
	stateTitle gameState = iota
	statePlay
	stateResult
)

type beatAction struct {
	beat float64
	fn   func()
}

// Input 是一次已调度的输入判定（HS PlayerActionEvent 等价物）。
type Input struct {
	Beat   float64
	hitT   float64
	judged bool
	Result Judgment
	// OnHit 在 NG 窗口内的任意按键触发；state 为 just 窗归一化偏移
	//（|state|<=1 = just 命中，1<|state|<=2 = NG，负 = 早），与 C# 语义一致。
	OnHit func(state float64, j Judgment)
	// OnMiss 在超窗未按时触发。
	OnMiss func()
}

type flashEvt struct {
	beat, length float64
	c0, c1       [4]float64
}

type gameSwitch struct {
	beat float64
	id   string
}

type timingHit struct {
	y      float64
	rating Judgment
	t      float64
}

var audioCtx *audio.Context // audio.NewContext 进程内只能调用一次

// App 实现 ebiten.Game。
type App struct {
	assetsRoot string

	// 谱面态（importRiq 时整体重建）
	r        *riq.Riq
	bm       *riq.Beatmap
	cond     *conductor.Conductor
	player   *audio.Player
	modules  map[string]Module
	active   Module
	switches []gameSwitch
	swIdx    int
	actions  []beatAction
	actIdx   int
	inputs   []*Input
	flashes  []flashEvt
	unported []string

	state   gameState
	inputOn bool

	starBeat float64
	starGot  bool
	endBeat  float64

	aces, justs, ngs, misses, whiffs int
	lastMsg                          string
	msgT                             float64
	debug                            bool
	loadErr                          string

	tdArrow, tdTarget float64
	tdHits            []timingHit

	// LatencyMS：输入延迟校准（毫秒，正值 = 按键时间戳前移），'['/']' 热键 ±5ms
	LatencyMS float64

	faceBig, faceMid, faceSmall *text.GoTextFace
}

// New 创建 App 并加载初始谱面（path 可为空：进入等待拖放的标题屏）。
func New(assetsRoot, riqPath string) (*App, error) {
	if audioCtx == nil {
		audioCtx = audio.NewContext(SampleRate)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	a := &App{
		assetsRoot: assetsRoot,
		faceBig:    &text.GoTextFace{Source: src, Size: 44},
		faceMid:    &text.GoTextFace{Source: src, Size: 24},
		faceSmall:  &text.GoTextFace{Source: src, Size: 15},
	}
	if riqPath != "" {
		r, err := riq.Load(riqPath)
		if err != nil {
			return nil, err
		}
		if err := a.loadRiq(r); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ---------- 谱面装载 ----------

func (a *App) loadRiq(r *riq.Riq) error {
	// 解码音乐
	var (
		stream io.Reader
		err    error
	)
	br := bytes.NewReader(r.Audio)
	switch r.AudioFormat {
	case riq.AudioWAV:
		stream, err = wav.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioOGG:
		stream, err = vorbis.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioMP3:
		stream, err = mp3.DecodeWithSampleRate(SampleRate, br)
	default:
		return fmt.Errorf("不支持的音频格式（%s）", r.AudioName)
	}
	if err != nil {
		return fmt.Errorf("解码音乐失败: %w", err)
	}
	player, err := audioCtx.NewPlayer(stream)
	if err != nil {
		return err
	}
	player.SetVolume(0.85)

	if a.player != nil {
		a.player.Close()
	}

	a.r, a.bm = r, r.Beatmap
	a.player = player
	a.cond = conductor.New(r.Beatmap, player)
	a.modules = map[string]Module{}
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.flashes = nil
	a.unported = nil
	a.starBeat, a.endBeat = -1, 0
	a.resetRunState()

	// 找出谱面用到的游戏并实例化模块
	used := map[string]bool{}
	for i := range a.bm.Entities {
		e := &a.bm.Entities[i]
		if g, ok := strings.CutPrefix(e.Datamodel, "gameManager/switchGame/"); ok {
			a.switches = append(a.switches, gameSwitch{e.Beat, g})
			used[g] = true
			continue
		}
		switch e.Game() {
		case "gameManager", "vfx", "countIn", "global":
			continue
		}
		used[e.Game()] = true
	}
	for id := range used {
		var m Module
		if f, ok := registry[id]; ok {
			m = f()
		} else {
			m = newPlaceholder(id)
			a.unported = append(a.unported, id)
		}
		ctx := &Ctx{App: a, module: m}
		if err := m.Load(ctx); err != nil {
			return fmt.Errorf("加载 %s 资产失败: %w（先运行 go run ./cmd/extract -game %s）", id, err, id)
		}
		a.modules[id] = m
	}
	sort.Strings(a.unported)
	sort.Slice(a.switches, func(i, j int) bool { return a.switches[i].beat < a.switches[j].beat })

	// 分发实体
	for i := range a.bm.Entities {
		e := &a.bm.Entities[i]
		switch {
		case strings.HasPrefix(e.Datamodel, "gameManager/switchGame/"):
			// 已在上面收集
		case e.Datamodel == "gameManager/end":
			a.endBeat = e.Beat
		case e.Datamodel == "gameManager/skill star":
			a.starBeat = e.Beat + e.Length
		case e.Datamodel == "gameManager/toggle inputs":
			on := boolParam(e, "toggle")
			b := e.Beat
			a.at(b, func() { a.inputOn = on })
		case e.Datamodel == "vfx/flash":
			a.flashes = append(a.flashes, flashEvt{
				beat: e.Beat, length: e.Length,
				c0: colorParam(e, "colorA"), c1: colorParam(e, "colorB"),
			})
		case e.Game() == "gameManager" || e.Game() == "vfx" || e.Game() == "countIn" || e.Game() == "global":
			// 其余全局事件暂不支持
		default:
			if m, ok := a.modules[e.Game()]; ok {
				m.OnEvent(e)
			}
		}
	}
	for _, m := range a.modules {
		m.Ready()
	}
	if a.endBeat == 0 && len(a.bm.Entities) > 0 {
		last := a.bm.Entities[len(a.bm.Entities)-1]
		a.endBeat = last.Beat + last.Length + 4
	}
	sortActions(a.actions)
	sort.Slice(a.inputs, func(i, j int) bool { return a.inputs[i].Beat < a.inputs[j].Beat })

	// 初始活动游戏
	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	log.Printf("riq loaded: %q by %q, %d entities, games=%v unported=%v",
		a.bm.Prop("remixtitle"), a.bm.Prop("remixauthor"), len(a.bm.Entities), keys(a.modules), a.unported)
	return nil
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortActions(as []beatAction) {
	sort.SliceStable(as, func(i, j int) bool { return as[i].beat < as[j].beat })
}

func (a *App) setActive(id string, beat float64) {
	if m, ok := a.modules[id]; ok {
		a.active = m
		m.OnSwitch(beat)
	}
}

func (a *App) resetRunState() {
	a.swIdx = 0
	a.inputOn = true
	a.starGot = false
	a.aces, a.justs, a.ngs, a.misses, a.whiffs = 0, 0, 0, 0, 0
	a.lastMsg, a.msgT = "", 0
	a.tdArrow, a.tdTarget, a.tdHits = 0, 0, nil
}

// restart 重置一轮游玩（不重载资产）。
func (a *App) restart() error {
	if err := a.cond.Reset(); err != nil {
		return err
	}
	for _, in := range a.inputs {
		in.judged = false
		in.Result = JudgeNone
	}
	a.actIdx = 0
	a.resetRunState()
	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	a.state = stateTitle
	return nil
}

// ---------- 时间轴 / 判定服务（Ctx 转发到这里） ----------

func (a *App) at(beat float64, fn func()) {
	// 运行期插入需保序：找到插入点
	i := sort.Search(len(a.actions), func(i int) bool { return a.actions[i].beat > beat })
	a.actions = append(a.actions, beatAction{})
	copy(a.actions[i+1:], a.actions[i:])
	a.actions[i] = beatAction{beat, fn}
	if i < a.actIdx {
		a.actIdx++ // 插到已执行区前面（理论上不应发生），保持指针不回退
	}
}

func (a *App) scheduleInput(beat float64, onHit func(state float64, j Judgment), onMiss func()) {
	a.inputs = append(a.inputs, &Input{
		Beat: beat, hitT: a.bm.BeatToTime(beat),
		OnHit: onHit, OnMiss: onMiss,
	})
}

// ---------- Update ----------

func (a *App) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.debug = !a.debug
	}
	a.pollDroppedRiq()

	switch a.state {
	case stateTitle:
		if a.bm != nil && pressed() {
			a.cond.Play()
			a.state = statePlay
		}
	case statePlay:
		a.cond.Update()
		a.updatePlay()
	case stateResult:
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			return a.restart()
		}
	}
	return nil
}

func pressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

func (a *App) updatePlay() {
	t := a.cond.Time()
	beat := a.cond.Beat()

	// 游戏切换
	for a.swIdx < len(a.switches) && a.switches[a.swIdx].beat <= beat {
		a.setActive(a.switches[a.swIdx].id, a.switches[a.swIdx].beat)
		a.swIdx++
	}
	// 时间轴动作
	for a.actIdx < len(a.actions) && a.actions[a.actIdx].beat <= beat {
		a.actions[a.actIdx].fn()
		a.actIdx++
	}
	// 时机条箭头
	dt := 1.0 / float64(ebiten.TPS())
	a.tdArrow += (a.tdTarget - a.tdArrow) * math.Min(4*dt, 1)

	// 音量时间轴（riq__VolumeChange，含渐变）
	a.player.SetVolume(a.bm.VolumeAt(beat))

	// 延迟校准热键
	if inpututil.IsKeyJustPressed(ebiten.KeyLeftBracket) {
		a.LatencyMS -= 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRightBracket) {
		a.LatencyMS += 5
		a.setMsg(fmt.Sprintf("latency %+.0fms", a.LatencyMS))
	}

	if pressed() && a.inputOn {
		a.judgePress(t-a.LatencyMS/1000, beat)
	}

	// 超窗 miss
	for _, in := range a.inputs {
		if !in.judged && t > in.hitT+WinNG {
			in.judged = true
			in.Result = JudgeMiss
			a.misses++
			a.setMsg("MISS...")
			if in.OnMiss != nil {
				in.OnMiss()
			}
		}
	}

	if a.active != nil {
		a.active.Update(t, beat)
	}

	if beat > a.endBeat {
		a.cond.Pause()
		a.state = stateResult
	}
}

func (a *App) judgePress(t, beat float64) {
	var best *Input
	bestDiff := math.Inf(1)
	for _, in := range a.inputs {
		if in.judged {
			continue
		}
		if d := math.Abs(t - in.hitT); d < bestDiff {
			best, bestDiff = in, d
		}
	}
	if best == nil || bestDiff > WinNG {
		a.whiffs++
		a.setMsg("...")
		if a.active != nil {
			a.active.Whiff(beat)
		}
		return
	}

	best.judged = true
	signed := t - best.hitT
	state := signed / WinJust // |state|<=1 = just，与 C# stateProg 同语义
	var j Judgment
	switch d := math.Abs(signed); {
	case d <= WinAce:
		j = JudgeAce
		a.aces++
		a.setMsg("ACE!!")
	case d <= WinJust:
		j = JudgeJust
		a.justs++
		a.setMsg("OK!")
	default:
		j = JudgeNG
		a.ngs++
		a.setMsg("NG")
	}
	best.Result = j
	if j == JudgeAce && a.starBeat >= 0 && !a.starGot && math.Abs(best.Beat-a.starBeat) < 0.25 {
		a.starGot = true
		a.setMsg("SKILL STAR!")
	}
	a.pushTiming(signed, j)
	if best.OnHit != nil {
		best.OnHit(state, j)
	}
}

func (a *App) setMsg(s string) {
	a.lastMsg = s
	a.msgT = a.cond.Time()
}

func (a *App) pushTiming(signed float64, j Judgment) {
	y := math.Max(-1, math.Min(1, signed/WinNG))
	a.tdTarget = (a.tdTarget + y) * 0.5
	a.tdHits = append(a.tdHits, timingHit{y: y, rating: j, t: a.cond.Time()})
}

// ---------- riq 拖放导入 ----------

func (a *App) pollDroppedRiq() {
	df := ebiten.DroppedFiles()
	if df == nil {
		return
	}
	entries, err := fs.ReadDir(df, ".")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".riq") {
			continue
		}
		b, err := fs.ReadFile(df, name)
		if err != nil {
			a.loadErr = fmt.Sprintf("读取 %s 失败: %v", name, err)
			return
		}
		r, err := riq.LoadBytes(b)
		if err != nil {
			a.loadErr = fmt.Sprintf("%s 不是有效的 riq: %v", name, err)
			return
		}
		if err := a.loadRiq(r); err != nil {
			a.loadErr = fmt.Sprintf("加载 %s 失败: %v", filepath.Base(name), err)
			return
		}
		a.loadErr = ""
		a.state = stateTitle
		return
	}
}

// ---------- Draw ----------

func (a *App) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{16, 16, 20, 255})
	t, beat := 0.0, 0.0
	if a.cond != nil {
		t, beat = a.cond.Time(), a.cond.Beat()
	}

	if a.active != nil {
		a.active.Draw(screen, t, beat)
	}

	a.drawFlash(screen, beat)

	white := color.RGBA{245, 245, 250, 255}
	dim := color.RGBA{200, 200, 210, 200}
	switch a.state {
	case stateTitle:
		a.drawTitle(screen, white, dim)
	case statePlay:
		if a.lastMsg != "" && t-a.msgT < 0.6 {
			a.text(screen, a.lastMsg, a.faceBig, ScreenW/2, 90, white, true)
		}
		if a.starGot {
			a.text(screen, "* SKILL STAR", a.faceSmall, ScreenW-130, 20, color.RGBA{255, 230, 90, 255}, false)
		}
		if sec := a.bm.SectionAt(beat); sec != "" {
			a.text(screen, "- "+sec+" -", a.faceSmall, ScreenW-130, 40, color.RGBA{210, 210, 225, 200}, false)
		}
		if a.endBeat > 0 {
			prog := math.Min(beat/a.endBeat, 1)
			vector.DrawFilledRect(screen, 0, 0, float32(ScreenW*prog), 4, white, false)
		}
		a.drawTimingBar(screen, t)
	case stateResult:
		a.drawResult(screen, white)
	}

	if a.debug {
		a.drawDebug(screen, t, beat)
	}
}

func (a *App) drawTitle(screen *ebiten.Image, white, dim color.RGBA) {
	if a.bm == nil {
		a.text(screen, "HEAVEN GO", a.faceBig, ScreenW/2, 180, white, true)
		a.text(screen, "拖入 .riq 文件开始游玩 / drop a .riq file here", a.faceMid, ScreenW/2, 280, dim, true)
	} else {
		title := a.bm.Prop("remixtitle")
		if title == "" {
			title = "Untitled Remix"
		}
		a.text(screen, title, a.faceBig, ScreenW/2, 110, white, true)
		if author := a.bm.Prop("remixauthor"); author != "" {
			a.text(screen, "chart by "+author, a.faceMid, ScreenW/2, 170, dim, true)
		}
		a.text(screen, fmt.Sprintf("%d inputs | %.0f BPM | games: %s",
			len(a.inputs), a.bm.Tempos[0].BPM, strings.Join(keys(a.modules), ", ")),
			a.faceSmall, ScreenW/2, 208, dim, true)
		if len(a.unported) > 0 {
			a.text(screen, "尚未移植: "+strings.Join(a.unported, ", "), a.faceSmall, ScreenW/2, 232,
				color.RGBA{255, 170, 120, 255}, true)
		}
		a.text(screen, "Space / J / Click — play    (可拖入其他 .riq 切换)", a.faceMid, ScreenW/2, ScreenH-110, white, true)
		a.text(screen, "press to start", a.faceMid, ScreenW/2, ScreenH-72, white, true)
	}
	if a.loadErr != "" {
		a.text(screen, a.loadErr, a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawResult(screen *ebiten.Image, white color.RGBA) {
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{0, 0, 0, 150}, false)
	total := len(a.inputs)
	bad := a.ngs + a.misses + a.whiffs
	rank, c := "TRY AGAIN", color.RGBA{255, 120, 120, 255}
	switch {
	case bad == 0:
		rank, c = "SUPERB!!", color.RGBA{140, 255, 170, 255}
	case total > 0 && bad <= total/6:
		rank, c = "OK", color.RGBA{140, 200, 255, 255}
	}
	a.text(screen, rank, a.faceBig, ScreenW/2, 150, c, true)
	a.text(screen, fmt.Sprintf("ACE %d   OK %d   NG %d   MISS %d   whiff %d",
		a.aces, a.justs, a.ngs, a.misses, a.whiffs), a.faceMid, ScreenW/2, 230, white, true)
	if a.starGot {
		a.text(screen, "* skill star acquired", a.faceMid, ScreenW/2, 270, color.RGBA{255, 230, 90, 255}, true)
	}
	a.text(screen, "R — restart    Esc — quit    (可拖入其他 .riq)", a.faceMid, ScreenW/2, 340, white, true)
}

func (a *App) drawFlash(screen *ebiten.Image, beat float64) {
	for _, f := range a.flashes {
		if beat < f.beat || f.length <= 0 {
			continue
		}
		u := math.Min((beat-f.beat)/f.length, 1)
		c := [4]float64{}
		for i := range c {
			c[i] = f.c0[i] + (f.c1[i]-f.c0[i])*u
		}
		if c[3] > 0 {
			vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{
				uint8(c[0] * 255 * c[3]), uint8(c[1] * 255 * c[3]), uint8(c[2] * 255 * c[3]), uint8(c[3] * 255),
			}, false)
		}
	}
}

func (a *App) drawPlaceholder(screen *ebiten.Image, id string) {
	screen.Fill(color.RGBA{40, 40, 52, 255})
	a.text(screen, id, a.faceBig, ScreenW/2, ScreenH/2-40, color.RGBA{210, 210, 225, 255}, true)
	a.text(screen, "这个 minigame 还没有移植，乐曲继续……", a.faceMid, ScreenW/2, ScreenH/2+20, color.RGBA{160, 160, 175, 255}, true)
}

func (a *App) drawDebug(screen *ebiten.Image, t, beat float64) {
	white := color.RGBA{235, 235, 245, 255}
	lines := []string{
		fmt.Sprintf("songPos %8.3fs  beat %7.3f", t, beat),
		fmt.Sprintf("tps %.0f fps %.0f", ebiten.ActualTPS(), ebiten.ActualFPS()),
	}
	if a.cond != nil {
		lines = append(lines, fmt.Sprintf("drift %+6.1fms", a.cond.Drift()*1000))
	}
	n := 0
	for _, in := range a.inputs {
		if !in.judged {
			n++
		}
	}
	lines = append(lines, fmt.Sprintf("actions %d/%d  inputs left %d", a.actIdx, len(a.actions), n))
	for i, s := range lines {
		a.text(screen, s, a.faceSmall, 20, 40+float64(i)*18, white, false)
	}
}

func (a *App) text(screen *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color, center bool) {
	if center {
		w, _ := text.Measure(s, face, 0)
		x -= w / 2
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

func (a *App) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }

// Stats 是装载结果摘要（测试/诊断用）。
type Stats struct {
	Inputs   int
	Actions  int
	EndBeat  float64
	StarBeat float64
	Unported []string
}

// LoadStats 返回当前谱面的装载摘要。
func (a *App) LoadStats() Stats {
	return Stats{
		Inputs: len(a.inputs), Actions: len(a.actions),
		EndBeat: a.endBeat, StarBeat: a.starBeat, Unported: a.unported,
	}
}

// ---------- 参数辅助 ----------

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func colorParam(e *riq.Entity, key string) [4]float64 {
	out := [4]float64{}
	if m, ok := e.Data[key].(map[string]any); ok {
		get := func(k string) float64 {
			if f, ok := m[k].(float64); ok {
				return f
			}
			return 0
		}
		out = [4]float64{get("r"), get("g"), get("b"), get("a")}
	}
	return out
}
