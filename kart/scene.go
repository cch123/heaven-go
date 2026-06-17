// scene.go：整场景运行时——节点树 + 多个并行的 Animator 播放器。
//
// 与单骨架 RigInst 的区别：
//   - 节点保留 prefab 的世界摆位（根不归零），由 proj 模拟相机；
//   - 任意子树根可以绑定一个播放器（对应 Unity Animator），同时播放；
//   - 剪辑本地时间以"拍"为基准：clipT(秒) = 经过拍数 × timeScale，
//     复刻 HS DoScaledAnimationAsync 的速度语义（动画速度随 BPM 缩放）；
//   - 支持 GameObject m_IsActive 的层级传播与 m_Color 调色。
package kart

import (
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kmdata"
)

type sceneNodeState struct {
	pos             [2]float64
	rot             float64
	scale           [2]float64
	sprite          string
	flipX           bool
	flipY           bool
	active          bool // GameObject m_IsActive（沿层级传播）
	renderOn        bool // SpriteRenderer m_Enabled（仅本节点，不传播）
	color           [4]float64
	matColor        [4]float64
	hasMatColor     bool
	matAlpha        float64 // material._Alpha：材质级透明度，最终与 SpriteRenderer.color.a 相乘
	matOpacity      float64 // material._Opacity：Airboarder 透明材质的独立淡入淡出因子
	matProgress     float64 // material._Progress：ChargingChicken ChickenCar 充能渐变
	hasMatProgress  bool
	matAdd          [4]float64
	matBlend        [4]float64
	matHueShift     float64
	matLinearAdd    bool
	matDoodle       DoodleParams
	outlineWidth    float64
	matThreshold    float64
	hasMatThreshold bool
	palette         Palette
	hasPalette      bool
	size            [2]float64 // drawMode != 0 时生效
	order           int        // sortingOrder（可被动画驱动）
}

// scenePlayer 是绑定到某个子树根的剪辑播放器。
type scenePlayer struct {
	rootIdx   int
	rootPath  string
	anim      *kmdata.Anim
	clipName  string
	startBeat float64
	timeScale float64

	normalized bool    // DoNormalizedAnimation 语义：固定在归一化时间 normT 采样
	normT      float64 // [0,1]
}

// smachine 是绑定到子树根的 AnimatorController 状态机
// （DoScaledAnimationAsync 按状态名播放 + 剪辑结束按 bool 条件转换）。
type smachine struct {
	ctrl   *kmdata.Controller
	state  string
	params map[string]bool
	lastT  float64 // 上次 Sample 的剪辑时间（循环边界跨越检测）
}

// SceneInst 是一个可播放多路动画的场景实例。
type SceneInst struct {
	as          *Assets
	byPath      map[string]int
	state       []sceneNodeState
	world       []Aff
	worldZ      []float64 // 节点深度（透视投影：s = CamDist/(CamDist+z)）
	actives     []bool    // activeInHierarchy
	groupOf     []int     // 节点归属的 SortingGroup 节点下标（-1 = 无）
	players     map[string]*scenePlayer
	playerOrder []string // Unity layer 叠放顺序有定义；Go map 采样不能决定覆盖顺序。

	machines map[string]*smachine // rootPath → 状态机（有 controller 的 Animator）

	// 模块驱动的持久覆盖（在 prefab 默认值之后、剪辑采样之前生效）
	spinOver   map[int]float64    // 节点下标 → 旋转叠加（弧度，transform.Rotate 积分）
	activeOver map[int]bool       // 节点下标 → m_IsActive 覆盖（SetActive 语义）
	renderOver map[int]bool       // 节点下标 → SpriteRenderer.enabled 覆盖（不沿层级传播）
	mirrorOver map[int]bool       // 节点下标 → localScale.x 取负（transform.localScale=(-1,1,1)）
	colorOver  map[int][4]float64 // 节点下标 → SpriteRenderer.color 覆盖
	matOver    map[int]materialState
	matFor     map[string]materialState
	matForSkip map[string][]string
	texFor     map[string][2]float64
	palOver    map[int]Palette
	posOver    map[int][2]float64 // 节点下标 → localPosition 覆盖（伪相机平移等）
	scaleOver  map[int][2]float64 // 节点下标 → localScale 覆盖（脚本瞬时 squash/pose）
	sizeOver   map[int][2]float64 // 节点下标 → SpriteRenderer.size 覆盖（sliced/tiled）
	orderOver  map[int]int        // 节点下标 → SpriteRenderer.sortingOrder 覆盖
	zOver      map[int]float64    // 节点下标 → 世界 z 覆盖（kitties 斜列生成等，根节点语义）
	spriteOver map[int]string     // 节点下标 → 切片覆盖（sr.sprite 直写，如海报换图）

	queued       []ExtraSprite // 本帧注入的动态 SpriteRenderer 绘制项
	queuedMeshes []ExtraMesh   // 本帧注入的动态 MeshRenderer 绘制项

	cam    [3]float64 // 相机世界位置（vfx/move camera），默认 (0,0,-10)
	camFOV float64    // 纵向 FOV，0 表示使用 GameCamera 默认 FOV
	camYaw float64    // 绕世界 y 轴的相机 orbit yaw（弧度），BuiltToScaleDS cameraPivot
	camRight,
	camUp,
	camForward [3]float64 // 相机 local axes in world space
	hasCam      bool
	hasCamBasis bool

	palette  Palette            // 映射材质默认调色板（单材质游戏）
	palettes map[string]Palette // 按材质名（Node.Mat）覆盖（多材质游戏）

	drawOrder []int // 预排序的可绘制节点（layer, order, dfs）

	scratch *ebiten.Image // SpriteMask 合成的离屏缓冲（懒分配）
	maskBuf *ebiten.Image
}

type materialState struct {
	color     [4]float64
	hasColor  bool
	colors    map[string][4]float64
	add       [4]float64
	hueShift  float64
	linearAdd bool
	doodle    DoodleParams
}

// SetCamera 设置相机世界位置（GameCamera：默认 (0,0,-10)、FOV 53.15°）。
// 透视缩放从 s = CamDist/(CamDist+z) 推广为 s = CamDist/(z - camZ)，
// 屏幕坐标先平移 -cam.xy 再缩放（vfx/move camera 的拉近/平移）。
func (s *SceneInst) SetCamera(x, y, z float64) {
	s.cam, s.hasCam = [3]float64{x, y, z}, true
}

// SetCameraFOV sets the vertical camera field of view in degrees. A non-positive
// or invalid value restores the default Heaven Studio GameCamera projection.
func (s *SceneInst) SetCameraFOV(deg float64) {
	if deg <= 0 || math.IsNaN(deg) || math.IsInf(deg, 0) || deg >= 179 {
		s.camFOV = 0
		return
	}
	s.camFOV = deg
}

// SetCameraYaw sets the camera's orbit rotation around the vertical world axis.
// Most HS games use 2D camera moves and should leave this at 0; mesh-heavy games
// such as Built to Scale DS serialize cameraPivot.rotation.y and need per-vertex
// projection instead of a final screen-space rotation.
func (s *SceneInst) SetCameraYaw(deg float64) {
	s.hasCamBasis = false
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		s.camYaw = 0
		return
	}
	s.camYaw = deg * math.Pi / 180
}

// SetCameraQuat sets a full 3D camera pose. q is a Unity-style local-to-world
// quaternion (x,y,z,w), with local +z as camera forward. It is used by mesh
// games whose authored CameraPos has pitch/roll as well as orbit yaw.
func (s *SceneInst) SetCameraQuat(x, y, z float64, q [4]float64) {
	q = normalizeQuat(q)
	s.cam, s.hasCam = [3]float64{x, y, z}, true
	s.camYaw = 0
	s.camRight = quatRotateVec(q, [3]float64{1, 0, 0})
	s.camUp = quatRotateVec(q, [3]float64{0, 1, 0})
	s.camForward = quatRotateVec(q, [3]float64{0, 0, 1})
	s.hasCamBasis = true
}

// SetPalette 设置映射材质（CellAnime_MappedInvert）的默认调色板（recolor 事件）。
func (s *SceneInst) SetPalette(p Palette) { s.palette = p }

