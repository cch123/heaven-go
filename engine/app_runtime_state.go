package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"hsdemo/conductor"
	"hsdemo/riq"
)

type appConfig struct {
	assetsRoot string
}

// chartRuntimeState is rebuilt whenever a new .riq is loaded.
type chartRuntimeState struct {
	r      *riq.Riq
	bm     *riq.Beatmap
	cond   *conductor.Conductor
	player *audio.Player
}

type moduleRuntimeState struct {
	modules  map[string]Module
	active   Module
	switches []gameSwitch
	swIdx    int
	actions  []beatAction
	actIdx   int
	unported []string
}

type inputRuntimeState struct {
	inputs []*Input

	pressedNow  bool // 本逻辑帧是否有按下（模块轮询用，如 totemClimb HoldCo）
	releasedNow bool // 本逻辑帧是否有抬起
	inputOn     bool

	// LatencyMS：输入延迟校准（毫秒，正值 = 按键时间戳前移），'['/']' 热键 ±5ms
	LatencyMS float64

	// Autoplay：调试用完美自动打击（HS 的 autoplay 等价物），-autoplay 开启
	Autoplay bool
}

type scoreRuntimeState struct {
	scores []resultScoreInput

	starBeat float64
	starGot  bool

	aces, justs, ngs, misses, whiffs int
}

type effectsRuntimeState struct {
	// commonSounds：assets/common/sounds 的公共音效（countIn 计数音、
	// miss/nearMiss 等），缺目录时为空 map（相关事件静默跳过）
	commonSounds map[string][]byte

	flashes    []flashEvt
	camEvts    []camEvt
	musicFades []musicFadeEvt

	fx  postFX    // ppe/* 屏幕后处理
	flt filterFX  // vfx/filter（LUT 滤镜）
	tbx textboxFX // vfx/display textbox

	viewScales []viewScaleEvt // vfx/scale view（画布缩放）
	viewBuf    *ebiten.Image  // 缩放生效时的离屏画布
}

type appFlowState struct {
	state   gameState
	endBeat float64

	lastMsg string
	msgT    float64
	debug   bool
	loadErr string
}

type timingDisplayState struct {
	tdArrow, tdTarget float64
	tdHits            []timingHit
}
