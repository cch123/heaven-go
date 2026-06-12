// Package kmdata 定义资产导出管线的中间格式：
// 提取器（cmd/extract）把 Unity 资产写成这些结构的 JSON，
// 运行时（游戏）只认这些结构，不再接触 Unity 序列化格式。
package kmdata

// StepSlope 之外的斜率视为正常 Hermite 斜率；
// Unity 的 Infinity 斜率（阶跃帧）导出时编码为 ±1e30（JSON 不支持 Inf）。
const StepSlope = 1e29

// Key 是一个浮点曲线关键帧（Hermite 插值）。
type Key struct {
	T float64 `json:"t"`
	V float64 `json:"v"`
	I float64 `json:"i"` // inSlope
	O float64 `json:"o"` // outSlope
}

// SwapKey 是 sprite 换帧关键帧（阶跃）。
type SwapKey struct {
	T    float64 `json:"t"`
	Name string  `json:"name"`
}

// SpriteInfo 是图集中一个切片，X/Y 为图集内左上角像素坐标。
type SpriteInfo struct {
	X, Y, W, H     int
	PivotX, PivotY float64    // 归一化枢轴（Unity 约定：Y 向上）
	Atlas          int        `json:"atlas,omitempty"`  // 多图集时的索引（Sheet.Atlases）
	PPU            float64    `json:"ppu,omitempty"`    // 单切片 PPU；0 表示用 Sheet.PPU
	Border         [4]float64 `json:"border,omitempty"` // 九宫格边距 px：左/下/右/上（Unity x/y/z/w）
}

// Sheet 是切片表。单图集时用 Atlas/PPU；多图集时用 Atlases + 切片级字段。
type Sheet struct {
	Atlas   string                `json:"atlas,omitempty"`   // 单图集文件名（legacy）
	Atlases []string              `json:"atlases,omitempty"` // 多图集文件名列表
	PPU     float64               `json:"ppu"`               // 默认 pixels per unit
	Sprites map[string]SpriteInfo `json:"sprites"`
}

// Node 是场景/骨架节点（prefab 子树的一员），DFS 先序排列，Parent 必在自身之前。
type Node struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"` // 相对子树根，如 "LeftArm/LeftArmAttatch"
	Parent   int        `json:"parent"`
	Pos      [2]float64 `json:"pos"`
	PosZ     float64    `json:"z,omitempty"` // 深度（透视投影用，相机在 z=-10）
	RotZ     float64    `json:"rotZ"`        // 弧度
	Scale    [2]float64 `json:"scale"`
	Sprite   string     `json:"sprite,omitempty"`
	Order    int        `json:"order"`           // SpriteRenderer sortingOrder
	Layer    int        `json:"layer,omitempty"` // SpriteRenderer sortingLayer
	Hidden   bool       `json:"hidden,omitempty"`
	FlipX    bool       `json:"flipX,omitempty"`
	Inactive bool       `json:"inactive,omitempty"` // GameObject m_IsActive == 0
	Color    [4]float64 `json:"color,omitempty"`    // SpriteRenderer m_Color（零值视为白色）
	FlipY    bool       `json:"flipY,omitempty"`
	DrawMode int        `json:"drawMode,omitempty"` // 0=simple 1=sliced 2=tiled
	Size     [2]float64 `json:"size,omitempty"`     // drawMode != 0 时的渲染尺寸（unit）
	// SortGroup 非空表示该节点挂有 Unity SortingGroup（[layer, order]）：
	// 整个子树作为单一排序单元参与全局排序，子树内部再按各自 order 排。
	SortGroup []int `json:"sortGroup,omitempty"`
}

// Rig 是一棵节点树（KarateMan 的单骨架与整游戏场景共用此结构）。
type Rig struct {
	Nodes []Node `json:"nodes"`
}

// Roles 是游戏脚本序列化字段名 → 场景节点 path 的绑定表
// （如 "FarCrane" → "Crane/FarArm"），由提取器从 prefab 的 MonoBehaviour 解析。
type Roles map[string]string

