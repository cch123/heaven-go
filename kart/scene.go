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
	pos      [2]float64
	rot      float64
	scale    [2]float64
	sprite   string
	flipX    bool
	flipY    bool
	active   bool // GameObject m_IsActive（沿层级传播）
	renderOn bool // SpriteRenderer m_Enabled（仅本节点，不传播）
	color    [4]float64
	size     [2]float64 // drawMode != 0 时生效
	order    int        // sortingOrder（可被动画驱动）
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
//（DoScaledAnimationAsync 按状态名播放 + 剪辑结束按 bool 条件转换）。
type smachine struct {
	ctrl   *kmdata.Controller
	state  string
	params map[string]bool
	lastT  float64 // 上次 Sample 的剪辑时间（循环边界跨越检测）
}

// SceneInst 是一个可播放多路动画的场景实例。
type SceneInst struct {
	as      *Assets
	byPath  map[string]int
	state   []sceneNodeState
	world   []Aff
	worldZ  []float64 // 节点深度（透视投影：s = CamDist/(CamDist+z)）
	actives []bool    // activeInHierarchy
	groupOf []int     // 节点归属的 SortingGroup 节点下标（-1 = 无）
	players map[string]*scenePlayer

	machines map[string]*smachine // rootPath → 状态机（有 controller 的 Animator）

	// 模块驱动的持久覆盖（在 prefab 默认值之后、剪辑采样之前生效）
	spinOver   map[int]float64    // 节点下标 → 旋转叠加（弧度，transform.Rotate 积分）
	activeOver map[int]bool       // 节点下标 → m_IsActive 覆盖（SetActive 语义）
	mirrorOver map[int]bool       // 节点下标 → localScale.x 取负（transform.localScale=(-1,1,1)）
	colorOver  map[int][4]float64 // 节点下标 → SpriteRenderer.color 覆盖
	posOver    map[int][2]float64 // 节点下标 → localPosition 覆盖（伪相机平移等）
	zOver      map[int]float64    // 节点下标 → 世界 z 覆盖（kitties 斜列生成等，根节点语义）
	spriteOver map[int]string     // 节点下标 → 切片覆盖（sr.sprite 直写，如海报换图）

	queued []ExtraSprite // 本帧注入的动态绘制项

	cam    [3]float64 // 相机世界位置（vfx/move camera），默认 (0,0,-10)
	hasCam bool

	palette  Palette            // 映射材质默认调色板（单材质游戏）
	palettes map[string]Palette // 按材质名（Node.Mat）覆盖（多材质游戏）

	drawOrder []int // 预排序的可绘制节点（layer, order, dfs）

	scratch *ebiten.Image // SpriteMask 合成的离屏缓冲（懒分配）
	maskBuf *ebiten.Image
}

