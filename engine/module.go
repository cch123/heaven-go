// Package engine 是移植版的"GameManager + Minigame 基类"：
// 拥有谱面时间轴、输入判定、游戏切换、HUD/时机条/flash 与结算；
// 每个 minigame 实现 Module 接口并注册，引擎按谱面实体路由。
package engine

import (
	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

// Judgment 是一次输入的评级（窗口常量见 engine.go）。
type Judgment int

const (
	JudgeNone Judgment = iota
	JudgeAce
	JudgeJust
	JudgeNG
	JudgeMiss
)

// Module 是一个 minigame 的玩法实现。
//
// 生命周期：Load（一次）→ 谱面载入时对每个本游戏实体调用 OnEvent →
// Ready → 播放中 switchGame 到本游戏时 OnSwitch → 每帧 Update/Draw。
// 判定由引擎完成：模块通过 ctx.ScheduleInput 注册目标拍与回调。
type Module interface {
	// ID 返回 datamodel 前缀（如 "rhythmSomen"）。
	ID() string
	// Load 加载资产并初始化场景。
	Load(ctx *Ctx) error
	// OnEvent 为一个谱面实体注册时间轴动作（载入期调用，按拍升序）。
	OnEvent(e *riq.Entity)
	// Ready 在全部 OnEvent 之后调用（用于 bop 区间等需要全量信息的调度）。
	Ready()
	// OnSwitch 在游戏成为活动游戏时调用（含曲首），做空闲动画初始化。
	OnSwitch(beat float64)
	// Whiff 处理非预期按键（无输入在窗口内）。
	Whiff(beat float64)
	// Update 每逻辑帧调用（仅活动时）。
	Update(t, beat float64)
	// Draw 绘制本游戏画面（含背景；HUD 由引擎叠加）。
	Draw(screen *ebiten.Image, t, beat float64)
}

// ActionWhiffer 是可区分动作通道空击的模块（双键游戏如 blueBear）。
// 实现它的模块不再收到 Whiff 调用（含主键空击）。
type ActionWhiffer interface {
	WhiffAction(beat float64, action int)
}

// factory 注册表
var registry = map[string]func() Module{}

// Register 注册一个 minigame 工厂，应在 main 里对每个已移植游戏调用。
func Register(id string, f func() Module) { registry[id] = f }

// Supported 报告游戏是否已移植。
func Supported(id string) bool { _, ok := registry[id]; return ok }

// ---------- 未移植游戏的占位 ----------

type placeholder struct {
	id  string
	ctx *Ctx
}

func newPlaceholder(id string) Module { return &placeholder{id: id} }

func (p *placeholder) ID() string             { return p.id }
func (p *placeholder) Load(ctx *Ctx) error    { p.ctx = ctx; return nil }
func (p *placeholder) OnEvent(e *riq.Entity)  {} // 不注册输入：乐曲继续，事件跳过
func (p *placeholder) Ready()                 {}
func (p *placeholder) OnSwitch(beat float64)  {}
func (p *placeholder) Whiff(beat float64)     {}
func (p *placeholder) Update(t, beat float64) {}
func (p *placeholder) Draw(screen *ebiten.Image, t, beat float64) {
	p.ctx.App.drawPlaceholder(screen, p.id)
}
