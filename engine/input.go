package engine

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
	// NoAutoplay 复刻 HS ScheduleUserInput：真实玩家可以触发回调，但
	// autoplay 不会代打，常用于“错误动作”窗口。
	NoAutoplay bool
	// OnHit 在 NG 窗口内的任意按键触发；state 为 just 窗归一化偏移
	//（|state|<=1 = just 命中，1<|state|<=2 = NG，负 = 早），与 C# 语义一致。
	OnHit func(state float64, j Judgment)
	// OnMiss 在超窗未按时触发。
	OnMiss func()
	// CanHit 对应 HS ScheduleInput 的 canJust 谓词。条件变 false 后，
	// 旧判定窗会静默失效，避免玩家对象已停止/消失后仍吃到输入。
	CanHit func() bool
}
