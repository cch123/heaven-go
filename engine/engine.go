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

	menuVisibleRows = 7
	menuRowX        = 64
	menuRowY        = 104
	menuRowW        = 470
	menuRowH        = 45
	menuRowGap      = 8
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
	// Release 为 true 时在按键"抬起"时判定（HS InputAction_FlickRelease，
	// totemClimb 高跳的甩出）；否则按下判定。
	Release bool
	// Action 是输入动作通道：0=主键（Space/J/左键） 1=左（F/←/↑）
	// 2=右（K/→） 3=替代键（L/↓/X，HS 的 South/Alt）。
	Action int
	// OnHit 在 NG 窗口内的任意按键触发；state 为 just 窗归一化偏移
	//（|state|<=1 = just 命中，1<|state|<=2 = NG，负 = 早），与 C# 语义一致。
	OnHit func(state float64, j Judgment)
	// OnMiss 在超窗未按时触发。
	OnMiss func()
}

// camEvt 是 vfx/move camera 事件（GameCamera.UpdateCameraTranslate 语义）。
type camEvt struct {
	beat, length float64
	target       [3]float64 // (valA, valB, -valC)
	ease         int
	axis         int // 0=All 1=X 2=Y 3=Z
}

type flashEvt struct {
	beat, length float64
	c0, c1       [4]float64
}

type gameSwitch struct {
	beat float64
	id   string
}

type menuLevel struct {
	path string
	name string
}

// viewScaleEvt 是 vfx/scale view 事件（StaticCamera：整张游戏画布的缩放，
// 画布外露出 letterbox 黑场）。
type viewScaleEvt struct {
	beat, length float64
	x, y         float64
	ease         int
	axis         int // 0=All 1=X 2=Y
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
	camEvts  []camEvt
	unported []string

	// commonSounds：assets/common/sounds 的公共音效（countIn 计数音、
	// miss/nearMiss 等），缺目录时为空 map（相关事件静默跳过）
	commonSounds map[string][]byte

	fx  postFX    // ppe/* 屏幕后处理
	flt filterFX  // vfx/filter（LUT 滤镜）
	tbx textboxFX // vfx/display textbox

	viewScales []viewScaleEvt // vfx/scale view（画布缩放）
	viewBuf    *ebiten.Image  // 缩放生效时的离屏画布

	pressedNow  bool // 本逻辑帧是否有按下（模块轮询用，如 totemClimb HoldCo）
	releasedNow bool // 本逻辑帧是否有抬起

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

	// Autoplay：调试用完美自动打击（HS 的 autoplay 等价物），-autoplay 开启
	Autoplay bool

	levels     []menuLevel
	menuSel    int
	menuScroll int

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
		assetsRoot:   assetsRoot,
		faceBig:      &text.GoTextFace{Source: src, Size: 44},
		faceMid:      &text.GoTextFace{Source: src, Size: 24},
		faceSmall:    &text.GoTextFace{Source: src, Size: 15},
		commonSounds: map[string][]byte{},
		levels:       discoverLevels("levels"),
	}
	a.loadCommonSounds()
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

func discoverLevels(dir string) []menuLevel {
	paths, err := filepath.Glob(filepath.Join(dir, "*.riq"))
	if err != nil {
		return nil
	}
	sort.Strings(paths)
	out := make([]menuLevel, 0, len(paths))
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		out = append(out, menuLevel{path: p, name: name})
	}
	return out
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
		return fmt.Errorf("unsupported audio format (%s)", r.AudioName)
	}
	if err != nil {
		return fmt.Errorf("decode music: %w", err)
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
	a.camEvts = nil
	a.viewScales = nil
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
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
		case "gameManager", "vfx", "countIn", "global", "ppe":
			continue // ppe（屏幕后处理）由引擎处理，不占模块位
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
			return fmt.Errorf("load %s assets: %w (run go run ./cmd/extract -game %s first)", id, err, id)
		}
		a.modules[id] = m
	}
	sort.Strings(a.unported)
	sort.Slice(a.switches, func(i, j int) bool { return a.switches[i].beat < a.switches[j].beat })
	sort.Slice(a.flashes, func(i, j int) bool { return a.flashes[i].beat < a.flashes[j].beat })

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
		case e.Datamodel == "vfx/scale view":
			a.viewScales = append(a.viewScales, viewScaleEvt{
				beat: e.Beat, length: e.Length,
				x: e.Float("valA", 1), y: e.Float("valB", 1),
				ease: int(e.Float("ease", 0)),
				axis: int(e.Float("axis", 0)),
			})
		case e.Datamodel == "vfx/move camera" || e.Datamodel == "gameManager/move camera":
			a.camEvts = append(a.camEvts, camEvt{
				beat: e.Beat, length: e.Length,
				target: [3]float64{e.Float("valA", 0), e.Float("valB", 0), -e.Float("valC", 10)},
				ease:   int(e.Float("ease", 0)),
				axis:   int(e.Float("axis", 0)),
			})
		case e.Game() == "countIn":
			a.scheduleCountIn(e.Datamodel, e.Beat, e.Length, e.Data)
		case e.Datamodel == "vfx/filter":
			a.flt.add(e)
		case e.Datamodel == "vfx/display textbox":
			a.tbx.add(e)
		case e.Game() == "ppe":
			a.fx.add(e)
		case e.Game() == "gameManager" || e.Game() == "vfx" || e.Game() == "global":
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
	a.fx.sortAll()
	if a.fx.active() {
		if err := a.fx.ensure(); err != nil {
			return fmt.Errorf("compile ppe shader: %w", err)
		}
	}

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