// SetPaletteFor 按材质名设置调色板（marchingOrders 的 Tile/Pipe/Conveyor 等）。
func (s *SceneInst) SetPaletteFor(mat string, p Palette) {
	if s.palettes == nil {
		s.palettes = map[string]Palette{}
	}
	s.palettes[mat] = p
}

// SetPaletteOver sets a mapped-material palette for a single renderer node.
// Octopus Machine recolors each Octo-Pop through SpriteRenderer.material,
// so using only the shared material name would incorrectly recolor all three.
func (s *SceneInst) SetPaletteOver(path string, p Palette) {
	if i, ok := s.byPath[path]; ok {
		s.palOver[i] = p
	}
}

// ClearPaletteOver removes a per-node mapped-material override.
func (s *SceneInst) ClearPaletteOver(path string) {
	if i, ok := s.byPath[path]; ok {
		delete(s.palOver, i)
	}
}

// paletteOf 取节点应使用的调色板。
func (s *SceneInst) paletteOf(mat string) Palette {
	if p, ok := s.palettes[mat]; ok {
		return p
	}
	return s.palette
}

func (s *SceneInst) paletteForNode(i int) Palette {
	if p, ok := s.palOver[i]; ok {
		return p
	}
	return s.paletteOf(s.as.Rig.Nodes[i].Mat)
}

// camView 返回节点深度 z 处的视图变换（含相机平移与透视缩放）；ok=false 表示在相机背后。
func (s *SceneInst) camView(z float64) (Aff, bool) {
	if s.hasCamBasis {
		return s.camViewBasis(z)
	}
	if s.camYaw != 0 {
		return s.camViewYaw(z)
	}
	focal := CameraFocalDistance(s.camFOV)
	if !s.hasCam {
		if z == 0 {
			return Identity(), true
		}
		ps := focal / (focal + z)
		if ps <= 0 {
			return Identity(), false
		}
		return Scale(ps, ps), true
	}
	d := z - s.cam[2]
	if d <= 0 {
		return Identity(), false
	}
	ps := focal / d
	return Scale(ps, ps).Mul(Translate(-s.cam[0], -s.cam[1])), true
}

func (s *SceneInst) camViewYaw(z float64) (Aff, bool) {
	focal := CameraFocalDistance(s.camFOV)
	sn, cs := math.Sin(s.camYaw), math.Cos(s.camYaw)
	zr := cs * z
	d := focal + zr
	tx, ty := -sn*z, 0.0
	if s.hasCam {
		d = zr - s.cam[2]
		tx -= s.cam[0]
		ty -= s.cam[1]
	}
	if d <= 0 {
		return Identity(), false
	}
	ps := focal / d
	// This affine is only an approximation for SpriteRenderer paths at a fixed
	// node depth. MeshRenderer drawing bypasses it and projects every vertex, so
	// BuiltToScaleDS camera yaw does not flatten wide planes or rods.
	return Scale(ps, ps).Mul(Aff{A: cs, D: 1, Tx: tx, Ty: ty}), true
}

func (s *SceneInst) camViewBasis(z float64) (Aff, bool) {
	x0, y0, _, ok := s.projectPoint(0, 0, z)
	if !ok {
		return Identity(), false
	}
	x1, y1, _, ok := s.projectPoint(1, 0, z)
	if !ok {
		return Identity(), false
	}
	x2, y2, _, ok := s.projectPoint(0, 1, z)
	if !ok {
		return Identity(), false
	}
	// A fixed-z plane under a pitched perspective camera is technically
	// projective. SpriteRenderer paths are small billboards, so this local affine
	// keeps them usable while MeshRenderer paths use exact per-vertex projection.
	return Aff{A: x1 - x0, B: y1 - y0, C: x2 - x0, D: y2 - y0, Tx: x0, Ty: y0}, true
}

func (s *SceneInst) projectPoint(x, y, z float64) (float64, float64, float64, bool) {
	focal := CameraFocalDistance(s.camFOV)
	if s.hasCamBasis {
		rel := [3]float64{x - s.cam[0], y - s.cam[1], z - s.cam[2]}
		vx := dot3(rel, s.camRight)
		vy := dot3(rel, s.camUp)
		vz := dot3(rel, s.camForward)
		if vz <= 0 {
			return 0, 0, 0, false
		}
		ps := focal / vz
		return vx * ps, vy * ps, ps, true
	}
	if s.camYaw != 0 {
		sn, cs := math.Sin(s.camYaw), math.Cos(s.camYaw)
		x, z = cs*x-sn*z, sn*x+cs*z
	}
	if !s.hasCam {
		d := focal + z
		if d <= 0 {
			return 0, 0, 0, false
		}
		ps := focal / d
		return x * ps, y * ps, ps, true
	}
	d := z - s.cam[2]
	if d <= 0 {
		return 0, 0, 0, false
	}
	ps := focal / d
	return (x - s.cam[0]) * ps, (y - s.cam[1]) * ps, ps, true
}

func (s *SceneInst) cameraSortZ(x, y, z float64) float64 {
	if s.hasCamBasis {
		rel := [3]float64{x - s.cam[0], y - s.cam[1], z - s.cam[2]}
		return dot3(rel, s.camForward)
	}
	if s.camYaw == 0 {
		return z
	}
	sn, cs := math.Sin(s.camYaw), math.Cos(s.camYaw)
	return sn*x + cs*z
}

func normalizeQuat(q [4]float64) [4]float64 {
	n := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return [4]float64{0, 0, 0, 1}
	}
	return [4]float64{q[0] / n, q[1] / n, q[2] / n, q[3] / n}
}

func quatRotateVec(q [4]float64, v [3]float64) [3]float64 {
	cx := q[1]*v[2] - q[2]*v[1] + q[3]*v[0]
	cy := q[2]*v[0] - q[0]*v[2] + q[3]*v[1]
	cz := q[0]*v[1] - q[1]*v[0] + q[3]*v[2]
	return [3]float64{
		v[0] + 2*(q[1]*cz-q[2]*cy),
		v[1] + 2*(q[2]*cx-q[0]*cz),
		v[2] + 2*(q[0]*cy-q[1]*cx),
	}
}

func dot3(a, b [3]float64) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func NewScene(as *Assets) *SceneInst {
	s := &SceneInst{
		as:         as,
		byPath:     map[string]int{},
		state:      make([]sceneNodeState, len(as.Rig.Nodes)),
		worldZ:     make([]float64, len(as.Rig.Nodes)),
		world:      make([]Aff, len(as.Rig.Nodes)),
		actives:    make([]bool, len(as.Rig.Nodes)),
		players:    map[string]*scenePlayer{},
		machines:   map[string]*smachine{},
		spinOver:   map[int]float64{},
		activeOver: map[int]bool{},
		renderOver: map[int]bool{},
		mirrorOver: map[int]bool{},
		colorOver:  map[int][4]float64{},
		matOver:    map[int]materialState{},
		matFor:     map[string]materialState{},
		matForSkip: map[string][]string{},
		texFor:     map[string][2]float64{},
		palOver:    map[int]Palette{},
		posOver:    map[int][2]float64{},
		scaleOver:  map[int][2]float64{},
		sizeOver:   map[int][2]float64{},
		orderOver:  map[int]int{},
		zOver:      map[int]float64{},
		spriteOver: map[int]string{},
		palette:    DefaultPalette(),
	}
	for path, ctrlName := range as.Animators {
		if ctrl, ok := as.Controllers[ctrlName]; ok {
			params := map[string]bool{}
			for k, v := range ctrl.Params {
				params[k] = v
			}
			c := ctrl
			s.machines[path] = &smachine{ctrl: &c, params: params}
		}
	}
	for i, n := range as.Rig.Nodes {
		if _, dup := s.byPath[n.Path]; !dup { // 重名路径取首个（Unity 同语义）
			s.byPath[n.Path] = i
		}
	}
	// SortingGroup：每个节点的归属组 = 最近的挂组祖先（含自身），-1 = 无
	s.groupOf = make([]int, len(as.Rig.Nodes))
	for i, n := range as.Rig.Nodes {
		switch {
		case len(n.SortGroup) == 2:
			s.groupOf[i] = i
		case n.Parent >= 0:
			s.groupOf[i] = s.groupOf[n.Parent]
		default:
			s.groupOf[i] = -1
		}
	}
	type item struct{ idx, layer, order int }
	items := []item{}
	for i, n := range as.Rig.Nodes {
		if n.Sprite != "" || true { // 动画可能换上 sprite，全部纳入排序
			items = append(items, item{i, n.Layer, n.Order})
		}
	}
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].layer != items[b].layer {
			return items[a].layer < items[b].layer
		}
		return items[a].order < items[b].order
	})
	for _, it := range items {
		s.drawOrder = append(s.drawOrder, it.idx)
	}
	return s
}

