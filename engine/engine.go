// engine.go：App 核心状态与构造。具体职责拆在同包的 load/update/menu/result/draw 等文件。
package engine

import (
	"bytes"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
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
	// NoScore 复刻 HS 的 countsForAccuracy=false：窗口吞掉输入并执行回调，
	// 但不写判定计数、Timing Bar 或结算分数。
	NoScore bool
	// OnHit 在 NG 窗口内的任意按键触发；state 为 just 窗归一化偏移
	//（|state|<=1 = just 命中，1<|state|<=2 = NG，负 = 早），与 C# 语义一致。
	OnHit func(state float64, j Judgment)
	// OnMiss 在超窗未按时触发。
	OnMiss func()
	// CanHit 对应 HS ScheduleInput 的 canJust 谓词。条件变 false 后，
	// 旧判定窗会静默失效，避免玩家对象已停止/消失后仍吃到输入。
	CanHit func() bool
}

// camEvt 是 vfx/move camera 事件（GameCamera.UpdateCameraTranslate 语义）。
type camEvt struct {
	beat, length float64
	target       [3]float64 // (valA, valB, -valC)
	ease         int
	axis         int // 0=All 1=X 2=Y 3=Z
}

type musicFadeEvt struct {
	beat, length float64
	from, to     float64
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
	y      float64 // normalized visual position on the TimingAccuracy bar, after prefab segment scaling.
	signed float64
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
	r          *riq.Riq
	bm         *riq.Beatmap
	cond       *conductor.Conductor
	player     *audio.Player
	modules    map[string]Module
	active     Module
	switches   []gameSwitch
	swIdx      int
	actions    []beatAction
	actIdx     int
	inputs     []*Input
	scores     []resultScoreInput
	flashes    []flashEvt
	camEvts    []camEvt
	musicFades []musicFadeEvt
	unported   []string

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