// SetCamera 设置相机世界位置（GameCamera：默认 (0,0,-10)、FOV 53.15°）。
// 透视缩放从 s = CamDist/(CamDist+z) 推广为 s = CamDist/(z - camZ)，
// 屏幕坐标先平移 -cam.xy 再缩放（vfx/move camera 的拉近/平移）。
func (s *SceneInst) SetCamera(x, y, z float64) {
	s.cam, s.hasCam = [3]float64{x, y, z}, true
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

// paletteOf 取节点应使用的调色板。
func (s *SceneInst) paletteOf(mat string) Palette {
	if p, ok := s.palettes[mat]; ok {
		return p
	}
	return s.palette
}

// camView 返回节点深度 z 处的视图变换（含相机平移与透视缩放）；ok=false 表示在相机背后。
func (s *SceneInst) camView(z float64) (Aff, bool) {
	if !s.hasCam {
		if z == 0 {
			return Identity(), true
		}
		ps := CamDist / (CamDist + z)
		if ps <= 0 {
			return Identity(), false
		}
		return Scale(ps, ps), true
	}
	d := z - s.cam[2]
	if d <= 0 {
		return Identity(), false
	}
	ps := CamDist / d
	return Scale(ps, ps).Mul(Translate(-s.cam[0], -s.cam[1])), true
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
		mirrorOver: map[int]bool{},
		colorOver:  map[int][4]float64{},
		posOver:    map[int][2]float64{},
		zOver:      map[int]float64{},
		spriteOver: map[int]string{},
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
	anim, ok := s.as.Anims[clip]
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	s.players[rootPath] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: clip,
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
//（Unity 等价 Play(name, 0, t) + speed 0，cartGuy 的推车位移用它逐帧驱动）。
func (s *SceneInst) PlayNormalized(rootPath, clip string, t float64) {
	anim, ok := s.as.Anims[clip]
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	s.players[rootPath] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: clip,
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
	anim, ok := s.as.Anims[st.Clip]
	if !ok {
		return
	}
	idx, ok := s.byPath[rootPath]
	if !ok {
		return
	}
	m.state, m.lastT = stateName, 0
	s.players[rootPath] = &scenePlayer{
		rootIdx: idx, rootPath: rootPath, anim: anim, clipName: st.Clip,
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

// SetActive 覆盖节点 m_IsActive（GameObject.SetActive 语义，沿层级传播）。
func (s *SceneInst) SetActive(path string, active bool) {
	if i, ok := s.byPath[path]; ok {
		s.activeOver[i] = active
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

// SetPosOver 覆盖节点 localPosition（伪相机 gameTrans 平移等）。
func (s *SceneInst) SetPosOver(path string, x, y float64) {
	if i, ok := s.byPath[path]; ok {
		s.posOver[i] = [2]float64{x, y}
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
	FlipX, FlipY bool
	Tint         [4]float64 // 零值视为白色
	Mapped       bool       // 调色板映射材质（SceneInst.SetPalette）
	Mat          string     // 映射材质名（按名调色板）
}

// Queue 注入一帧动态绘制项（Draw 后清空，每帧重新注入）。
func (s *SceneInst) Queue(e ExtraSprite) { s.queued = append(s.queued, e) }

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
			color: c, size: n.Size, order: n.Order,
		}
	}
	for i, v := range s.activeOver {
		s.state[i].active = v
	}
	for i, v := range s.colorOver {
		s.state[i].color = v
	}
	for i, v := range s.posOver {
		s.state[i].pos = v
	}
	for _, p := range s.players {
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

func (s *SceneInst) node(p *scenePlayer, curvePath string) (int, bool) {
	full := p.rootPath
	if curvePath != "" {
		if full == "" {
			full = curvePath
		} else {
			full = full + "/" + curvePath
		}
	}
	i, ok := s.byPath[full]
	return i, ok
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
			case strings.HasPrefix(attr, "m_Color."):
				switch attr[len("m_Color."):] {
				case "r":
					s.state[i].color[0] = v
				case "g":
					s.state[i].color[1] = v
				case "b":
					s.state[i].color[2] = v
				case "a":
					s.state[i].color[3] = v
				}
			}
		}
	}
}

// CamDist 是 GameCamera 默认相机距离（位置 (0,0,-10)，FOV 53.15°，
// 在 z=0 平面恰好等价于半高 5 的正交视野）。
const CamDist = 10.0

// Draw 按 (sortingLayer, sortingOrder, 深度, DFS) 顺序绘制（需先 Sample）。
// sortingOrder 可能被动画驱动（m_SortingOrder 曲线），故每帧重排；
// 节点深度 z 经透视投影缩放（s = CamDist/(CamDist+z)），复刻原版透视相机。
func (s *SceneInst) Draw(dst *ebiten.Image, proj Aff) {
	type item struct {
		idx, layer, order int
		z                 float64
		gIdx              int // 排序单元（SortingGroup 根或自身）
		gLayer, gOrder    int
		gZ                float64
		extra             int // ≥0：s.queued 下标（动态绘制项）
	}
	items := make([]item, 0, len(s.state)+len(s.queued))
	// 活动的 SpriteMask（本体不绘制，为 MaskIn=1 的渲染器提供可见区域）
	var masks []int
	for i := range s.state {
		if s.as.Rig.Nodes[i].Mask && s.actives[i] && s.state[i].renderOn && s.state[i].sprite != "" {
			masks = append(masks, i)
		}
	}
	for i := range s.state {
		st := &s.state[i]
		if !s.actives[i] || !st.renderOn {
			continue
		}
		if st.sprite == "" || st.color[3] <= 0 {
			continue
		}
		if s.as.Rig.Nodes[i].Mask {
			continue
		}
		it := item{idx: i, layer: s.as.Rig.Nodes[i].Layer, order: st.order, z: s.worldZ[i], extra: -1}
		if g := s.groupOf[i]; g >= 0 {
			sg := s.as.Rig.Nodes[g].SortGroup
			it.gIdx, it.gLayer, it.gOrder, it.gZ = g, sg[0], sg[1], s.worldZ[g]
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = i, it.layer, it.order, it.z
		}
		items = append(items, it)
	}
	for qi := range s.queued {
		q := &s.queued[qi]
		it := item{
			idx: len(s.state) + qi, layer: q.Layer, order: q.Order, z: q.Z, extra: qi,
		}
		it.gIdx, it.gLayer, it.gOrder, it.gZ = it.idx, q.Layer, q.Order, q.Z
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
	for _, it := range items {
		if it.extra >= 0 {
			q := &s.queued[it.extra]
			view, ok := s.camView(q.Z)
			if !ok {
				continue
			}
			qo := SpriteOpts{FlipX: q.FlipX, FlipY: q.FlipY, Tint: q.Tint}
			if q.Mapped {
				s.as.DrawSpriteMapped(dst, q.Sprite, view.Mul(q.World), proj, qo, s.paletteOf(q.Mat))
			} else {
				s.as.DrawSpriteOpts(dst, q.Sprite, view.Mul(q.World), proj, qo)
			}
			continue
		}
		i := it.idx
		st := &s.state[i]
		opts := SpriteOpts{FlipX: st.flipX, FlipY: st.flipY, Tint: st.color}
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
			w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
			if s.scratch == nil || s.scratch.Bounds().Dx() != w || s.scratch.Bounds().Dy() != h {
				s.scratch = ebiten.NewImage(w, h)
				s.maskBuf = ebiten.NewImage(w, h)
			}
			s.scratch.Clear()
			if s.as.Rig.Nodes[i].Mapped {
				s.as.DrawSpriteMapped(s.scratch, st.sprite, view.Mul(s.world[i]), proj, opts, s.paletteOf(s.as.Rig.Nodes[i].Mat))
			} else {
				s.as.DrawSpriteOpts(s.scratch, st.sprite, view.Mul(s.world[i]), proj, opts)
			}
			s.maskBuf.Clear()
			for _, mi := range masks {
				mview, ok := s.camView(s.worldZ[mi])
				if !ok {
					continue
				}
				ms := &s.state[mi]
				s.as.DrawSpriteOpts(s.maskBuf, ms.sprite, mview.Mul(s.world[mi]), proj,
					SpriteOpts{FlipX: ms.flipX, FlipY: ms.flipY})
			}
			mop := &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn}
			s.scratch.DrawImage(s.maskBuf, mop)
			dst.DrawImage(s.scratch, nil)
			continue
		}
		if s.as.Rig.Nodes[i].Mapped {
			s.as.DrawSpriteMapped(dst, st.sprite, view.Mul(s.world[i]), proj, opts, s.paletteOf(s.as.Rig.Nodes[i].Mat))
		} else {
			s.as.DrawSpriteOpts(dst, st.sprite, view.Mul(s.world[i]), proj, opts)
		}
	}
	s.queued = s.queued[:0]
}
