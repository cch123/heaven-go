// engine.go：App 外壳——谱面加载/重载（含拖放导入）、音频、conductor、
// 时间轴执行、输入判定、游戏切换、HUD/时机条/flash、结算。
package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "image/jpeg"
	_ "image/png"

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

	rankOkThreshold = 0.6
	rankHiThreshold = 0.8

	menuGridX        = 54
	menuGridY        = 116
	menuCardW        = 148
	menuCardH        = 172
	menuCardGapX     = 20
	menuCardGapY     = 24
	menuGridCols     = 3
	menuGridRows     = 2
	menuVisibleItems = menuGridCols * menuGridRows

	resultMsgTime  = 0.55
	resultMsg2Time = 1.25
	resultBarStart = 1.8
	resultBarDur   = 1.25
	resultRankTime = 3.45
	// JudgementManager.WaitAndRank waits 1.5s after the rank sound before
	// starting the rank jingle/loop music.
	resultRankMusicWait = 1.5
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
	// Weight/Category 来自当前 SectionMarker；Judgement 场景用它们做加权总评
	// 和分类评价消息。
	Weight   float64
	Category int
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
	path       string
	fileName   string
	title      string
	author     string
	desc       string
	games      []string
	bpm        float64
	customIcon *ebiten.Image
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

type resultRank int

const (
	resultRankNg resultRank = iota
	resultRankOk
	resultRankHi
)

type resultScoreInput struct {
	Beat     float64
	Accuracy float64
	Weight   float64
	Category int
}

type resultSummary struct {
	Score      float64
	Rank       resultRank
	Header     string
	Message0   string
	Message1   string
	Message2   string
	TwoMessage bool
	SubRank    bool
	NoMiss     bool
	Perfect    bool
	Star       bool
}

type resultAssets struct {
	bg          *ebiten.Image
	rankHi      *ebiten.Image
	rankHiStar  *ebiten.Image
	rankOk      *ebiten.Image
	rankOkSweat *ebiten.Image
	rankNg      []*ebiten.Image
	epHi        *ebiten.Image
	epOk        *ebiten.Image
	epNg        *ebiten.Image
}

type libraryAssets struct {
	bgBase      *ebiten.Image
	bgGradient  *ebiten.Image
	bgStars     *ebiten.Image
	bgWaves     *ebiten.Image
	borderSheet *ebiten.Image
	border      *ebiten.Image
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
	scores   []resultScoreInput
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

	result           resultSummary
	resultAssets     resultAssets
	resultAudio      resultAudioAssets
	resultAudioState resultAudioState
	libraryAssets    libraryAssets
	resultT          float64
	resultEpilogue   bool

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
	a.resultAssets = loadResultAssets(filepath.Join(assetsRoot, "common", "ratings"))
	a.resultAudio = loadResultAudio(filepath.Join(assetsRoot, "common", "result_sounds"))
	a.libraryAssets = loadLibraryAssets(filepath.Join(assetsRoot, "common", "library"))
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

func loadResultAssets(dir string) resultAssets {
	load := func(name string) *ebiten.Image {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			log.Printf("engine: decode result asset %s: %v", name, err)
			return nil
		}
		return ebiten.NewImageFromImage(img)
	}
	return resultAssets{
		bg:          load("judgementBg.png"),
		rankHi:      load(filepath.Join("Superb", "superbrating.png")),
		rankHiStar:  load(filepath.Join("Superb", "superbratingstar.png")),
		rankOk:      load(filepath.Join("OK", "okrating.png")),
		rankOkSweat: load(filepath.Join("OK", "okratingsweat.png")),
		rankNg: []*ebiten.Image{
			load(filepath.Join("TryAgain", "tryagainrating0001.png")),
			load(filepath.Join("TryAgain", "tryagainrating0002.png")),
			load(filepath.Join("TryAgain", "tryagainrating0003.png")),
		},
		epHi: load(filepath.Join("Epilogue", "superb.png")),
		epOk: load(filepath.Join("Epilogue", "ok.png")),
		epNg: load(filepath.Join("Epilogue", "tryagain.png")),
	}
}

func loadLibraryAssets(dir string) libraryAssets {
	load := func(name string) *ebiten.Image {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			log.Printf("engine: decode library asset %s: %v", name, err)
			return nil
		}
		return ebiten.NewImageFromImage(img)
	}
	assets := libraryAssets{
		bgBase:      load(filepath.Join("bg", "libBgBase.png")),
		bgGradient:  load(filepath.Join("bg", "libBgGradient.png")),
		bgStars:     load(filepath.Join("bg", "libBgStars.png")),
		bgWaves:     load(filepath.Join("bg", "libBgWaves.png")),
		borderSheet: load("levelBorders.png"),
	}
	// Unity's sprite atlas rects use a bottom-left origin. This is the original
	// unplayed level border slice from levelBorders.png.
	if assets.borderSheet != nil {
		sheetH := assets.borderSheet.Bounds().Dy()
		rect := image.Rect(40, sheetH-740-576, 40+576, sheetH-740)
		if sub, ok := assets.borderSheet.SubImage(rect).(*ebiten.Image); ok {
			assets.border = sub
		}
	}
	return assets
}