// Play 在子树 rootPath 上从 startBeat 起播放剪辑（替换该子树原有播放器）。
// timeScale 同 HS DoScaledAnimationAsync：clip 每秒对应 1/timeScale 拍。
func (s *SceneInst) Play(rootPath, clip string, startBeat, timeScale float64) {
	s.PlayLayer(rootPath, rootPath, clip, startBeat, timeScale)
}

// PlayLayer 在同一个子树 rootPath 上用独立 key 播放剪辑。
// Unity 的一个 Animator 可同时通过多条不冲突曲线驱动同一根下的不同子树
// （Tunnel 背景 Near/Far 滚动就是这种数据形态）；常规 Play 仍按 rootPath
// 替换旧播放器，只有确实需要并行曲线时才传入额外 key。
func (s *SceneInst) PlayLayer(key, rootPath, clip string, startBeat, timeScale float64) {
	anim, resolvedClip, ok := resolveAnim(s.as, clip)
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	if _, exists := s.players[key]; !exists {
		s.playerOrder = append(s.playerOrder, key)
	}
	s.players[key] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: resolvedClip,
		startBeat: startBeat, timeScale: timeScale,
	}
}

// Current 返回子树当前播放的剪辑名。
func (s *SceneInst) Current(rootPath string) string {
	if p, ok := s.players[rootPath]; ok {
		return p.clipName
	}
	return ""
}

// PlayNormalized 以 DoNormalizedAnimation 语义播放：固定在归一化时间 t 采样
// （Unity 等价 Play(name, 0, t) + speed 0，cartGuy 的推车位移用它逐帧驱动）。
func (s *SceneInst) PlayNormalized(rootPath, clip string, t float64) {
	anim, resolvedClip, ok := resolveAnim(s.as, clip)
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	s.players[rootPath] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: resolvedClip,
		normalized: true, normT: math.Max(0, math.Min(1, t)),
	}
}

// PlayLayerNormalized is PlayNormalized for an independent overlay layer.
// Unity face poser controllers often use float parameters to hold a static
// frame while the body Animator keeps running on layer 0; using a separate key
// preserves that composition instead of replacing the base player.
func (s *SceneInst) PlayLayerNormalized(key, rootPath, clip string, t float64) {
	anim, resolvedClip, ok := resolveAnim(s.as, clip)
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	if _, exists := s.players[key]; !exists {
		s.playerOrder = append(s.playerOrder, key)
	}
	s.players[key] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: resolvedClip,
		normalized: true, normT: math.Max(0, math.Min(1, t)),
	}
}

// PlayFrozen 按状态名冻结在归一化时间 normT（DoScaledAnimationAsync(name, 0) 语义）。
func (s *SceneInst) PlayFrozen(rootPath, stateName string, normT float64) {
	m, ok := s.machines[rootPath]
	if !ok {
		return
	}
	st, ok := m.ctrl.States[stateName]
	if !ok || st.Clip == "" {
		return
	}
	anim, resolvedClip, ok := resolveAnim(s.as, st.Clip)
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	m.state, m.lastT = stateName, 0
	s.players[rootPath] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: resolvedClip,
		normalized: true, normT: normT,
	}
}

// ---------- AnimatorController 状态机 ----------

// PlayState 按状态名播放（DoScaledAnimationAsync 语义）：状态映射到剪辑，
// 剪辑结束后由 Sample 按 controller 转换自动切换状态。
// timeScale persists：转换到的新状态沿用（Unity animator.speed 同语义）。
func (s *SceneInst) PlayState(rootPath, stateName string, startBeat, timeScale float64) {
	m, ok := s.machines[rootPath]
	if !ok {
		s.Play(rootPath, stateName, startBeat, timeScale) // 无 controller：按剪辑名直接播
		return
	}
	st, ok := m.ctrl.States[stateName]
	if !ok {
		return
	}
	m.state, m.lastT = stateName, 0
	s.playMachineClip(rootPath, st, startBeat, timeScale)
}

// PlayStateLayer mirrors DoScaledAnimationAsync(name, ..., animLayer:n) for
// games that use one Animator's layers as independent, non-overlapping curve
// sets. The controller is used only to resolve stateName -> clip; state-machine
// transitions remain owned by PlayState on the base root player.
func (s *SceneInst) PlayStateLayer(key, rootPath, stateName string, startBeat, timeScale float64) {
	m, ok := s.machines[rootPath]
	if !ok {
		s.PlayLayer(key, rootPath, stateName, startBeat, timeScale)
		return
	}
	st, ok := m.ctrl.States[stateName]
	if !ok {
		return
	}
	if st.Clip == "" || st.Speed*timeScale == 0 {
		delete(s.players, key)
		return
	}
	s.PlayLayer(key, rootPath, st.Clip, startBeat, timeScale*st.Speed)
}

// PlayDefaultState 进入 controller 默认状态（OnGameSwitch 时机；
// Unity Animator 激活即按真实秒速播放默认态，故 timeScale 应传 secPerBeat）。
func (s *SceneInst) PlayDefaultState(rootPath string, startBeat, timeScale float64) {
	if m, ok := s.machines[rootPath]; ok {
		s.PlayState(rootPath, m.ctrl.Default, startBeat, timeScale)
	}
}

func (s *SceneInst) playMachineClip(rootPath string, st kmdata.CtrlState, startBeat, timeScale float64) {
	if st.Clip == "" || st.Speed*timeScale == 0 {
		delete(s.players, rootPath) // 无 motion / 速度 0：保持当前姿态（prefab 默认）
		return
	}
	s.Play(rootPath, st.Clip, startBeat, timeScale*st.Speed)
}

// SetBool 设置状态机 bool 参数（Animator.SetBool）。
func (s *SceneInst) SetBool(rootPath, param string, v bool) {
	if m, ok := s.machines[rootPath]; ok {
		m.params[param] = v
	}
}

// StateInfo 返回当前状态名与是否仍在播放（normalizedTime < 1，
// HS Util.IsPlayingAnimationNames 同语义），beat 为当前节拍。
func (s *SceneInst) StateInfo(rootPath string, beat float64) (string, bool) {
	m, ok := s.machines[rootPath]
	if !ok {
		return "", false
	}
	p := s.players[rootPath]
	if p == nil || p.anim == nil || p.timeScale <= 0 || p.anim.Duration <= 0 {
		return m.state, false
	}
	clipT := (beat - p.startBeat) * p.timeScale
	return m.state, clipT < p.anim.Duration
}