func (a *App) scheduleInput(beat float64, release bool, action int, onHit func(state float64, j Judgment), onMiss func()) {
	a.inputs = append(a.inputs, &Input{
		Beat: beat, hitT: a.bm.BeatToTime(beat), Release: release, Action: action,
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
		if a.bm == nil {
			a.updateLevelSelect()
			return nil
		}
		if a.bm != nil && (titlePressed() || a.Autoplay) {
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

func (a *App) updateLevelSelect() {
	if len(a.levels) == 0 {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyW) {
		a.moveMenu(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) {
		a.moveMenu(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		a.moveMenu(-menuVisibleRows)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		a.moveMenu(menuVisibleRows)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		a.moveMenu(-1)
	} else if wheelY < 0 {
		a.moveMenu(1)
	}
	if idx, ok := a.hoveredMenuLevel(); ok {
		a.menuSel = idx
		a.keepMenuSelectionVisible()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.loadSelectedLevel()
			return
		}
	}
	if menuConfirmPressed() {
		a.loadSelectedLevel()
	}
}

func menuConfirmPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func titlePressed() bool {
	return pressed() ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func (a *App) moveMenu(delta int) {
	a.menuSel += delta
	if a.menuSel < 0 {
		a.menuSel = 0
	}
	if a.menuSel >= len(a.levels) {
		a.menuSel = len(a.levels) - 1
	}
	a.keepMenuSelectionVisible()
}

func (a *App) keepMenuSelectionVisible() {
	if a.menuSel < a.menuScroll {
		a.menuScroll = a.menuSel
	}
	if a.menuSel >= a.menuScroll+menuVisibleRows {
		a.menuScroll = a.menuSel - menuVisibleRows + 1
	}
	maxScroll := len(a.levels) - menuVisibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.menuScroll > maxScroll {
		a.menuScroll = maxScroll
	}
	if a.menuScroll < 0 {
		a.menuScroll = 0
	}
}

func (a *App) hoveredMenuLevel() (int, bool) {
	x, y := ebiten.CursorPosition()
	if x < menuRowX || x >= menuRowX+menuRowW {
		return 0, false
	}
	rowStep := menuRowH + menuRowGap
	if y < menuRowY || y >= menuRowY+menuVisibleRows*rowStep {
		return 0, false
	}
	row := (y - menuRowY) / rowStep
	if y >= menuRowY+row*rowStep+menuRowH {
		return 0, false
	}
	idx := a.menuScroll + row
	if idx < 0 || idx >= len(a.levels) {
		return 0, false
	}
	return idx, true
}

func (a *App) loadSelectedLevel() {
	if a.menuSel < 0 || a.menuSel >= len(a.levels) {
		return
	}
	level := a.levels[a.menuSel]
	r, err := riq.Load(level.path)
	if err != nil {
		a.loadErr = fmt.Sprintf("read %s failed: %v", level.name, err)
		return
	}
	if err := a.loadRiq(r); err != nil {
		a.loadErr = fmt.Sprintf("load %s failed: %v", level.name, err)
		return
	}
	a.loadErr = ""
	a.state = stateTitle
}

func pressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// pressedN：动作通道 1=左（F/←/↑）、2=右（K/→）、3=替代（L/↓/X）。
func pressedN(action int) bool {
	switch action {
	case 1:
		return inpututil.IsKeyJustPressed(ebiten.KeyF) ||
			inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
			inpututil.IsKeyJustPressed(ebiten.KeyUp)
	case 2:
		return inpututil.IsKeyJustPressed(ebiten.KeyK) ||
			inpututil.IsKeyJustPressed(ebiten.KeyRight)
	case 3:
		return inpututil.IsKeyJustPressed(ebiten.KeyL) ||
			inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
			inpututil.IsKeyJustPressed(ebiten.KeyX)
	}
	return false
}

func released() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeySpace) ||
		inpututil.IsKeyJustReleased(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
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

	a.pressedNow, a.releasedNow = pressed(), released()
	if a.pressedNow && a.inputOn {
		a.judgePress(t-a.LatencyMS/1000, beat, false, 0)
	}
	for act := 1; act <= 3; act++ {
		if pressedN(act) && a.inputOn {
			a.judgePress(t-a.LatencyMS/1000, beat, false, act)
		}
	}
	if a.releasedNow && a.inputOn {
		a.judgePress(t-a.LatencyMS/1000, beat, true, 0)
	}
	if a.Autoplay {
		for _, in := range a.inputs {
			if !in.judged && t >= in.hitT {
				a.judgePress(in.hitT, beat, in.Release, in.Action)
			}
		}
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

// judgePress 判定一次按下（release=false）或抬起（release=true）。
// 抬起只匹配 Release 输入，且空抬不计 whiff（HS 的 flick release whiff
// 由游戏侧自行处理，如 totemClimb HoldCo）。
func (a *App) judgePress(t, beat float64, release bool, action int) {
	var best *Input
	bestDiff := math.Inf(1)
	for _, in := range a.inputs {
		if in.judged || in.Release != release || in.Action != action {
			continue
		}
		if d := math.Abs(t - in.hitT); d < bestDiff {
			best, bestDiff = in, d
		}
	}
	if best == nil || bestDiff > WinNG {
		if release {
			return
		}
		a.whiffs++
		a.setMsg("...")
		if a.active != nil {
			if aw, ok := a.active.(ActionWhiffer); ok {
				aw.WhiffAction(beat, action)
			} else if action == 0 {
				a.active.Whiff(beat)
			}
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
			a.loadErr = fmt.Sprintf("read %s failed: %v", name, err)
			return
		}
		r, err := riq.LoadBytes(b)
		if err != nil {
			a.loadErr = fmt.Sprintf("%s is not a valid riq: %v", name, err)
			return
		}
		if err := a.loadRiq(r); err != nil {
			a.loadErr = fmt.Sprintf("load %s failed: %v", filepath.Base(name), err)
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

	// vfx/scale view：缩放生效时游戏画布整体渲到离屏帧再贴回
	//（StaticCamera 语义：画布外露出 letterbox 黑场；HUD 不参与缩放）。
	vsx, vsy := a.viewScaleAt(beat)
	canvas := screen
	if vsx != 1 || vsy != 1 {
		if a.viewBuf == nil {
			a.viewBuf = ebiten.NewImage(ScreenW, ScreenH)
		}
		a.viewBuf.Fill(color.RGBA{16, 16, 20, 255})
		canvas = a.viewBuf
	}

	if a.active != nil {
		if a.fx.active() {
			// ppe：游戏画面渲到离屏帧，经后处理链上屏（flash/HUD 不参与，
			// 对应 HS 的编辑器叠层不过 PostProcessLayer）
			a.active.Draw(a.fx.Target(), t, beat)
			a.fx.Apply(canvas, beat, t)
		} else {
			a.active.Draw(canvas, t, beat)
		}
		a.flt.Apply(canvas, a.assetsRoot, beat)
		a.tbx.Draw(canvas, a.assetsRoot, beat)
	}

	a.drawFlash(canvas, beat)

	if canvas != screen {
		screen.Fill(color.RGBA{0, 0, 0, 255}) // letterbox 黑场
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(-ScreenW/2, -ScreenH/2)
		op.GeoM.Scale(vsx, vsy)
		op.GeoM.Translate(ScreenW/2, ScreenH/2)
		screen.DrawImage(a.viewBuf, op)
	}

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
		a.drawLevelSelect(screen, white, dim)
		return
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
			a.text(screen, "Unported: "+strings.Join(a.unported, ", "), a.faceSmall, ScreenW/2, 232,
				color.RGBA{255, 170, 120, 255}, true)
		}
		a.text(screen, "Space / J / Click to play    (drop another .riq to switch)", a.faceMid, ScreenW/2, ScreenH-110, white, true)
		a.text(screen, "press to start", a.faceMid, ScreenW/2, ScreenH-72, white, true)
	}
	if a.loadErr != "" {
		a.text(screen, a.loadErr, a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLevelSelect(screen *ebiten.Image, white, dim color.RGBA) {
	screen.Fill(color.RGBA{22, 24, 28, 255})
	vector.DrawFilledRect(screen, 0, 0, ScreenW, 86, color.RGBA{33, 41, 54, 255}, false)
	vector.DrawFilledRect(screen, 0, 84, ScreenW, 3, color.RGBA{232, 184, 74, 255}, false)
	vector.DrawFilledRect(screen, 0, ScreenH-58, ScreenW, 58, color.RGBA{18, 19, 23, 255}, false)

	a.text(screen, "HEAVEN GO", a.faceBig, 64, 22, white, false)
	a.text(screen, "LEVEL SELECT", a.faceMid, 720, 30, color.RGBA{232, 184, 74, 255}, true)

	if len(a.levels) == 0 {
		a.text(screen, "No .riq levels found under levels/", a.faceMid, ScreenW/2, 230, white, true)
		a.text(screen, "Drop a .riq file here to play", a.faceMid, ScreenW/2, 282, dim, true)
		if a.loadErr != "" {
			a.text(screen, a.fitText(a.loadErr, a.faceSmall, 760), a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
		}
		return
	}

	a.keepMenuSelectionVisible()
	for row := 0; row < menuVisibleRows; row++ {
		idx := a.menuScroll + row
		if idx >= len(a.levels) {
			break
		}
		y := menuRowY + row*(menuRowH+menuRowGap)
		fy := float32(y)
		ty := float64(y)
		selected := idx == a.menuSel
		bg := color.RGBA{38, 41, 49, 255}
		textCol := color.RGBA{216, 219, 228, 255}
		if selected {
			bg = color.RGBA{58, 66, 78, 255}
			textCol = white
		}
		vector.DrawFilledRect(screen, menuRowX, fy, menuRowW, menuRowH, bg, false)
		vector.DrawFilledRect(screen, menuRowX, fy, 7, menuRowH, menuAccent(idx), false)
		a.text(screen, fmt.Sprintf("%02d", idx+1), a.faceSmall, menuRowX+20, ty+16, color.RGBA{170, 176, 188, 255}, false)
		a.text(screen, a.fitText(a.levels[idx].name, a.faceMid, 340), a.faceMid, menuRowX+64, ty+10, textCol, false)
		if selected {
			vector.StrokeLine(screen, menuRowX, fy+menuRowH-1, menuRowX+menuRowW, fy+menuRowH-1, 2, color.RGBA{232, 184, 74, 255}, false)
		}
	}

	a.drawLevelDetails(screen, a.levels[a.menuSel], a.menuSel, white, dim)
	a.text(screen, "Enter / Space / Click to load    Arrow keys to choose    Drop .riq to import", a.faceSmall, ScreenW/2, ScreenH-38, dim, true)
	if a.loadErr != "" {
		a.text(screen, a.fitText(a.loadErr, a.faceSmall, 840), a.faceSmall, ScreenW/2, ScreenH-18, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLevelDetails(screen *ebiten.Image, level menuLevel, idx int, white, dim color.RGBA) {
	x, y := 575.0, 106.0
	w, h := 320.0, 328.0
	accent := menuAccent(idx)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{34, 37, 45, 255}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), 7, accent, false)
	a.text(screen, "SELECTED", a.faceSmall, x+24, y+28, color.RGBA{170, 176, 188, 255}, false)
	a.text(screen, a.fitText(level.name, a.faceMid, w-48), a.faceMid, x+24, y+62, white, false)
	a.text(screen, a.fitText(level.path, a.faceSmall, w-48), a.faceSmall, x+24, y+103, dim, false)

	baseY := y + 162
	for i := 0; i < 18; i++ {
		barH := 18 + 54*math.Abs(math.Sin(float64(i+idx)*0.72))
		c := accent
		switch i % 4 {
		case 1:
			c = color.RGBA{83, 189, 179, 255}
		case 2:
			c = color.RGBA{235, 111, 94, 255}
		case 3:
			c = color.RGBA{147, 194, 86, 255}
		}
		vector.DrawFilledRect(screen, float32(x+24+float64(i)*15), float32(baseY+80-barH), 8, float32(barH), c, false)
	}
	vector.DrawFilledRect(screen, float32(x+24), float32(baseY+90), float32(w-48), 2, color.RGBA{95, 101, 112, 255}, false)
	a.text(screen, "Official Pack-In level", a.faceSmall, x+24, y+h-72, dim, false)
	a.text(screen, "Load opens the chart title screen", a.faceSmall, x+24, y+h-46, color.RGBA{218, 222, 232, 255}, false)
}

func menuAccent(i int) color.RGBA {
	palette := []color.RGBA{
		{232, 184, 74, 255},
		{83, 189, 179, 255},
		{235, 111, 94, 255},
		{147, 194, 86, 255},
		{157, 139, 214, 255},
		{76, 151, 218, 255},
	}
	return palette[i%len(palette)]
}

func (a *App) fitText(s string, face *text.GoTextFace, maxW float64) string {
	if w, _ := text.Measure(s, face, 0); w <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		candidate := string(rs) + "..."
		if w, _ := text.Measure(candidate, face, 0); w <= maxW {
			return candidate
		}
	}
	return "..."
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
	a.text(screen, "R - restart    Esc - quit    (drop .riq to switch)", a.faceMid, ScreenW/2, 340, white, true)
}

// viewScaleAt 折叠 vfx/scale view 事件得到画布缩放（StaticCamera.UpdateScale：
// 进行中的事件从上一事件终值缓动到自身目标）。
func (a *App) viewScaleAt(beat float64) (float64, float64) {
	sx, sy := 1.0, 1.0
	lx, ly := 1.0, 1.0
	for _, e := range a.viewScales {
		if beat < e.beat {
			continue
		}
		prog := 1.0
		if e.length > 0 {
			prog = math.Min((beat-e.beat)/e.length, 1)
		}
		switch e.axis {
		case 1:
			sx = Ease(e.ease, lx, e.x, prog)
		case 2:
			sy = Ease(e.ease, ly, e.y, prog)
		default:
			sx = Ease(e.ease, lx, e.x, prog)
			sy = Ease(e.ease, ly, e.y, prog)
		}
		if prog >= 1 {
			switch e.axis {
			case 1:
				lx = e.x
			case 2:
				ly = e.y
			default:
				lx, ly = e.x, e.y
			}
		}
	}
	return sx, sy
}

// drawFlash：vfx/flash 是单一覆盖层（HS Fade 语义）——按拍序折叠，
// 最后一个已开始的事件决定当前颜色（事件结束后停在其终色），
// 不能把多个事件叠画（先前事件的不透明终色会永久压住画面）。
func (a *App) drawFlash(screen *ebiten.Image, beat float64) {
	var c [4]float64
	hit := false
	for _, f := range a.flashes {
		if beat < f.beat || f.length <= 0 {
			continue
		}
		u := math.Min((beat-f.beat)/f.length, 1)
		for i := range c {
			c[i] = f.c0[i] + (f.c1[i]-f.c0[i])*u
		}
		hit = true
	}
	if hit && c[3] > 0 {
		vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{
			uint8(c[0] * 255 * c[3]), uint8(c[1] * 255 * c[3]), uint8(c[2] * 255 * c[3]), uint8(c[3] * 255),
		}, false)
	}
}

func (a *App) drawPlaceholder(screen *ebiten.Image, id string) {
	screen.Fill(color.RGBA{40, 40, 52, 255})
	a.text(screen, id, a.faceBig, ScreenW/2, ScreenH/2-40, color.RGBA{210, 210, 225, 255}, true)
	a.text(screen, "This minigame is not ported yet; the song continues.", a.faceMid, ScreenW/2, ScreenH/2+20, color.RGBA{160, 160, 175, 255}, true)
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

// BeatNow 返回当前歌曲节拍（录制/验证工具用）。
func (a *App) BeatNow() float64 {
	if a.cond == nil {
		return 0
	}
	return a.cond.Beat()
}

// RunCounts 返回当前判定计数 ace/just/ng/miss/whiff（验证工具用）。
func (a *App) RunCounts() (int, int, int, int, int) {
	return a.aces, a.justs, a.ngs, a.misses, a.whiffs
}

// Finished 报告是否已进入结算画面。
func (a *App) Finished() bool { return a.state == stateResult }

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