func discoverLevels(dir string) []menuLevel {
	paths, err := filepath.Glob(filepath.Join(dir, "*.riq"))
	if err != nil {
		return nil
	}
	sort.Strings(paths)
	out := make([]menuLevel, 0, len(paths))
	for _, p := range paths {
		out = append(out, inspectMenuLevel(p))
	}
	return out
}

func inspectMenuLevel(p string) menuLevel {
	fileName := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	level := menuLevel{
		path:     p,
		fileName: fileName,
		title:    fileName,
		bpm:      120,
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		return level
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			files[f.Name] = f
		}
	}
	if raw, ok := readZipFile(files, "remix.json"); ok {
		applyV1MenuMetadata(&level, raw)
	} else if chartName, ok := findZipChart(files); ok {
		if raw, ok := readZipFile(files, chartName); ok {
			applyV2MenuMetadata(&level, raw)
		}
	}
	level.customIcon = readLibraryLevelIcon(files)
	return level
}

type menuV1Chart struct {
	Properties   map[string]any `json:"properties"`
	Entities     []menuV1Entity `json:"entities"`
	TempoChanges []menuV1Entity `json:"tempoChanges"`
}

type menuV1Entity struct {
	Datamodel   string         `json:"datamodel"`
	DynamicData map[string]any `json:"dynamicData"`
}

type menuV2Chart struct {
	SongName string            `json:"songname"`
	Entities []menuV2Entity    `json:"entities"`
	Models   map[string]string `json:"models"`
	Types    map[string]string `json:"types"`
}

type menuV2Entity struct {
	Type  int            `json:"type"`
	Model int            `json:"model"`
	Data  map[string]any `json:"data"`
}

func applyV1MenuMetadata(level *menuLevel, raw []byte) {
	var c menuV1Chart
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &c); err != nil {
		log.Printf("engine: parse level metadata %s: %v", level.path, err)
		return
	}
	if title := menuString(c.Properties, "remixtitle"); title != "" {
		level.title = title
	}
	level.author = menuString(c.Properties, "remixauthor")
	level.desc = menuString(c.Properties, "remixdesc")
	if bpm := menuFloat(c.Properties, "remixtempo"); bpm > 0 {
		level.bpm = bpm
	}
	for _, e := range c.TempoChanges {
		if bpm := menuFloat(e.DynamicData, "tempo"); bpm > 0 {
			level.bpm = bpm
			break
		}
	}
	level.games = summarizeMenuGames(v1MenuModels(c.Entities))
}

func v1MenuModels(entities []menuV1Entity) []string {
	models := make([]string, 0, len(entities))
	for _, e := range entities {
		models = append(models, e.Datamodel)
	}
	return models
}

func applyV2MenuMetadata(level *menuLevel, raw []byte) {
	var c menuV2Chart
	if err := json.Unmarshal(raw, &c); err != nil {
		log.Printf("engine: parse level metadata %s: %v", level.path, err)
		return
	}
	if c.SongName != "" {
		level.title = c.SongName
	}
	models := make([]string, 0, len(c.Entities))
	for _, e := range c.Entities {
		model := c.Models[fmt.Sprint(e.Model)]
		typ := c.Types[fmt.Sprint(e.Type)]
		if typ == "riq__TempoChange" {
			if bpm := menuFloat(e.Data, "tempo"); bpm > 0 {
				level.bpm = bpm
			}
			continue
		}
		models = append(models, model)
	}
	level.games = summarizeMenuGames(models)
}

func summarizeMenuGames(models []string) []string {
	switches := make([]string, 0, 4)
	fallback := make([]string, 0, 4)
	seenSwitches := map[string]bool{}
	seenFallback := map[string]bool{}
	for _, model := range models {
		if game, ok := strings.CutPrefix(model, "gameManager/switchGame/"); ok {
			addUniqueGame(&switches, seenSwitches, game)
			continue
		}
		game, _, ok := strings.Cut(model, "/")
		if !ok || ignoredMenuGame(game) {
			continue
		}
		addUniqueGame(&fallback, seenFallback, game)
	}
	if len(switches) > 0 {
		return switches
	}
	return fallback
}

func ignoredMenuGame(game string) bool {
	switch game {
	case "", "gameManager", "global", "vfx", "ppe", "countIn":
		return true
	}
	return false
}

func addUniqueGame(out *[]string, seen map[string]bool, game string) {
	if game == "" || seen[game] {
		return
	}
	seen[game] = true
	*out = append(*out, game)
}

