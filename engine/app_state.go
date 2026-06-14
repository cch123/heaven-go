package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"hsdemo/conductor"
	"hsdemo/riq"
)

type gameState int

const (
	stateTitle gameState = iota
	statePlay
	stateResult
)

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