// stepMachines 推进状态机：剪辑过了退出时间（循环剪辑按完整 normalizedTime 计）
// 且条件满足时切换状态。条件在闸点后每帧评估（Unity hasExitTime+conditions 语义：
// 退出时间是最早可触发时刻，此后条件一旦为真即转换）。
func (s *SceneInst) stepMachines(beat float64) {
	for path, m := range s.machines {
		for iter := 0; iter < 8; iter++ { // 链式转换护栏
			p := s.players[path]
			if p == nil || p.normalized || m.state == "" || p.anim == nil ||
				p.timeScale <= 0 || p.anim.Duration <= 0 {
				break
			}
			st := m.ctrl.States[m.state]
			clipT := (beat - p.startBeat) * p.timeScale
			if clipT < 0 {
				break
			}
			D := p.anim.Duration
			var fired *kmdata.CtrlTransition
			var fireBeat float64
			for i := range st.Transitions {
				tr := &st.Transitions[i]
				gateT := D * tr.ExitTime
				if clipT < gateT {
					continue
				}
				if gateT == 0 && clipT == 0 && m.lastT == 0 {
					// Animator.Play should make the requested state visible for
					// the current evaluation. Without this guard, 0-exit states
					// such as Blue Bear's BiteL/BiteR transition away before
					// their first frame is ever sampled.
					continue
				}
				ok := true
				for _, c := range tr.Conds {
					v := m.params[c.Param]
					if (c.Mode == "if" && !v) || (c.Mode == "ifnot" && v) {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				// 恰在本帧跨过闸点 → 从闸点起播；早已过闸（条件后到）→ 从当前拍起播
				if m.lastT < gateT {
					fireBeat = p.startBeat + gateT/p.timeScale
				} else {
					fireBeat = beat
				}
				fired = tr
				break
			}
			if fired == nil {
				m.lastT = clipT
				break
			}
			dst, ok := m.ctrl.States[fired.Dst]
			if !ok {
				m.lastT = clipT
				break
			}
			// Duration（过渡混合）按立即切换处理：用到非零值的
			// BossCall→BossCallIdle 已验证源末帧与目标姿态逐曲线一致
			m.state, m.lastT = fired.Dst, 0
			s.playMachineClip(path, dst, fireBeat, p.timeScale/maxf(st.Speed, 1e-9))
		}
	}
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ---------- 模块驱动的覆盖 ----------

// SetSpinIdx 设置节点旋转叠加（弧度；transform.Rotate 的积分由模块自做）。
func (s *SceneInst) SetSpinIdx(idx int, rad float64) { s.spinOver[idx] = rad }

// SetSpinOver 设置节点局部旋转叠加（弧度）。一些 HS 脚本直接写
// transform.rotation，例如 Quiz Show 的秒表指针；按 path 暴露避免模块
// 依赖 scene 内部下标。
func (s *SceneInst) SetSpinOver(path string, rad float64) {
	if i, ok := s.byPath[path]; ok {
		s.spinOver[i] = rad
	}
}

// SetActive 覆盖节点 m_IsActive（GameObject.SetActive 语义，沿层级传播）。
func (s *SceneInst) SetActive(path string, active bool) {
	if i, ok := s.byPath[path]; ok {
		s.activeOver[i] = active
	}
}

// SetRenderOver 覆盖 SpriteRenderer.enabled。它只影响当前节点的 renderer，
// 不影响子物体 activeInHierarchy；Fan Club 的 faceposer 就依赖这个区别。
func (s *SceneInst) SetRenderOver(path string, on bool) {
	if i, ok := s.byPath[path]; ok {
		s.renderOver[i] = on
	}
}

// SetMirrorX 覆盖节点 localScale.x 的符号（transform.localScale = (-1,1,1) 语义）。
func (s *SceneInst) SetMirrorX(path string, mirror bool) {
	if i, ok := s.byPath[path]; ok {
		s.mirrorOver[i] = mirror
	}
}

// SetColorOver 覆盖节点 SpriteRenderer.color（代码直写 sr.color 语义）。
func (s *SceneInst) SetColorOver(path string, c [4]float64) {
	if i, ok := s.byPath[path]; ok {
		s.colorOver[i] = c
	}
}

// SetMaterialOver 覆盖 CellAnime 材质的 _Color/_AddColor。
// SpriteRenderer.color 仍由 SetColorOver/m_Color 曲线管理；Tunnel 的灯墙
// 是材质色而非 renderer 色，二者需要分开避免误染同节点其它动画属性。
func (s *SceneInst) SetMaterialOver(path string, matColor, add [4]float64) {
	if i, ok := s.byPath[path]; ok {
		s.matOver[i] = materialState{color: matColor, hasColor: true, add: add}
	}
}

// SetMamboMaterialFor 覆盖共享 MamboDoodle 材质的 _HueShift/_AddColor。
// WarioDeMambo.cs 直接改 mainMat/lightMat/floorLightMat；这些是材质实例，
// 不是单个 SpriteRenderer。按材质名覆盖可表达共享材质；如果导出时多个
// Unity 材质实例折叠成同名材质，应使用 SetMamboMaterialForExcept 保留边界。
func (s *SceneInst) SetMamboMaterialFor(mat string, hueShift float64, add [4]float64) {
	s.SetMamboMaterialForExcept(mat, hueShift, add)
}

// SetMamboMaterialForAt is SetMamboMaterialFor with the runtime time needed by
// MamboDoodle's DoodleTextureOffset shader function.
func (s *SceneInst) SetMamboMaterialForAt(mat string, hueShift float64, add [4]float64, time float64) {
	s.SetMamboMaterialForExceptAt(mat, hueShift, add, time)
}

// SetMamboMaterialForExcept 覆盖共享 MamboDoodle 材质，并排除若干场景子树。
// Wario de Mambo 的 prefab 里多处 renderer 导出为同一个材质名，但 Unity
// 脚本实际操作的是序列化字段上的材质实例；排除子树用于保留这种实例边界。
func (s *SceneInst) SetMamboMaterialForExcept(mat string, hueShift float64, add [4]float64, excludeRoots ...string) {
	s.setMamboMaterialForExcept(mat, hueShift, add, DoodleParams{}, excludeRoots...)
}

// SetMamboMaterialForExceptAt 覆盖共享 MamboDoodle 材质，同时启用从
// materials.json 导出的 DoodleTextureOffset 参数。
func (s *SceneInst) SetMamboMaterialForExceptAt(mat string, hueShift float64, add [4]float64, time float64, excludeRoots ...string) {
	s.setMamboMaterialForExcept(mat, hueShift, add, s.mamboDoodleParams(mat, time), excludeRoots...)
}

func (s *SceneInst) setMamboMaterialForExcept(mat string, hueShift float64, add [4]float64, doodle DoodleParams, excludeRoots ...string) {
	if mat == "" {
		return
	}
	s.matFor[mat] = materialState{
		color:     [4]float64{1, 1, 1, 1},
		hasColor:  true,
		add:       add,
		hueShift:  hueShift,
		linearAdd: true,
		doodle:    doodle,
	}
	if len(excludeRoots) == 0 {
		delete(s.matForSkip, mat)
		return
	}
	s.matForSkip[mat] = append([]string(nil), excludeRoots...)
}

func (s *SceneInst) mamboDoodleParams(mat string, time float64) DoodleParams {
	if s == nil || s.as == nil || s.as.Materials == nil {
		return DoodleParams{}
	}
	m, ok := s.as.Materials[mat]
	if !ok {
		return DoodleParams{}
	}
	max, hasMax := m.Colors["_DoodleMaxOffset"]
	if !hasMax {
		return DoodleParams{}
	}
	frameTime := m.Floats["_DoodleFrameTime"]
	frameCount := m.Floats["_DoodleFrameCount"]
	if frameTime <= 0 || frameCount <= 0 {
		return DoodleParams{}
	}
	noise := m.Colors["_DoodleNoiseScale"]
	return DoodleParams{
		Enabled:    true,
		Time:       time,
		MaxOffset:  [2]float64{max[0], max[1]},
		FrameTime:  frameTime,
		FrameCount: frameCount,
		NoiseScale: [2]float64{noise[0], noise[1]},
	}
}

func (s *SceneInst) materialForApplies(mat, path string) bool {
	for _, root := range s.matForSkip[mat] {
		if root != "" && (path == root || strings.HasPrefix(path, root+"/")) {
			return false
		}
	}
	return true
}

// SetMaterialColorFor 覆盖共享材质的 _Color。Mesh-only games such as
// Built to Scale DS recolor material instances directly rather than individual
// renderers, so the runtime must key this by Unity material name.
func (s *SceneInst) SetMaterialColorFor(mat string, color [4]float64) {
	s.SetMaterialColorParamFor(mat, "_Color", color)
}

// SetMaterialColorParamFor 覆盖共享材质上的命名颜色参数。多数 mesh 只使用
// _Color 作为整体 tint；Airboarder floor 等自定义 shader 会额外写
// _BlueColor/_RedColor，先保留 Unity 的参数边界，避免把 shader 色槽折叠成
// 一个近似 tint。
func (s *SceneInst) SetMaterialColorParamFor(mat, param string, color [4]float64) {
	if mat == "" {
		return
	}
	if param == "" {
		param = "_Color"
	}
	st := s.matFor[mat]
	if st.colors == nil {
		st.colors = map[string][4]float64{}
	}
	st.colors[param] = color
	if param == "_Color" {
		st.color = color
		st.hasColor = true
	}
	s.matFor[mat] = st
}

// MaterialColorParamForTest exposes shared material color slots for audits that
// compare script-driven Unity material properties against the native runtime.
func (s *SceneInst) MaterialColorParamForTest(mat, param string) ([4]float64, bool) {
	if param == "" {
		param = "_Color"
	}
	st, ok := s.matFor[mat]
	if !ok {
		return [4]float64{}, false
	}
	if param == "_Color" && st.hasColor {
		return st.color, true
	}
	c, ok := st.colors[param]
	return c, ok
}

// SetMaterialTextureOffsetFor 覆盖共享材质的 _MainTex offset。
func (s *SceneInst) SetMaterialTextureOffsetFor(mat string, offset [2]float64) {
	if mat == "" {
		return
	}
	s.texFor[mat] = offset
}

// SetPosOver 覆盖节点 localPosition（伪相机 gameTrans 平移等）。
func (s *SceneInst) SetPosOver(path string, x, y float64) {
	if i, ok := s.byPath[path]; ok {
		s.posOver[i] = [2]float64{x, y}
	}
}

// SetScaleOver 覆盖节点 localScale。Ringside 的 pose 命中会在脚本里临时
// 放大整名选手，这类一帧级变换不是 AnimationClip 曲线的一部分。
func (s *SceneInst) SetScaleOver(path string, sx, sy float64) {
	if i, ok := s.byPath[path]; ok {
		s.scaleOver[i] = [2]float64{sx, sy}
	}
}

// ClearScaleOver 撤销 localScale 覆盖。
func (s *SceneInst) ClearScaleOver(path string) {
	if i, ok := s.byPath[path]; ok {
		delete(s.scaleOver, i)
	}
}

// SetSizeOver 覆盖 SpriteRenderer.size（Unity 代码直写 tiled/sliced 尺寸）。
func (s *SceneInst) SetSizeOver(path string, w, h float64) {
	if i, ok := s.byPath[path]; ok {
		s.sizeOver[i] = [2]float64{w, h}
	}
}

// SetOrderOver 覆盖 SpriteRenderer.sortingOrder。少数 HS 脚本直接改
// Renderer.sortingOrder（例如 Packing Pests 盒子前板压住/让出物体），这
// 和动画曲线驱动的 m_SortingOrder 一样必须进入统一排序。
func (s *SceneInst) SetOrderOver(path string, order int) {
	if i, ok := s.byPath[path]; ok {
		s.orderOver[i] = order
	}
}

// ClearOrderOver 撤销 sortingOrder 覆盖，恢复 prefab/动画曲线给出的顺序。
func (s *SceneInst) ClearOrderOver(path string) {
	if i, ok := s.byPath[path]; ok {
		delete(s.orderOver, i)
	}
}

// ClearPosOver 撤销 localPosition 覆盖。
func (s *SceneInst) ClearPosOver(path string) {
	if i, ok := s.byPath[path]; ok {
		delete(s.posOver, i)
	}
}

// SetSpriteOver 覆盖节点的切片（SpriteRenderer.sprite 直写语义；
// 覆盖在剪辑采样之后生效，空串恢复 prefab/剪辑值）。
func (s *SceneInst) SetSpriteOver(path, sprite string) {
	if i, ok := s.byPath[path]; ok {
		if sprite == "" {
			delete(s.spriteOver, i)
		} else {
			s.spriteOver[i] = sprite
		}
	}
}

// SetZOver 覆盖节点的深度 z（transform.position.z 直写语义；
// 仅根节点向（worldZ 不再叠加父链））。
func (s *SceneInst) SetZOver(path string, z float64) {
	if i, ok := s.byPath[path]; ok {
		s.zOver[i] = z
	}
}

// ClearZOver 撤销世界 z 覆盖。
func (s *SceneInst) ClearZOver(path string) {
	if i, ok := s.byPath[path]; ok {
		delete(s.zOver, i)
	}
}

// Index 返回 path 的节点下标（重名 path 取首个，Unity 同语义）。
func (s *SceneInst) Index(path string) (int, bool) {
	i, ok := s.byPath[path]
	return i, ok
}

// NodeSprite 返回节点当前的切片名与翻转（需先 Sample；
// lockstep 人群平铺等"主驱动、从复制切片"的 master/slave 模式用）。
func (s *SceneInst) NodeSprite(path string) (sprite string, flipX, flipY bool) {
	if i, ok := s.byPath[path]; ok {
		return s.state[i].sprite, s.state[i].flipX, s.state[i].flipY
	}
	return "", false, false
}

// ExtraSprite 是模块注入的动态绘制项（模板实例/手写粒子），
// 与场景节点按同一 (layer, order, z) 规则统一排序。
type ExtraSprite struct {
	Sprite       string
	World        Aff // 单位空间变换（z 的透视缩放由 Draw 统一施加）
	Z            float64
	Layer, Order int
	HasGroup     bool    // true 时先按 Group* 作为 Unity SortingGroup 单元排序
	GroupKey     int     // 同一动态实例内的 renderer 必须共享 key 才会按组内 order 排
	GroupLayer   int     // SortingGroup.sortingLayer
	GroupOrder   int     // SortingGroup.sortingOrder；不要混入 renderer order
	GroupZ       float64 // SortingGroup 所在深度
	FlipX, FlipY bool
	Tint         [4]float64 // 零值视为白色
	MatColor     [4]float64 // material._Color for queued sprites
	Add          [4]float64 // material._AddColor for mapped queued sprites
	Blend        [4]float64 // material._BlendColor for queued sprites
	HueShift     float64    // material._HueShift for MamboDoodle queued sprites
	LinearAdd    bool       // true when Add is MamboDoodle linear add
	Doodle       DoodleParams
	OutlineWidth float64 // TMP material._OutlineWidth for queued text sprites
	Threshold    float64 // material._Threshold for mapped queued sprites
	HasThreshold bool
	Progress     float64 // material._Progress for mapped queued sprites
	HasProgress  bool
	Mapped       bool   // 调色板映射材质（SceneInst.SetPalette）
	Mat          string // 映射材质名（按名调色板）
	HasPalette   bool   // true 时 Palette 为实例级 mapped 材质参数
	Palette      Palette
	Mask         bool // SpriteMask：本体不绘制，只为 MaskIn=1 的项目提供可见区域
	MaskIn       int  // SpriteRenderer m_MaskInteraction（1=VisibleInsideMask）
}

// Queue 注入一帧动态绘制项（Draw 后清空，每帧重新注入）。
func (s *SceneInst) Queue(e ExtraSprite) { s.queued = append(s.queued, e) }

// QueuedSpritesForTest returns a snapshot of pending dynamic sprites for
// cross-package audit tests. Production code should queue and draw through
// SceneInst instead of depending on this transient per-frame buffer.
func (s *SceneInst) QueuedSpritesForTest() []ExtraSprite {
	return append([]ExtraSprite(nil), s.queued...)
}

// MaterialStateForTest is the small part of sceneNodeState needed by
// cross-package audits that verify Unity material-instance boundaries.
type MaterialStateForTest struct {
	MatHueShift  float64
	MatLinearAdd bool
	MatDoodle    DoodleParams
}

// NodeMaterialStateForTest returns the sampled material override state for a
// scene node. Callers must Sample first, matching NodeWorld/NodeSprite.
func (s *SceneInst) NodeMaterialStateForTest(path string) (MaterialStateForTest, bool) {
	i, ok := s.byPath[path]
	if !ok {
		return MaterialStateForTest{}, false
	}
	st := s.state[i]
	return MaterialStateForTest{
		MatHueShift:  st.matHueShift,
		MatLinearAdd: st.matLinearAdd,
		MatDoodle:    st.matDoodle,
	}, true
}

// NodePaletteForTest returns the mapped-material palette a renderer would use.
// It lets port audits verify Unity per-renderer material instances without
// depending on Draw's shader path.
func (s *SceneInst) NodePaletteForTest(path string) (Palette, bool) {
	i, ok := s.byPath[path]
	if !ok {
		return Palette{}, false
	}
	return s.paletteForNode(i), true
}

// ExtraMesh 是模板实例注入的 MeshRenderer 绘制项。
// Unity 的 Instantiate 会复制 MeshRenderer；场景里的原 prefab 往往保持 inactive，
// 所以动态实例不能复用 scene node 的 active/render 状态，只能携带采样后的 world/tint。
type ExtraMesh struct {
	Binding      int
	World        Aff
	Z            float64
	Layer, Order int
	HasGroup     bool
	GroupKey     int
	GroupLayer   int
	GroupOrder   int
	GroupZ       float64
	Tint         [4]float64
}

// QueueMesh 注入一帧动态 MeshRenderer 绘制项（Draw 后清空，每帧重新注入）。
func (s *SceneInst) QueueMesh(e ExtraMesh) { s.queuedMeshes = append(s.queuedMeshes, e) }

// Sample 按歌曲节拍采样所有播放器并更新世界变换。
func (s *SceneInst) Sample(beat float64) {
	s.stepMachines(beat)
	for i, n := range s.as.Rig.Nodes {
		c := n.Color
		if c == [4]float64{} {
			c = [4]float64{1, 1, 1, 1}
		}
		s.state[i] = sceneNodeState{
			pos: n.Pos, rot: n.RotZ, scale: n.Scale,
			sprite: n.Sprite, flipX: n.FlipX, flipY: n.FlipY,
			active: !n.Inactive, renderOn: !n.Hidden,
			color: c, matColor: [4]float64{1, 1, 1, 1}, matAlpha: 1, matOpacity: 1, size: n.Size, order: n.Order,
		}
	}
	for i, v := range s.activeOver {
		s.state[i].active = v
	}
	for i, v := range s.renderOver {
		s.state[i].renderOn = v
	}
	for i, v := range s.colorOver {
		s.state[i].color = v
	}
	for i, v := range s.matOver {
		if v.hasColor {
			s.state[i].matColor = v.color
			s.state[i].hasMatColor = true
		}
		s.state[i].matAdd = v.add
		s.state[i].matHueShift = v.hueShift
		s.state[i].matLinearAdd = v.linearAdd
		s.state[i].matDoodle = v.doodle
	}
	for i, v := range s.posOver {
		s.state[i].pos = v
	}
	for i, v := range s.scaleOver {
		s.state[i].scale = v
	}
	for _, key := range s.playerOrder {
		p := s.players[key]
		if p == nil {
			continue
		}
		var clipT float64
		if p.normalized {
			clipT = p.normT * p.anim.Duration
		} else {
			clipT = (beat - p.startBeat) * p.timeScale
			if clipT < 0 {
				clipT = 0
			}
			if p.anim.Loop && p.anim.Duration > 0 {
				clipT = math.Mod(clipT, p.anim.Duration)
			} else if clipT > p.anim.Duration {
				clipT = p.anim.Duration // 非循环：保持末帧
			}
		}
		s.applyClip(p, clipT)
	}
	for i, sp := range s.spriteOver {
		s.state[i].sprite = sp
	}
	for i, n := range s.as.Rig.Nodes {
		if v, ok := s.matFor[n.Mat]; ok && s.materialForApplies(n.Mat, n.Path) {
			if v.hasColor {
				s.state[i].matColor = v.color
				s.state[i].hasMatColor = true
			}
			s.state[i].matAdd = v.add
			s.state[i].matHueShift = v.hueShift
			s.state[i].matLinearAdd = v.linearAdd
			s.state[i].matDoodle = v.doodle
		}
	}
	for i, sz := range s.sizeOver {
		s.state[i].size = sz
	}
	for i, o := range s.orderOver {
		s.state[i].order = o
	}
	for i, rad := range s.spinOver {
		s.state[i].rot += rad
	}
	for i, m := range s.mirrorOver {
		sx := s.state[i].scale[0]
		if (m && sx > 0) || (!m && sx < 0) {
			s.state[i].scale[0] = -sx
		}
	}
	for i, n := range s.as.Rig.Nodes {
		st := &s.state[i]
		local := TRS(st.pos[0], st.pos[1], st.rot, st.scale[0], st.scale[1])
		if n.Parent < 0 {
			s.world[i] = local
			s.worldZ[i] = n.PosZ
			s.actives[i] = st.active
		} else {
			s.world[i] = s.world[n.Parent].Mul(local)
			s.worldZ[i] = s.worldZ[n.Parent] + n.PosZ
			s.actives[i] = st.active && s.actives[n.Parent]
		}
		if z, ok := s.zOver[i]; ok {
			s.worldZ[i] = z
		}
	}
}

// NodeWorld 返回节点当前的世界变换（需先 Sample）。
func (s *SceneInst) NodeWorld(path string) (Aff, bool) {
	if i, ok := s.byPath[path]; ok {
		return s.world[i], true
	}
	return Identity(), false
}

// NodeZForTest returns the sampled world z for a node. It is intentionally only
// useful to cross-package audits that verify camera/sticky-depth semantics.
func (s *SceneInst) NodeZForTest(path string) (float64, bool) {
	if i, ok := s.byPath[path]; ok {
		return s.worldZ[i], true
	}
	return 0, false
}

func (s *SceneInst) node(p *scenePlayer, curvePath string) (int, bool) {
	full := p.rootPath
	if curvePath != "" {
		if full == "" {
			full = curvePath
		} else {
			full = full + "/" + curvePath
		}
	}
	if i, ok := s.byPath[full]; ok {
		return i, true
	}
	if strings.Contains(full, "/Upper/Head") {
		// Some nested Unity prefabs keep parent clips authored against Upper/Head,
		// while extraction inserts a child Animator wrapper (HeadAnim/Upper/Head).
		// Resolve that serialized shape here so the original body clips still drive
		// the head sprites instead of silently dropping those curves.
		alt := strings.Replace(full, "/Upper/Head", "/Upper/HeadAnim/Upper/Head", 1)
		if i, ok := s.byPath[alt]; ok {
			return i, true
		}
	}
	if strings.Contains(full, "/Body/Head/Cork") {
		// Octopus Machine's clips were authored before Cork moved under Mouth.
		// Keep the serialized clip path compatible so Prepare/Pop still animate
		// the cork instead of dropping those curves.
		alt := strings.Replace(full, "/Body/Head/Cork", "/Body/Head/Mouth/Cork", 1)
		if i, ok := s.byPath[alt]; ok {
			return i, true
		}
	}
	if curvePath == "CorkString" && p.rootPath != "" {
		// The same Octopus prefab keeps a bare CorkString binding in Pop.anim,
		// but the scene hierarchy stores it under Body/Head.
		alt := p.rootPath + "/Body/Head/CorkString"
		if i, ok := s.byPath[alt]; ok {
			return i, true
		}
	}
	return 0, false
}

func (s *SceneInst) applyClip(p *scenePlayer, at float64) {
	a := p.anim
	for path, c := range a.Pos {
		if i, ok := s.node(p, path); ok {
			if len(c.X) > 0 {
				s.state[i].pos[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				s.state[i].pos[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Euler {
		if i, ok := s.node(p, path); ok && len(keys) > 0 {
			s.state[i].rot = evalKeys(keys, at) * math.Pi / 180
		}
	}
	for path, c := range a.Scale {
		if i, ok := s.node(p, path); ok {
			if len(c.X) > 0 {
				s.state[i].scale[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				s.state[i].scale[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Sprites {
		if i, ok := s.node(p, path); ok {
			if name, ok := sampleSwap(keys, at); ok {
				s.state[i].sprite = name
			}
		}
	}
	for path, attrs := range a.Floats {
		i, ok := s.node(p, path)
		if !ok {
			continue
		}
		for attr, keys := range attrs {
			if len(keys) == 0 {
				continue
			}
			v := evalKeys(keys, at)
			switch {
			case attr == "m_FlipX":
				s.state[i].flipX = v > 0.5
			case attr == "m_FlipY":
				s.state[i].flipY = v > 0.5
			case attr == "m_Size.x":
				s.state[i].size[0] = v
			case attr == "m_Size.y":
				s.state[i].size[1] = v
			case attr == "m_SortingOrder":
				s.state[i].order = int(v)
			case attr == "m_IsActive":
				s.state[i].active = v > 0.5
			case attr == "m_Enabled":
				s.state[i].renderOn = v > 0.5
			case attr == "m_AnchoredPosition.x":
				// World-space Canvas children are serialized as RectTransforms, but
				// HS uses them as ordinary local offsets (Charging Chicken water UV
				// scroll). In the Go scene tree they must drive transform position.
				s.state[i].pos[0] = v
			case attr == "m_AnchoredPosition.y":
				s.state[i].pos[1] = v
			case strings.HasPrefix(attr, "m_Color."), strings.HasPrefix(attr, "m_fontColor."):
				ch := strings.TrimPrefix(attr, "m_Color.")
				ch = strings.TrimPrefix(ch, "m_fontColor.")
				switch ch {
				case "r":
					s.state[i].color[0] = v
				case "g":
					s.state[i].color[1] = v
				case "b":
					s.state[i].color[2] = v
				case "a":
					s.state[i].color[3] = v
				}
			case strings.HasPrefix(attr, "material._AddColor."):
				ch := strings.TrimPrefix(attr, "material._AddColor.")
				switch ch {
				case "r":
					s.state[i].matAdd[0] = v
				case "g":
					s.state[i].matAdd[1] = v
				case "b":
					s.state[i].matAdd[2] = v
				case "a":
					s.state[i].matAdd[3] = v
				}
			case strings.HasPrefix(attr, "material._Color."):
				s.state[i].hasMatColor = true
				ch := strings.TrimPrefix(attr, "material._Color.")
				switch ch {
				case "r":
					s.state[i].matColor[0] = v
				case "g":
					s.state[i].matColor[1] = v
				case "b":
					s.state[i].matColor[2] = v
				case "a":
					s.state[i].matColor[3] = v
				}
			case attr == "material._Alpha":
				s.state[i].matAlpha = v
			case attr == "material._Opacity":
				s.state[i].matOpacity = v
			case attr == "material._Progress":
				s.state[i].matProgress = v
				s.state[i].hasMatProgress = true
			case attr == "material._HueShift":
				s.state[i].matHueShift = v
			case strings.HasPrefix(attr, "material._BlendColor."):
				ch := strings.TrimPrefix(attr, "material._BlendColor.")
				switch ch {
				case "r":
					s.state[i].matBlend[0] = v
				case "g":
					s.state[i].matBlend[1] = v
				case "b":
					s.state[i].matBlend[2] = v
				case "a":
					s.state[i].matBlend[3] = v
				}
			case attr == "material._OutlineWidth":
				s.state[i].outlineWidth = v
			case attr == "material._Threshold":
				s.state[i].matThreshold = v
				s.state[i].hasMatThreshold = true
			case strings.HasPrefix(attr, "material._ColorAlpha."):
				if !s.state[i].hasPalette {
					s.state[i].palette = s.paletteForNode(i)
					s.state[i].hasPalette = true
				}
				setPaletteChannel(&s.state[i].palette.Alpha, strings.TrimPrefix(attr, "material._ColorAlpha."), v)
			case strings.HasPrefix(attr, "material._ColorBravo."):
				if !s.state[i].hasPalette {
					s.state[i].palette = s.paletteForNode(i)
					s.state[i].hasPalette = true
				}
				setPaletteChannel(&s.state[i].palette.Fill, strings.TrimPrefix(attr, "material._ColorBravo."), v)
			case strings.HasPrefix(attr, "material._ColorDelta."):
				if !s.state[i].hasPalette {
					s.state[i].palette = s.paletteForNode(i)
					s.state[i].hasPalette = true
				}
				setPaletteChannel(&s.state[i].palette.Outline, strings.TrimPrefix(attr, "material._ColorDelta."), v)
			}
		}
	}
}

func setPaletteChannel(c *[4]float64, ch string, v float64) {
	switch ch {
	case "r":
		c[0] = v
	case "g":
		c[1] = v
	case "b":
		c[2] = v
	case "a":
		c[3] = v
	}
}

// CamDist 是 GameCamera 默认相机距离（位置 (0,0,-10)，FOV 53.15°，
// 在 z=0 平面恰好等价于半高 5 的正交视野）。
const CamDist = 10.0

const cameraHalfHeight = 5.0

// CameraFocalDistance converts a Unity vertical FOV into the focal distance used
// by the 2D projection shim. The default FOV intentionally returns CamDist so
// older games keep their exact projection when they do not serialize a FOV.
func CameraFocalDistance(fovDeg float64) float64 {
	if fovDeg <= 0 || math.IsNaN(fovDeg) || math.IsInf(fovDeg, 0) || fovDeg >= 179 {
		return CamDist
	}
	tan := math.Tan(fovDeg * math.Pi / 360)
	if tan <= 0 {
		return CamDist
	}
	return cameraHalfHeight / tan
}

// Draw 按 (sortingLayer, sortingOrder, 深度, DFS) 顺序绘制（需先 Sample）。
// sortingOrder 可能被动画驱动（m_SortingOrder 曲线），故每帧重排；
// 节点深度 z 经透视投影缩放（默认 s = CamDist/(CamDist+z)），复刻原版透视相机。
func (s *SceneInst) Draw(dst *ebiten.Image, proj Aff) {
	type item struct {
		idx, layer, order int
		z                 float64
		gIdx              int // 排序单元（SortingGroup 根或自身）
		gLayer, gOrder    int
		gZ                float64
		extra             int // ≥0：s.queued 下标（动态 SpriteRenderer 绘制项）
		extraMesh         int // ≥0：s.queuedMeshes 下标（动态 MeshRenderer 绘制项）
		mesh              int // ≥0：as.Meshes.Bindings 下标（scene MeshRenderer）
	}
	type maskItem struct {
		idx, extra int // extra >= 0 表示 s.queued，下标否则为 scene 节点
	}
	items := make([]item, 0, len(s.state)+len(s.queued)+len(s.queuedMeshes))
	// 活动的 SpriteMask（本体不绘制，为 MaskIn=1 的渲染器提供可见区域）
	var masks []maskItem
	for i := range s.state {
		if s.as.Rig.Nodes[i].Mask && s.actives[i] && s.state[i].renderOn && s.state[i].sprite != "" {
			masks = append(masks, maskItem{idx: i, extra: -1})
		}
	}
	for i := range s.state {
		st := &s.state[i]
		if !s.actives[i] || !st.renderOn {
			continue
		}
		if st.sprite == "" || st.color[3]*st.matAlpha*st.matOpacity <= 0 {
			continue
		}
		if s.as.Rig.Nodes[i].Mask {
			continue
		}
		it := item{idx: i, layer: s.as.Rig.Nodes[i].Layer, order: st.order, z: s.cameraSortZ(s.world[i].Tx, s.world[i].Ty, s.worldZ[i]), extra: -1, extraMesh: -1, mesh: -1}
		if g := s.groupOf[i]; g >= 0 {
			sg := s.as.Rig.Nodes[g].SortGroup
			it.gIdx, it.gLayer, it.gOrder, it.gZ = g, sg[0], sg[1], s.cameraSortZ(s.world[g].Tx, s.world[g].Ty, s.worldZ[g])
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = i, it.layer, it.order, it.z
		}
		items = append(items, it)
	}
	for mi, b := range s.as.Meshes.Bindings {
		i, _, ok := s.meshDrawable(mi)
		if !ok {
			continue
		}
		order := b.Order
		if s.state[i].order != s.as.Rig.Nodes[i].Order {
			order = s.state[i].order
		}
		it := item{idx: i, layer: b.Layer, order: order, z: s.cameraSortZ(s.world[i].Tx, s.world[i].Ty, s.worldZ[i]), extra: -1, extraMesh: -1, mesh: mi}
		if g := s.groupOf[i]; g >= 0 {
			sg := s.as.Rig.Nodes[g].SortGroup
			it.gIdx, it.gLayer, it.gOrder, it.gZ = g, sg[0], sg[1], s.cameraSortZ(s.world[g].Tx, s.world[g].Ty, s.worldZ[g])
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = i, it.layer, it.order, it.z
		}
		items = append(items, it)
	}
	for qi := range s.queued {
		q := &s.queued[qi]
		if q.Mask {
			if q.Sprite != "" {
				masks = append(masks, maskItem{extra: qi})
			}
			continue
		}
		if q.Sprite == "" {
			continue
		}
		if q.Tint != [4]float64{} && q.Tint[3] <= 0 {
			continue
		}
		it := item{idx: len(s.state) + qi, layer: q.Layer, order: q.Order, z: s.cameraSortZ(q.World.Tx, q.World.Ty, q.Z), extra: qi, extraMesh: -1, mesh: -1}
		if q.HasGroup {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = q.GroupKey, q.GroupLayer, q.GroupOrder, s.cameraSortZ(q.World.Tx, q.World.Ty, q.GroupZ)
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = it.idx, q.Layer, q.Order, it.z
		}
		items = append(items, it)
	}
	for qi := range s.queuedMeshes {
		q := &s.queuedMeshes[qi]
		if !s.meshRenderable(q.Binding) || q.Tint[3] <= 0 {
			continue
		}
		it := item{idx: len(s.state) + len(s.queued) + qi, layer: q.Layer, order: q.Order, z: s.cameraSortZ(q.World.Tx, q.World.Ty, q.Z), extra: -1, extraMesh: qi, mesh: -1}
		if q.HasGroup {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = q.GroupKey, q.GroupLayer, q.GroupOrder, s.cameraSortZ(q.World.Tx, q.World.Ty, q.GroupZ)
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = it.idx, q.Layer, q.Order, it.z
		}
		items = append(items, it)
	}
	sort.SliceStable(items, func(a, b int) bool {
		x, y := &items[a], &items[b]
		// 组级（Unity SortingGroup：子树作为单一单元参与全局排序）
		if x.gLayer != y.gLayer {
			return x.gLayer < y.gLayer
		}
		if x.gOrder != y.gOrder {
			return x.gOrder < y.gOrder
		}
		if x.gZ != y.gZ {
			return x.gZ > y.gZ // 远者先画
		}
		if x.gIdx != y.gIdx {
			return x.gIdx < y.gIdx
		}
		// 组内
		if x.layer != y.layer {
			return x.layer < y.layer
		}
		if x.order != y.order {
			return x.order < y.order
		}
		if x.z != y.z {
			return x.z > y.z
		}
		return x.idx < y.idx
	})
	ensureScratch := func() {
		w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
		if s.scratch == nil || s.scratch.Bounds().Dx() != w || s.scratch.Bounds().Dy() != h {
			s.scratch = ebiten.NewImage(w, h)
			s.maskBuf = ebiten.NewImage(w, h)
		}
	}
	drawQueued := func(target *ebiten.Image, q *ExtraSprite, qo SpriteOpts, view Aff) {
		if q.Mapped {
			pal := s.paletteOf(q.Mat)
			if q.HasPalette {
				pal = q.Palette
			}
			if q.HasThreshold {
				pal.Threshold = q.Threshold
			}
			if q.HasProgress {
				pal.Progress = q.Progress
				pal.UseProgress = true
			}
			s.as.DrawSpriteMapped(target, q.Sprite, view.Mul(q.World), proj, qo, pal)
		} else {
			s.as.DrawSpriteOpts(target, q.Sprite, view.Mul(q.World), proj, qo)
		}
	}
	drawMasks := func() {
		s.maskBuf.Clear()
		for _, mi := range masks {
			if mi.extra >= 0 {
				q := &s.queued[mi.extra]
				view, ok := s.camView(q.Z)
				if !ok {
					continue
				}
				// SpriteMask ignores SpriteRenderer.color; draw the mask silhouette
				// opaque even when the prefab renderer color is transparent.
				s.as.DrawSpriteOpts(s.maskBuf, q.Sprite, view.Mul(q.World), proj,
					SpriteOpts{FlipX: q.FlipX, FlipY: q.FlipY})
				continue
			}
			mview, ok := s.camView(s.worldZ[mi.idx])
			if !ok {
				continue
			}
			ms := &s.state[mi.idx]
			s.as.DrawSpriteOpts(s.maskBuf, ms.sprite, mview.Mul(s.world[mi.idx]), proj,
				SpriteOpts{FlipX: ms.flipX, FlipY: ms.flipY})
		}
	}
	for _, it := range items {
		if it.extra >= 0 {
			q := &s.queued[it.extra]
			view, ok := s.camView(q.Z)
			if !ok {
				continue
			}
			qo := SpriteOpts{
				FlipX: q.FlipX, FlipY: q.FlipY, Tint: q.Tint,
				MatColor: q.MatColor, Add: q.Add, Blend: q.Blend,
				HueShift: q.HueShift, LinearAdd: q.LinearAdd,
				Doodle:       q.Doodle,
				OutlineWidth: q.OutlineWidth,
			}
			if q.MaskIn == 1 {
				if len(masks) == 0 {
					continue
				}
				ensureScratch()
				s.scratch.Clear()
				drawQueued(s.scratch, q, qo, view)
				drawMasks()
				mop := &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn}
				s.scratch.DrawImage(s.maskBuf, mop)
				dst.DrawImage(s.scratch, nil)
				continue
			}
			drawQueued(dst, q, qo, view)
			continue
		}
		if it.extraMesh >= 0 {
			q := &s.queuedMeshes[it.extraMesh]
			if s.hasCamBasis || s.camYaw != 0 {
				s.drawMeshBindingProjected(dst, q.Binding, q.World, q.Z, proj, q.Tint)
				continue
			}
			view, ok := s.camView(q.Z)
			if !ok {
				continue
			}
			s.drawMeshBindingTinted(dst, q.Binding, view.Mul(q.World), proj, q.Tint)
			continue
		}
		if it.mesh >= 0 {
			i := it.idx
			if s.hasCamBasis || s.camYaw != 0 {
				s.drawMeshBindingProjected(dst, it.mesh, s.world[i], s.worldZ[i], proj, s.meshTint(i, &s.as.Meshes.Bindings[it.mesh]))
				continue
			}
			view, ok := s.camView(s.worldZ[i])
			if !ok {
				continue
			}
			s.drawMeshBinding(dst, it.mesh, i, view.Mul(s.world[i]), proj)
			continue
		}
		i := it.idx
		st := &s.state[i]
		tint := st.color
		tint[3] *= st.matAlpha * st.matOpacity
		opts := SpriteOpts{
			FlipX: st.flipX, FlipY: st.flipY, Tint: tint,
			MatColor: st.matColor, Add: st.matAdd, Blend: st.matBlend,
			HueShift: st.matHueShift, LinearAdd: st.matLinearAdd,
			Doodle:       st.matDoodle,
			OutlineWidth: st.outlineWidth,
		}
		if s.as.Rig.Nodes[i].DrawMode != 0 {
			// sliced/tiled：m_Size 是权威尺寸——动画把它压到 0 即等于隐藏
			//（原版光束收束就是 size.y→0），不能退化成"按原始尺寸绘制"
			if st.size[0] <= 0 || st.size[1] <= 0 {
				continue
			}
			opts.Stretch = st.size
		}
		view, ok := s.camView(s.worldZ[i])
		if !ok {
			continue // 相机背后
		}
		if s.as.Rig.Nodes[i].MaskIn == 1 {
			// Unity VisibleInsideMask：先画到离屏，再与掩码并集做
			// DestinationIn 合成（无活动掩码时不可见）。
			if len(masks) == 0 {
				continue
			}
			ensureScratch()
			s.scratch.Clear()
			if s.as.Rig.Nodes[i].Mapped {
				pal := s.paletteForNode(i)
				if st.hasPalette {
					pal = st.palette
				}
				if st.hasMatThreshold {
					pal.Threshold = st.matThreshold
				}
				if st.hasMatProgress {
					pal.Progress = st.matProgress
					pal.UseProgress = true
				}
				s.as.DrawSpriteMapped(s.scratch, st.sprite, view.Mul(s.world[i]), proj, opts, pal)
			} else {
				s.as.DrawSpriteOpts(s.scratch, st.sprite, view.Mul(s.world[i]), proj, opts)
			}
			drawMasks()
			mop := &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn}
			s.scratch.DrawImage(s.maskBuf, mop)
			dst.DrawImage(s.scratch, nil)
			continue
		}
		if s.as.Rig.Nodes[i].Mapped {
			pal := s.paletteForNode(i)
			if st.hasPalette {
				pal = st.palette
			}
			if st.hasMatThreshold {
				pal.Threshold = st.matThreshold
			}
			if st.hasMatProgress {
				pal.Progress = st.matProgress
				pal.UseProgress = true
			}
			s.as.DrawSpriteMapped(dst, st.sprite, view.Mul(s.world[i]), proj, opts, pal)
		} else {
			s.as.DrawSpriteOpts(dst, st.sprite, view.Mul(s.world[i]), proj, opts)
		}
	}
	s.queued = s.queued[:0]
	s.queuedMeshes = s.queuedMeshes[:0]
}