func findZipChart(files map[string]*zip.File) (string, bool) {
	if _, ok := files["Charts/chart0.json"]; ok {
		return "Charts/chart0.json", true
	}
	names := make([]string, 0)
	for name := range files {
		if strings.HasPrefix(name, "Charts/") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	return names[0], true
}

func readZipFile(files map[string]*zip.File, name string) ([]byte, bool) {
	f, ok := findZipFile(files, name)
	if !ok {
		return nil, false
	}
	rc, err := f.Open()
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return b, err == nil
}

func readLibraryLevelIcon(files map[string]*zip.File) *ebiten.Image {
	for _, name := range []string{
		"Resources/Images/LibraryIcon/LibraryLevelIcon.png",
		"Resources/Images/LibraryIcon/LibraryLevelIcon.jpg",
		"Resources/Images/LibraryIcon/LibraryLevelIcon.jpeg",
	} {
		f, ok := findZipFile(files, name)
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		img, _, err := image.Decode(rc)
		rc.Close()
		if err != nil {
			log.Printf("engine: decode library icon %s: %v", name, err)
			continue
		}
		return ebiten.NewImageFromImage(img)
	}
	return nil
}

func findZipFile(files map[string]*zip.File, name string) (*zip.File, bool) {
	if f, ok := files[name]; ok {
		return f, true
	}
	for path, f := range files {
		if strings.EqualFold(path, name) {
			return f, true
		}
	}
	return nil, false
}

func menuString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func menuFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return 0
}

func (l menuLevel) displayName() string {
	if l.title != "" {
		return l.title
	}
	return l.fileName
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
	a.scores = nil
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
	a.scores = nil
	a.result, a.resultT, a.resultEpilogue = resultSummary{}, 0, false
	a.stopResultAudio()
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
	weight, category := a.resultSectionAt(beat)
	a.inputs = append(a.inputs, &Input{
		Beat: beat, hitT: a.bm.BeatToTime(beat), Release: release, Action: action,
		Weight: weight, Category: category, OnHit: onHit, OnMiss: onMiss,
	})
}

func (a *App) resultSectionAt(beat float64) (float64, int) {
	weight, category := 1.0, 0
	if a.bm == nil {
		return weight, category
	}
	for _, s := range a.bm.Sections {
		if s.Beat > beat {
			break
		}
		weight, category = s.Weight, s.Category
	}
	return weight, category
}

func (a *App) recordInputScore(in *Input, accuracy float64) {
	if in.Weight <= 0 {
		return
	}
	a.scores = append(a.scores, resultScoreInput{
		Beat: in.Beat, Accuracy: math.Max(0, math.Min(1, accuracy)),
		Weight: in.Weight, Category: in.Category,
	})
}

