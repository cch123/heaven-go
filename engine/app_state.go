package engine

import (
	"github.com/hajimehoshi/ebiten/v2/audio"
)

type gameState int

const (
	stateTitle gameState = iota
	statePlay
	stateResult
)

var audioCtx *audio.Context // audio.NewContext 进程内只能调用一次

// App 实现 ebiten.Game。
//
// 运行时字段按职责拆在 app_runtime_state.go / app_ui_state.go。这里保留匿名
// 嵌入，让既有代码仍可用 a.bm、a.result 这类直接访问，同时避免继续把
// 谱面、输入、VFX、菜单和结算状态堆进一个不可维护的大结构体。
type App struct {
	appConfig
	chartRuntimeState
	moduleRuntimeState
	inputRuntimeState
	scoreRuntimeState
	effectsRuntimeState
	appFlowState
	timingDisplayState
	menuRuntimeState
	resultRuntimeState
	fontState
}
