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

	drawOrder []int // 预排序的可绘制节点（layer, order, dfs）
}

func NewScene(as *Assets) *SceneInst {
	s := &SceneInst{
		as:      as,
		byPath:  map[string]int{},
		state:   make([]sceneNodeState, len(as.Rig.Nodes)),
		worldZ:  make([]float64, len(as.Rig.Nodes)),
		world:   make([]Aff, len(as.Rig.Nodes)),
		actives: make([]bool, len(as.Rig.Nodes)),
		players: map[string]*scenePlayer{},
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

// Sample 按歌曲节拍采样所有播放器并更新世界变换。
func (s *SceneInst) Sample(beat float64) {
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
	for _, p := range s.players {
		clipT := (beat - p.startBeat) * p.timeScale
		if clipT < 0 {
			clipT = 0
		}
		if p.anim.Loop && p.anim.Duration > 0 {
			clipT = math.Mod(clipT, p.anim.Duration)
		} else if clipT > p.anim.Duration {
			clipT = p.anim.Duration // 非循环：保持末帧
		}
		s.applyClip(p, clipT)
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
	}
	items := make([]item, 0, len(s.state))
	for i := range s.state {
		st := &s.state[i]
		if !s.actives[i] || !st.renderOn {
			continue
		}
		if st.sprite == "" || st.color[3] <= 0 {
			continue
		}
		it := item{idx: i, layer: s.as.Rig.Nodes[i].Layer, order: st.order, z: s.worldZ[i]}
		if g := s.groupOf[i]; g >= 0 {
			sg := s.as.Rig.Nodes[g].SortGroup
			it.gIdx, it.gLayer, it.gOrder, it.gZ = g, sg[0], sg[1], s.worldZ[g]
		} else {
			it.gIdx, it.gLayer, it.gOrder, it.gZ = i, it.layer, it.order, it.z
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
	for _, it := range items {
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
		world := s.world[i]
		if z := s.worldZ[i]; z != 0 {
			ps := CamDist / (CamDist + z)
			if ps <= 0 {
				continue // 相机背后
			}
			world = Scale(ps, ps).Mul(world)
		}
		s.as.DrawSpriteOpts(dst, st.sprite, world, proj, opts)
	}
}