func (a *App) recordMissScore(beat float64) {
	weight, category := a.resultSectionAt(beat)
	if weight <= 0 {
		return
	}
	a.scores = append(a.scores, resultScoreInput{
		Beat: beat, Accuracy: 0, Weight: weight, Category: category,
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
		a.resultT += 1 / float64(ebiten.TPS())
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			a.stopResultAudio()
			return a.restart()
		}
		if titlePressed() {
			if !a.resultEpilogue {
				if a.resultT < resultRankTime {
					a.resultT = resultRankTime
					a.skipResultAudioToRank()
				} else {
					a.enterResultEpilogue()
				}
			} else if a.resultT > 1.5 {
				a.stopResultAudio()
				return a.restart()
			}
		}
		a.updateResultAudio()
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
		a.moveMenu(-menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) {
		a.moveMenu(menuGridCols)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) {
		a.moveMenu(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD) {
		a.moveMenu(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		a.moveMenu(-menuVisibleItems)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		a.moveMenu(menuVisibleItems)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		a.moveMenu(-menuGridCols)
	} else if wheelY < 0 {
		a.moveMenu(menuGridCols)
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
	if a.menuSel >= a.menuScroll+menuVisibleItems {
		a.menuScroll = a.menuSel - menuVisibleItems + 1
	}
	maxScroll := len(a.levels) - menuVisibleItems
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
	if x < menuGridX || x >= menuGridX+menuGridCols*(menuCardW+menuCardGapX)-menuCardGapX {
		return 0, false
	}
	if y < menuGridY || y >= menuGridY+menuGridRows*(menuCardH+menuCardGapY)-menuCardGapY {
		return 0, false
	}
	colStep := menuCardW + menuCardGapX
	rowStep := menuCardH + menuCardGapY
	col := (x - menuGridX) / colStep
	row := (y - menuGridY) / rowStep
	if x >= menuGridX+col*colStep+menuCardW || y >= menuGridY+row*rowStep+menuCardH {
		return 0, false
	}
	idx := a.menuScroll + row*menuGridCols + col
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
		a.loadErr = fmt.Sprintf("read %s failed: %v", level.displayName(), err)
		return
	}
	if err := a.loadRiq(r); err != nil {
		a.loadErr = fmt.Sprintf("load %s failed: %v", level.displayName(), err)
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
			a.recordInputScore(in, 0)
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
		a.enterResult()
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
	a.recordInputScore(best, accuracyForDiff(math.Abs(signed)))
	if j == JudgeAce && a.starBeat >= 0 && !a.starGot && math.Abs(best.Beat-a.starBeat) < 0.25 {
		a.starGot = true
		a.setMsg("SKILL STAR!")
	}
	a.pushTiming(signed, j)
	if best.OnHit != nil {
		best.OnHit(state, j)
	}
}

func accuracyForDiff(d float64) float64 {
	switch {
	case d <= WinAce:
		return 1
	case d <= WinJust:
		u := (d - WinAce) / (WinJust - WinAce)
		return rankHiThreshold + (1-u)*(1-rankHiThreshold)
	case d <= WinNG:
		u := (d - WinJust) / (WinNG - WinJust)
		return (1 - u) * rankOkThreshold
	default:
		return 0
	}
}

func (a *App) enterResult() {
	a.cond.Pause()
	a.result = a.buildResultSummary()
	a.resultT = 0
	a.resultEpilogue = false
	a.resetResultAudioCues()
	a.state = stateResult
}

func (a *App) buildResultSummary() resultSummary {
	totalWeight, weightedScore := 0.0, 0.0
	noMiss, perfect := len(a.scores) > 0, len(a.scores) > 0
	for _, in := range a.scores {
		totalWeight += in.Weight
		weightedScore += math.Max(0, math.Min(1, in.Accuracy)) * in.Weight
		if in.Accuracy < rankOkThreshold {
			noMiss = false
		}
		if in.Accuracy < 1 {
			perfect = false
		}
	}
	score := 0.0
	if totalWeight > 0 {
		score = weightedScore / totalWeight
	}
	rank := resultRankHi
	suffix := "hi"
	switch {
	case score < rankOkThreshold:
		rank, suffix = resultRankNg, "ng"
	case score < rankHiThreshold:
		rank, suffix = resultRankOk, "ok"
	}

	res := resultSummary{
		Score: score, Rank: rank, Header: a.resultProp("resultcaption"),
		NoMiss: noMiss, Perfect: perfect, Star: a.starGot,
	}
	cats := a.resultCategories()
	catScores := a.resultCategoryScores()
	if len(cats) <= 1 {
		res.Message0 = a.resultProp("resultcommon_" + suffix)
		return res
	}

	switch rank {
	case resultRankOk:
		best, bestScore := cats[0], -1.0
		for _, cat := range cats {
			if catScores[cat] > bestScore {
				best, bestScore = cat, catScores[cat]
			}
		}
		if bestScore >= rankHiThreshold {
			res.SubRank = true
			res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_hi", best))
		} else {
			res.Message0 = a.resultProp("resultcommon_ok")
		}
	case resultRankNg:
		first, second := twoExtremeCategories(cats, catScores, true)
		res.TwoMessage = catScores[second] < rankOkThreshold
		res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_ng", first))
		res.Message1 = res.Message0
		res.Message2 = resultSecondMessage(a.resultProp(fmt.Sprintf("resultcat%d_ng", second)))
	case resultRankHi:
		first, second := twoExtremeCategories(cats, catScores, false)
		res.TwoMessage = catScores[second] >= rankHiThreshold
		res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_hi", first))
		res.Message1 = res.Message0
		res.Message2 = resultSecondMessage(a.resultProp(fmt.Sprintf("resultcat%d_hi", second)))
	}
	return res
}

func (a *App) resultCategories() []int {
	seen := map[int]bool{}
	var cats []int
	if a.bm != nil && len(a.bm.Sections) > 0 {
		for _, s := range a.bm.Sections {
			if !seen[s.Category] {
				seen[s.Category] = true
				cats = append(cats, s.Category)
			}
		}
	}
	for _, in := range a.scores {
		if !seen[in.Category] {
			seen[in.Category] = true
			cats = append(cats, in.Category)
		}
	}
	if len(cats) == 0 {
		cats = []int{0}
	}
	sort.Ints(cats)
	return cats
}

func (a *App) resultCategoryScores() map[int]float64 {
	type bucket struct{ score, weight float64 }
	buckets := map[int]bucket{}
	for _, in := range a.scores {
		b := buckets[in.Category]
		b.score += math.Max(0, math.Min(1, in.Accuracy)) * in.Weight
		b.weight += in.Weight
		buckets[in.Category] = b
	}
	out := map[int]float64{}
	for _, cat := range a.resultCategories() {
		if b := buckets[cat]; b.weight > 0 {
			out[cat] = b.score / b.weight
		} else {
			out[cat] = 0
		}
	}
	return out
}

func twoExtremeCategories(cats []int, scores map[int]float64, lowest bool) (int, int) {
	first, second := cats[0], cats[0]
	firstScore, secondScore := scores[first], scores[first]
	for i, cat := range cats {
		score := scores[cat]
		if i == 0 || (lowest && score < firstScore) || (!lowest && score > firstScore) {
			second, secondScore = first, firstScore
			first, firstScore = cat, score
			continue
		}
		if second == first || (lowest && score < secondScore) || (!lowest && score > secondScore) {
			second, secondScore = cat, score
		}
	}
	return first, second
}

func resultSecondMessage(s string) string {
	if len(s) > 1 {
		rs := []rune(s)
		if len(rs) > 1 && !isLowerASCII(rs[0]) && isLowerASCII(rs[1]) {
			rs[0] = []rune(strings.ToLower(string(rs[0])))[0]
			s = string(rs)
		}
	}
	return "Also... " + s
}

func isLowerASCII(r rune) bool { return r >= 'a' && r <= 'z' }

func (a *App) resultProp(key string) string {
	if a.bm != nil {
		if s := a.bm.Prop(key); s != "" {
			return s
		}
	}
	defaults := map[string]string{
		"resultcaption":   "Rhythm League Notes",
		"resultcommon_hi": "That was great! Really great!",
		"resultcommon_ok": "Eh. Passable.",
		"resultcommon_ng": "That...could have been better.",
		"resultcat0_hi":   "You show strong fundamentals.",
		"resultcat0_ng":   "Work on your fundamentals.",
		"resultcat1_hi":   "You kept the beat well.",
		"resultcat1_ng":   "You had trouble keeping the beat.",
		"resultcat2_hi":   "You had great aim.",
		"resultcat2_ng":   "Your aim was a little shaky.",
		"resultcat3_hi":   "You followed the example well.",
		"resultcat3_ng":   "Next time, follow the example better.",
		"epilogue_hi":     "Superb",
		"epilogue_ok":     "OK",
		"epilogue_ng":     "Try Again",
	}
	if s, ok := defaults[key]; ok {
		return s
	}
	switch {
	case strings.HasPrefix(key, "resultcat") && strings.HasSuffix(key, "_hi"):
		return defaults["resultcommon_hi"]
	case strings.HasPrefix(key, "resultcat") && strings.HasSuffix(key, "_ng"):
		return defaults["resultcommon_ng"]
	}
	return ""
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
	a.drawLibraryBackground(screen)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, 78, color.RGBA{255, 250, 236, 220}, false)
	vector.DrawFilledRect(screen, 0, 77, ScreenW, 2, color.RGBA{118, 88, 148, 160}, false)
	vector.DrawFilledRect(screen, 0, ScreenH-54, ScreenW, 54, color.RGBA{255, 250, 236, 220}, false)

	ink := color.RGBA{66, 50, 88, 255}
	soft := color.RGBA{104, 92, 118, 255}
	a.text(screen, "Library", a.faceBig, 58, 20, ink, false)
	a.text(screen, "HEAVEN GO", a.faceSmall, 854, 30, color.RGBA{122, 105, 142, 255}, true)

	if len(a.levels) == 0 {
		vector.DrawFilledRect(screen, 248, 178, 464, 154, color.RGBA{255, 252, 242, 230}, false)
		vector.DrawFilledRect(screen, 248, 178, 464, 4, color.RGBA{118, 88, 148, 210}, false)
		a.text(screen, "No .riq levels found under levels/", a.faceMid, ScreenW/2, 222, ink, true)
		a.text(screen, "Drop a .riq file here to play", a.faceMid, ScreenW/2, 274, soft, true)
		if a.loadErr != "" {
			a.text(screen, a.fitText(a.loadErr, a.faceSmall, 760), a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
		}
		return
	}

	a.keepMenuSelectionVisible()
	for slot := 0; slot < menuVisibleItems; slot++ {
		idx := a.menuScroll + slot
		if idx >= len(a.levels) {
			break
		}
		col := slot % menuGridCols
		row := slot / menuGridCols
		x := float64(menuGridX + col*(menuCardW+menuCardGapX))
		y := float64(menuGridY + row*(menuCardH+menuCardGapY))
		a.drawLevelCard(screen, a.levels[idx], idx, x, y, idx == a.menuSel)
	}

	first := a.menuScroll + 1
	last := a.menuScroll + menuVisibleItems
	if last > len(a.levels) {
		last = len(a.levels)
	}
	a.drawLibraryLevelInfo(screen, a.levels[a.menuSel], a.menuSel, ink, soft)
	a.text(screen, fmt.Sprintf("%d-%d / %d", first, last, len(a.levels)), a.faceSmall, 66, ScreenH-34, soft, false)
	a.text(screen, "Enter / Click    Arrows / WASD    Drop .riq", a.faceSmall, ScreenW/2, ScreenH-34, soft, true)
	if a.loadErr != "" {
		a.text(screen, a.fitText(a.loadErr, a.faceSmall, 840), a.faceSmall, ScreenW/2, ScreenH-18, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLibraryBackground(screen *ebiten.Image) {
	if a.libraryAssets.bgBase == nil {
		screen.Fill(color.RGBA{231, 226, 215, 255})
		return
	}
	drawImageCover(screen, a.libraryAssets.bgBase, 0, 0, ScreenW, ScreenH, 1)
	drawImageCover(screen, a.libraryAssets.bgGradient, 0, 0, ScreenW, ScreenH, 0.7)
	drawImageCover(screen, a.libraryAssets.bgStars, 0, 0, ScreenW, ScreenH, 0.28)
	drawImageCover(screen, a.libraryAssets.bgWaves, 0, 0, ScreenW, ScreenH, 0.24)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{255, 250, 238, 70}, false)
}

func (a *App) drawLevelCard(screen *ebiten.Image, level menuLevel, idx int, x, y float64, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-5), float32(y-5), menuCardW+10, menuCardH+10, color.RGBA{255, 227, 95, 235}, false)
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), menuCardW, menuCardH, color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-37), menuCardW, 37, color.RGBA{246, 239, 229, 245}, false)
	if selected {
		vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-4), menuCardW, 4, color.RGBA{118, 88, 148, 255}, false)
	}
	a.drawLevelThumbnail(screen, level, idx, x+11, y+9, 126)
	title := a.fitText(level.displayName(), a.faceSmall, menuCardW-20)
	a.text(screen, title, a.faceSmall, x+10, y+menuCardH-29, color.RGBA{68, 54, 82, 255}, false)
	meta := "RIQ"
	if len(level.games) > 0 {
		meta = fmt.Sprintf("%d games", len(level.games))
	}
	a.text(screen, meta, a.faceSmall, x+10, y+menuCardH-12, color.RGBA{120, 106, 133, 255}, false)
}