// CurvePoint 是 NaughtyBezierCurves 关键点（世界坐标 x/y/z，含两侧控制柄）。
// z 用于透视投影：对象沿曲线从背景（z 大）飞向前景时近大远小。
type CurvePoint struct {
	P  [3]float64 `json:"p"`
	LH [3]float64 `json:"lh"`
	RH [3]float64 `json:"rh"`
}

// Curve 是一条完整曲线：Sampling 为序列化的采样密度，
// 弧长估算的每段子采样数 = Sampling/(len(Points)-1) + 1（原版同式）。
type Curve struct {
	Sampling int          `json:"sampling"`
	Points   []CurvePoint `json:"points"`
}

// SeqClip 是音效序列（SoundSequence）中的一个片段。
type SeqClip struct {
	Clip   string  `json:"clip"` // 音效文件主名（已剥离 game/ 前缀）
	Beat   float64 `json:"beat"` // 相对序列起拍的偏移
	Volume float64 `json:"volume"`
}

// Extra 是游戏脚本的扩展序列化数据（数组引用、字符串表、Bezier 曲线、
// 对象模板字段、音效序列），按需由提取器填充。
type Extra struct {
	RefArrays map[string][]string           `json:"refArrays,omitempty"` // 字段 → 节点 path 列表
	Strings   map[string][]string           `json:"strings,omitempty"`   // 字段 → 字符串列表
	Curves    map[string]Curve              `json:"curves,omitempty"`    // 字段 → 曲线
	ObjNums   map[string]map[string]float64 `json:"objNums,omitempty"`   // 模板 path → 数值字段
	ObjStrs   map[string]map[string]string  `json:"objStrs,omitempty"`   // 模板 path → 字符串字段
	Sequences map[string][]SeqClip          `json:"sequences,omitempty"` // 音效序列名 → 片段
}

// XYCurve 是二维向量曲线（位置/缩放按分量存）。
type XYCurve struct {
	X []Key `json:"x,omitempty"`
	Y []Key `json:"y,omitempty"`
}

// Stage 是从 KarateManPot 序列化字段提取的舞台/轨迹参数（path=1，普通罐子），
// 坐标已换算为相对 Joe 骨架根的单位空间。
// 原版轨迹公式（KarateManPot.ProgressToFlyPosition）：
//
//	progress  = clamp((beat - throwBeat)/2, 0, 1-Slip)   // 全程 2 拍，判定在第 1 拍
//	pHit      = progress + (HitOffset - 0.5)
//	flyHeight = pHit*(pHit-1) / (HitOffset*(HitOffset-1)) // 归一化抛物线，判定时刻恰为 1
//	x         = lerp(HitPos.x+StartOffset.x, HitPos.x-StartOffset.x, progress)
//	y         = FloorY + (HitPos.y-FloorY + StartOffset.y*(1-min(beat-throwBeat,1))) * flyHeight
type Stage struct {
	HitPos       [2]float64 `json:"hitPos"`       // 判定点（拳头）
	FloorY       float64    `json:"floorY"`       // 地面 y
	StartOffset  [2]float64 `json:"startOffset"`  // 起点相对判定点的偏移（x, y）
	StartOffsetZ float64    `json:"startOffsetZ"` // 深度偏移：z 从 hit+z0 飞到 hit-z0（透视进场）
	HitOffset    float64    `json:"hitOffset"`    // HitPositionOffset[path]
	Slip         float64    `json:"slip"`         // ItemSlipRt[path]
}

// Anim 是一条动画剪辑，键为节点 path。
type Anim struct {
	Duration float64                     `json:"duration"`
	Loop     bool                        `json:"loop"`
	Pos      map[string]XYCurve          `json:"pos,omitempty"`
	Euler    map[string][]Key            `json:"euler,omitempty"` // z 轴角度（度）
	Scale    map[string]XYCurve          `json:"scale,omitempty"`
	Sprites  map[string][]SwapKey        `json:"sprites,omitempty"`
	Floats   map[string]map[string][]Key `json:"floats,omitempty"` // path → attribute → curve
}