func (a *App) drawLevelThumbnail(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	inner := size * 0.78
	innerX := x + (size-inner)/2
	innerY := y + (size-inner)/2
	vector.DrawFilledRect(screen, float32(innerX), float32(innerY), float32(inner), float32(inner), color.RGBA{240, 232, 220, 255}, false)
	if level.customIcon != nil {
		drawImageFit(screen, level.customIcon, innerX, innerY, inner, inner, 1)
	} else {
		a.drawFallbackLevelIcon(screen, level, idx, innerX, innerY, inner)
	}
	if a.libraryAssets.border != nil {
		drawImageFit(screen, a.libraryAssets.border, x, y, size, size, 1)
	} else {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y+size-5), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x+size-5), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
	}
}

func (a *App) drawFallbackLevelIcon(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	games := level.games
	if len(games) == 0 {
		games = []string{"RIQ"}
	}
	if len(games) == 1 {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), float32(size), menuAccent(idx), false)
		a.text(screen, a.fitText(menuGameLabel(games[0]), a.faceMid, size-18), a.faceMid, x+size/2, y+size/2-12, color.RGBA{255, 252, 242, 255}, true)
		return
	}
	tile := (size - 5) / 2
	for i := 0; i < 4; i++ {
		tx := x + float64(i%2)*(tile+5)
		ty := y + float64(i/2)*(tile+5)
		c := menuAccent(idx + i)
		vector.DrawFilledRect(screen, float32(tx), float32(ty), float32(tile), float32(tile), c, false)
		if i < len(games) {
			label := a.fitText(menuGameLabel(games[i]), a.faceSmall, tile-8)
			a.text(screen, label, a.faceSmall, tx+tile/2, ty+tile/2-8, color.RGBA{255, 252, 242, 255}, true)
		}
	}
}

func (a *App) drawLibraryLevelInfo(screen *ebiten.Image, level menuLevel, idx int, ink, soft color.RGBA) {
	x, y := 585.0, 116.0
	w, h := 326.0, 344.0
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), 5, color.RGBA{118, 88, 148, 230}, false)
	a.drawLevelThumbnail(screen, level, idx, x+24, y+32, 112)
	a.text(screen, a.fitText(level.displayName(), a.faceMid, 172), a.faceMid, x+154, y+38, ink, false)
	author := level.author
	if author == "" {
		author = "Unknown author"
	}
	a.text(screen, a.fitText(author, a.faceSmall, 150), a.faceSmall, x+156, y+78, soft, false)
	a.text(screen, fmt.Sprintf("%.0f BPM", level.bpm), a.faceSmall, x+156, y+104, color.RGBA{118, 88, 148, 255}, false)

	baseY := y + 176
	games := level.games
	if len(games) == 0 {
		games = []string{"Unknown game"}
	}
	a.text(screen, "Games", a.faceSmall, x+24, baseY, soft, false)
	a.drawGameList(screen, games, x+24, baseY+26, w-48)
	desc := strings.TrimSpace(level.desc)
	if desc != "" {
		a.text(screen, "Description", a.faceSmall, x+24, y+276, soft, false)
		a.drawWrappedTextLimit(screen, desc, a.faceSmall, x+24, y+302, w-48, 20, 2, ink)
	} else {
		a.text(screen, a.fitText(level.path, a.faceSmall, w-48), a.faceSmall, x+24, y+h-46, soft, false)
	}
}

func (a *App) drawGameList(screen *ebiten.Image, games []string, x, y, maxW float64) {
	cx := x
	for i, game := range games {
		label := a.fitText(menuGameLabel(game), a.faceSmall, 100)
		tw, _ := text.Measure(label, a.faceSmall, 0)
		pw := math.Min(tw+22, 120)
		if cx+pw > x+maxW {
			break
		}
		vector.DrawFilledRect(screen, float32(cx), float32(y), float32(pw), 24, menuAccent(i), false)
		a.text(screen, label, a.faceSmall, cx+11, y+6, color.RGBA{255, 252, 242, 255}, false)
		cx += pw + 8
	}
}

func (a *App) drawWrappedTextLimit(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, maxLines int, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 || maxLines <= 0 {
		return
	}
	line := words[0]
	lines := 0
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		lines++
		if lines >= maxLines {
			a.text(screen, a.fitText(line+"...", face, maxW), face, x, y, c, false)
			return
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
}

func menuGameLabel(game string) string {
	names := map[string]string{
		"blueBear":       "Blue Bear",
		"marchingOrders": "Marching Orders",
		"munchyMonk":     "Munchy Monk",
		"seeSaw":         "See-Saw",
		"somen":          "Somen",
		"totemClimb":     "Totem Climb",
		"trickClass":     "Trick Class",
	}
	if name, ok := names[game]; ok {
		return name
	}
	return humanizeGameID(game)
}

func humanizeGameID(id string) string {
	if id == "" {
		return "Unknown"
	}
	var b strings.Builder
	for i, r := range id {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		if i == 0 && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
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
	if a.resultEpilogue {
		a.drawResultEpilogue(screen, white)
		return
	}
	a.drawJudgementBackground(screen)

	panel := color.RGBA{245, 239, 224, 235}
	ink := color.RGBA{58, 46, 64, 255}
	dim := color.RGBA{112, 98, 118, 255}
	vector.DrawFilledRect(screen, 60, 82, 530, 255, panel, false)
	vector.DrawFilledRect(screen, 60, 82, 530, 42, resultRankColor(a.result.Rank), false)
	vector.DrawFilledRect(screen, 60, 333, 530, 4, color.RGBA{58, 46, 64, 180}, false)
	a.text(screen, a.result.Header, a.faceMid, 82, 94, color.RGBA{255, 252, 242, 255}, false)

	if a.result.TwoMessage {
		if a.resultT >= resultMsgTime {
			a.drawWrappedText(screen, a.result.Message1, a.faceMid, 90, 155, 455, 30, ink)
		}
		if a.resultT >= resultMsg2Time {
			a.drawWrappedText(screen, a.result.Message2, a.faceMid, 90, 235, 455, 30, ink)
		}
	} else if a.resultT >= resultMsgTime {
		a.drawWrappedText(screen, a.result.Message0, a.faceMid, 90, 178, 455, 32, ink)
	}

	scoreShown := a.result.Score
	if a.resultT < resultBarStart {
		scoreShown = 0
	} else if a.resultT < resultBarStart+resultBarDur {
		scoreShown *= (a.resultT - resultBarStart) / resultBarDur
	}
	barColor := resultScoreColor(scoreShown)
	vector.DrawFilledRect(screen, 95, 388, 508, 32, color.RGBA{42, 38, 48, 220}, false)
	vector.DrawFilledRect(screen, 101, 394, 496, 20, color.RGBA{102, 90, 108, 255}, false)
	vector.DrawFilledRect(screen, 101, 394, float32(496*scoreShown), 20, barColor, false)
	vector.StrokeLine(screen, 101+float32(496*rankOkThreshold), 390, 101+float32(496*rankOkThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	vector.StrokeLine(screen, 101+float32(496*rankHiThreshold), 390, 101+float32(496*rankHiThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	a.text(screen, fmt.Sprintf("%d", int(scoreShown*100)), a.faceBig, 626, 379, barColor, false)

	if a.resultT >= resultRankTime {
		a.drawRankLogo(screen)
		if a.result.SubRank {
			a.text(screen, "...but, just", a.faceMid, 760, 306, dim, true)
		}
		if a.result.Star {
			a.drawBadge(screen, 86, 446, "SKILL STAR", color.RGBA{255, 220, 82, 255})
		}
		if a.result.NoMiss {
			a.drawBadge(screen, 236, 446, "NO MISS", color.RGBA{92, 205, 236, 255})
		}
		if a.result.Perfect {
			a.drawBadge(screen, 366, 446, "PERFECT", color.RGBA{255, 140, 210, 255})
		}
		a.text(screen, "Enter / Click - epilogue    R - replay    Esc - quit", a.faceSmall, ScreenW/2, ScreenH-34, white, true)
	} else {
		a.text(screen, "Enter / Click - skip    R - replay    Esc - quit", a.faceSmall, ScreenW/2, ScreenH-34, dim, true)
	}
}

func (a *App) drawJudgementBackground(screen *ebiten.Image) {
	if a.resultAssets.bg != nil {
		drawImageCover(screen, a.resultAssets.bg, 0, 0, ScreenW, ScreenH, 1)
	} else {
		screen.Fill(color.RGBA{41, 38, 58, 255})
	}
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{20, 18, 28, 90}, false)
}

func (a *App) drawRankLogo(screen *ebiten.Image) {
	switch a.result.Rank {
	case resultRankHi:
		if a.resultAssets.rankHi != nil {
			drawImageFit(screen, a.resultAssets.rankHi, 620, 104, 300, 120, 1)
		} else {
			a.text(screen, "SUPERB", a.faceBig, 768, 150, resultRankColor(resultRankHi), true)
		}
		if a.resultAssets.rankHiStar != nil {
			s := 54 + 5*math.Sin(a.resultT*5)
			drawImageFit(screen, a.resultAssets.rankHiStar, 842, 82, s, s, 1)
		}
	case resultRankOk:
		if a.resultAssets.rankOk != nil {
			drawImageFit(screen, a.resultAssets.rankOk, 656, 118, 230, 132, 1)
		} else {
			a.text(screen, "OK", a.faceBig, 768, 150, resultRankColor(resultRankOk), true)
		}
		if a.resultAssets.rankOkSweat != nil {
			drawImageFit(screen, a.resultAssets.rankOkSweat, 826, 98, 58, 25, 1)
		}
	case resultRankNg:
		img := firstResultImage(a.resultAssets.rankNg)
		if len(a.resultAssets.rankNg) > 0 {
			if frame := int(a.resultT*8) % len(a.resultAssets.rankNg); a.resultAssets.rankNg[frame] != nil {
				img = a.resultAssets.rankNg[frame]
			}
		}
		if img != nil {
			drawImageFit(screen, img, 616, 120, 315, 118, 1)
		} else {
			a.text(screen, "TRY AGAIN", a.faceBig, 768, 150, resultRankColor(resultRankNg), true)
		}
	}
}

func firstResultImage(imgs []*ebiten.Image) *ebiten.Image {
	for _, img := range imgs {
		if img != nil {
			return img
		}
	}
	return nil
}

func (a *App) drawBadge(screen *ebiten.Image, x, y float32, label string, c color.RGBA) {
	vector.DrawFilledRect(screen, x, y, 112, 27, color.RGBA{35, 31, 42, 210}, false)
	vector.DrawFilledRect(screen, x, y, 6, 27, c, false)
	a.text(screen, label, a.faceSmall, float64(x+15), float64(y+6), color.RGBA{242, 240, 248, 255}, false)
}

func (a *App) drawResultEpilogue(screen *ebiten.Image, white color.RGBA) {
	img := a.resultAssets.epNg
	msg := a.resultProp("epilogue_ng")
	switch a.result.Rank {
	case resultRankOk:
		img, msg = a.resultAssets.epOk, a.resultProp("epilogue_ok")
	case resultRankHi:
		img, msg = a.resultAssets.epHi, a.resultProp("epilogue_hi")
	}
	if img != nil {
		drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, 1)
	} else {
		screen.Fill(resultRankColor(a.result.Rank))
	}
	vector.DrawFilledRect(screen, 0, ScreenH-116, ScreenW, 116, color.RGBA{24, 20, 30, 218}, false)
	a.text(screen, msg, a.faceBig, 54, ScreenH-96, white, false)
	a.text(screen, fmt.Sprintf("Final score %d  |  ACE %d  OK %d  NG %d  MISS %d",
		int(a.result.Score*100), a.aces, a.justs, a.ngs, a.misses),
		a.faceSmall, 58, ScreenH-42, color.RGBA{218, 214, 226, 255}, false)
	a.text(screen, "Enter / Click - chart title    R - replay    Esc - quit", a.faceSmall, ScreenW-306, ScreenH-42, white, false)
}

func resultRankColor(rank resultRank) color.RGBA {
	switch rank {
	case resultRankHi:
		return color.RGBA{252, 191, 54, 255}
	case resultRankOk:
		return color.RGBA{90, 196, 217, 255}
	default:
		return color.RGBA{238, 80, 93, 255}
	}
}

func resultScoreColor(score float64) color.RGBA {
	switch {
	case score >= rankHiThreshold:
		return resultRankColor(resultRankHi)
	case score >= rankOkThreshold:
		return resultRankColor(resultRankOk)
	default:
		return resultRankColor(resultRankNg)
	}
}

func drawImageFit(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Min(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func drawImageCover(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Max(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func (a *App) drawWrappedText(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 {
		return
	}
	line := words[0]
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
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
