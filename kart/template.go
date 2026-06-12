// template.go：prefab 子树的多实例运行时（Instantiate(template, parent) 等价物）。
//
// totemClimb 的图腾/青蛙/龙/鸟等都是同一模板的多个实例，各自带独立的
// Animator（含 controller 状态机）与激活状态。SceneInst 的播放器以节点
// path 为键，无法承载同一子树的多个并行实例，因此实例自持状态：
//   - 模板：子树节点的相对变换/切片/排序（取自 Assets.Rig）
//   - 实例：根世界位移 + 每个 Animator 根的剪辑播放器 + SetActive 覆盖
//   - 绘制：实例采样后注入 SceneInst.Queue，与场景节点统一排序
package kart

import (
	"math"
	"strings"

	"hsdemo/kmdata"
)

// TmplNode 是模板子树的一个节点（下标指向 Assets.Rig.Nodes）。
type TmplNode struct {
	RigIdx  int
	RelPath string // 相对模板根（"" = 根自身）
	Parent  int    // 模板内父下标（根为 -1）
}

// Template 是一个可多实例化的 prefab 子树。
type Template struct {
	as       *Assets
	RootPath string
	Nodes    []TmplNode
	// animRoots：子树内挂 Animator 的节点（模板内下标）→ controller 名
	animRoots map[int]string
}

// NewTemplate 按根 path 收集子树（重名 path 时按节点下标 NewTemplateIdx）。
func NewTemplate(as *Assets, rootPath string) *Template {
	rootIdx := -1
	for i := range as.Rig.Nodes {
		if as.Rig.Nodes[i].Path == rootPath {
			rootIdx = i
			break
		}
	}
	if rootIdx < 0 {
		return nil
	}
	return NewTemplateIdx(as, rootIdx)
}

// NewTemplateIdx 按根节点下标收集子树。
func NewTemplateIdx(as *Assets, rootIdx int) *Template {
	t := &Template{as: as, RootPath: as.Rig.Nodes[rootIdx].Path, animRoots: map[int]string{}}
	idxMap := map[int]int{rootIdx: 0} // rig 下标 → 模板内下标
	t.Nodes = append(t.Nodes, TmplNode{RigIdx: rootIdx, RelPath: "", Parent: -1})
	rootPrefix := t.RootPath + "/"
	for i := rootIdx + 1; i < len(as.Rig.Nodes); i++ {
		n := &as.Rig.Nodes[i]
		pi, ok := idxMap[n.Parent]
		if !ok {
			continue // 越过子树（DFS 先序保证子树连续，但保险起见按父链判断）
		}
		idxMap[i] = len(t.Nodes)
		t.Nodes = append(t.Nodes, TmplNode{
			RigIdx:  i,
			RelPath: strings.TrimPrefix(n.Path, rootPrefix),
			Parent:  pi,
		})
	}
	// Animator 绑定（animators.json 的 path 命中子树内任意节点）
	for path, ctrl := range as.Animators {
		for ti, tn := range t.Nodes {
			if as.Rig.Nodes[tn.RigIdx].Path == path {
				t.animRoots[ti] = ctrl
				break // path 重名时绑定首个（Unity 同语义）
			}
		}
	}
	return t
}

// instPlayer 是实例内一个 Animator 的播放器（含可选状态机）。
type instPlayer struct {
	rootTI    int // 模板内下标
	anim      *kmdata.Anim
	startBeat float64
	timeScale float64
	machine   *smachine
	frozen    bool    // Play(name, 0, t) 暂停语义
	frozenT   float64 // 暂停时的剪辑时间（秒）
}

// Instance 是模板的一个实例。
type Instance struct {
	T *Template
	// Offset：实例根的本地位移（替换模板根的 prefab 位移；
	// Instantiate 后 localPosition 被代码改写的语义）
	Offset [2]float64
	// 叠加变换：实例整体的额外世界仿射（滚动容器等），绘制时左乘
	players map[int]*instPlayer
	actives map[int]bool // 模板内下标 → SetActive 覆盖
	sprites map[int]string
}

// NewInstance 创建实例（Offset 先取模板根的 prefab 位置）。
func (t *Template) NewInstance() *Instance {
	root := &t.as.Rig.Nodes[t.Nodes[0].RigIdx]
	in := &Instance{
		T:       t,
		Offset:  root.Pos,
		players: map[int]*instPlayer{},
		actives: map[int]bool{},
		sprites: map[int]string{},
	}
	// controller 默认状态不自动播（Unity 激活时播默认态；由调用方
	// PlayDefaultState 以正确的 timeScale 启动）
	return in
}

// findAnimRoot 把"子树内相对 path"解析为带 Animator 的模板下标。
func (in *Instance) findAnimRoot(relPath string) (int, bool) {
	for ti := range in.T.animRoots {
		if in.T.Nodes[ti].RelPath == relPath {
			return ti, true
		}
	}
	return -1, false
}

// PlayState 在实例的 Animator（相对 path）上按状态名播放。
func (in *Instance) PlayState(relPath, stateName string, startBeat, timeScale float64) {
	ti, ok := in.findAnimRoot(relPath)
	if !ok {
		return
	}
	ctrlName := in.T.animRoots[ti]
	ctrl, ok := in.T.as.Controllers[ctrlName]
	if !ok {
		return
	}
	st, ok := ctrl.States[stateName]
	if !ok {
		return
	}
	p := in.players[ti]
	if p == nil || p.machine == nil {
		params := map[string]bool{}
		for k, v := range ctrl.Params {
			params[k] = v
		}
		c := ctrl
		p = &instPlayer{rootTI: ti, machine: &smachine{ctrl: &c, params: params}}
		in.players[ti] = p
	}
	p.machine.state, p.machine.lastT = stateName, 0
	p.frozen = false
	if st.Clip == "" || st.Speed*timeScale == 0 {
		p.anim = nil
		return
	}
	p.anim = in.T.as.Anims[st.Clip]
	p.startBeat, p.timeScale = startBeat, timeScale*st.Speed
}

// PlayFrozen 以暂停状态把状态摆到指定归一化时间（Anim.Play(name,0,t)+不推进；
// frog 的 WingsNoFlap 用 t=0）。
func (in *Instance) PlayFrozen(relPath, stateName string, normT float64) {
	in.PlayState(relPath, stateName, 0, 1)
	ti, ok := in.findAnimRoot(relPath)
	if !ok {
		return
	}
	if p := in.players[ti]; p != nil && p.anim != nil {
		p.frozen = true
		p.frozenT = normT * p.anim.Duration
	}
}

// PlayDefaultState 进入 controller 默认状态。
func (in *Instance) PlayDefaultState(relPath string, startBeat, timeScale float64) {
	ti, ok := in.findAnimRoot(relPath)
	if !ok {
		return
	}
	ctrl := in.T.as.Controllers[in.T.animRoots[ti]]
	in.PlayState(relPath, ctrl.Default, startBeat, timeScale)
}

// CurrentState 返回实例 Animator 的当前状态名（"" = 未启动）。
func (in *Instance) CurrentState(relPath string) string {
	ti, ok := in.findAnimRoot(relPath)
	if !ok {
		return ""
	}
	if p := in.players[ti]; p != nil && p.machine != nil {
		return p.machine.state
	}
	return ""
}

// SetActive 覆盖子树内节点（相对 path）的激活状态。
func (in *Instance) SetActive(relPath string, active bool) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.actives[ti] = active
			return
		}
	}
}

// SetSprite 覆盖子树内节点的切片（鸟的企鹅换皮等）。
func (in *Instance) SetSprite(relPath, sprite string) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.sprites[ti] = sprite
			return
		}
	}
}

// stepMachine 推进实例的一个状态机（SceneInst.stepMachines 的实例版）。
func (in *Instance) stepMachine(p *instPlayer, beat float64) {
	for iter := 0; iter < 8; iter++ {
		if p.machine == nil || p.machine.state == "" || p.anim == nil ||
			p.frozen || p.timeScale <= 0 || p.anim.Duration <= 0 {
			return
		}
		st := p.machine.ctrl.States[p.machine.state]
		clipT := (beat - p.startBeat) * p.timeScale
		if clipT < 0 {
			return
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
			for _, cnd := range tr.Conds {
				v := p.machine.params[cnd.Param]
				if (cnd.Mode == "if" && !v) || (cnd.Mode == "ifnot" && v) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			if p.machine.lastT < gateT {
				fireBeat = p.startBeat + gateT/p.timeScale
			} else {
				fireBeat = beat
			}
			fired = tr
			break
		}
		if fired == nil {
			p.machine.lastT = clipT
			return
		}
		dst, ok := p.machine.ctrl.States[fired.Dst]
		if !ok {
			p.machine.lastT = clipT
			return
		}
		baseTS := p.timeScale / maxf(st.Speed, 1e-9)
		p.machine.state, p.machine.lastT = fired.Dst, 0
		if dst.Clip == "" || dst.Speed*baseTS == 0 {
			p.anim = nil
			return
		}
		p.anim = in.T.as.Anims[dst.Clip]
		p.startBeat, p.timeScale = fireBeat, baseTS*dst.Speed
	}
}

// instNodeState 是实例采样后的节点状态。
type instNodeState struct {
	pos      [2]float64
	rot      float64
	scale    [2]float64
	sprite   string
	flipX    bool
	flipY    bool
	active   bool
	renderOn bool
	color    [4]float64
	order    int
}

// Queue 采样实例并把可见节点注入 scene 的统一排序绘制。
// baseWorld：实例外层的世界变换（滚动容器等），作用于实例根。
// z：排序深度（透视用，通常 0）。
func (in *Instance) Queue(scene *SceneInst, beat float64, baseWorld Aff, z float64) {
	t := in.T
	states := make([]instNodeState, len(t.Nodes))
	for ti, tn := range t.Nodes {
		n := &t.as.Rig.Nodes[tn.RigIdx]
		c := n.Color
		if c == [4]float64{} {
			c = [4]float64{1, 1, 1, 1}
		}
		states[ti] = instNodeState{
			pos: n.Pos, rot: n.RotZ, scale: n.Scale,
			sprite: n.Sprite, flipX: n.FlipX, flipY: n.FlipY,
			active: !n.Inactive, renderOn: !n.Hidden,
			color: c, order: n.Order,
		}
	}
	states[0].pos = in.Offset
	states[0].active = true // 模板本体可能 inactive（Instantiate 后 SetActive(true) 语义）
	for ti, v := range in.actives {
		states[ti].active = v
	}
	for ti, sp := range in.sprites {
		states[ti].sprite = sp
	}
	// 剪辑采样
	for _, p := range in.players {
		in.stepMachine(p, beat)
		if p.anim == nil {
			continue
		}
		var clipT float64
		if p.frozen {
			clipT = p.frozenT
		} else {
			clipT = (beat - p.startBeat) * p.timeScale
			if clipT < 0 {
				clipT = 0
			}
			if p.anim.Loop && p.anim.Duration > 0 {
				clipT = math.Mod(clipT, p.anim.Duration)
			} else if clipT > p.anim.Duration {
				clipT = p.anim.Duration
			}
		}
		in.applyClip(p, states, clipT)
	}
	// 合成 + 注入
	world := make([]Aff, len(t.Nodes))
	actives := make([]bool, len(t.Nodes))
	for ti, tn := range t.Nodes {
		st := &states[ti]
		local := TRS(st.pos[0], st.pos[1], st.rot, st.scale[0], st.scale[1])
		if tn.Parent < 0 {
			world[ti] = baseWorld.Mul(local)
			actives[ti] = st.active
		} else {
			world[ti] = world[tn.Parent].Mul(local)
			actives[ti] = st.active && actives[tn.Parent]
		}
		if !actives[ti] || !st.renderOn || st.sprite == "" || st.color[3] <= 0 {
			continue
		}
		n := &t.as.Rig.Nodes[tn.RigIdx]
		scene.Queue(ExtraSprite{
			Sprite: st.sprite, World: world[ti], Z: z,
			Layer: n.Layer, Order: st.order,
			FlipX: st.flipX, FlipY: st.flipY, Tint: st.color,
		})
	}
}

// NodeWorld 返回子树内节点（相对 path）在 baseWorld 下的世界变换
//（JumperPoint 等锚点查询；需与 Queue 相同的 beat 采样口径——
// 锚点节点不被剪辑驱动时直接按 prefab 变换合成）。
func (in *Instance) NodeWorld(relPath string, baseWorld Aff) (Aff, bool) {
	t := in.T
	target := -1
	for ti, tn := range t.Nodes {
		if tn.RelPath == relPath {
			target = ti
			break
		}
	}
	if target < 0 {
		return Identity(), false
	}
	// 自根向下合成（锚点父链不含剪辑驱动节点的场合；totemClimb 的
	// JumperPoint 都是静态子节点，剪辑只动头部堆叠）
	aff := baseWorld
	chain := []int{}
	for ti := target; ti >= 0; ti = t.Nodes[ti].Parent {
		chain = append(chain, ti)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		ti := chain[i]
		n := &t.as.Rig.Nodes[t.Nodes[ti].RigIdx]
		pos := n.Pos
		if ti == 0 {
			pos = in.Offset
		}
		aff = aff.Mul(TRS(pos[0], pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	return aff, true
}

// applyClip 把剪辑曲线套到实例节点状态（path 相对 Animator 根）。
func (in *Instance) applyClip(p *instPlayer, states []instNodeState, at float64) {
	t := in.T
	animRootRel := t.Nodes[p.rootTI].RelPath
	resolve := func(curvePath string) (int, bool) {
		full := curvePath
		if animRootRel != "" {
			if curvePath == "" {
				full = animRootRel
			} else {
				full = animRootRel + "/" + curvePath
			}
		}
		for ti, tn := range t.Nodes {
			if tn.RelPath == full {
				return ti, true
			}
		}
		return -1, false
	}
	a := p.anim
	for path, c := range a.Pos {
		if ti, ok := resolve(path); ok {
			if len(c.X) > 0 {
				states[ti].pos[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				states[ti].pos[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Euler {
		if ti, ok := resolve(path); ok && len(keys) > 0 {
			states[ti].rot = evalKeys(keys, at) * math.Pi / 180
		}
	}
	for path, c := range a.Scale {
		if ti, ok := resolve(path); ok {
			if len(c.X) > 0 {
				states[ti].scale[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				states[ti].scale[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Sprites {
		if ti, ok := resolve(path); ok {
			if name, ok2 := sampleSwap(keys, at); ok2 {
				states[ti].sprite = name
			}
		}
	}
	for path, attrs := range a.Floats {
		ti, ok := resolve(path)
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
				states[ti].flipX = v > 0.5
			case attr == "m_FlipY":
				states[ti].flipY = v > 0.5
			case attr == "m_SortingOrder":
				states[ti].order = int(v)
			case attr == "m_IsActive":
				states[ti].active = v > 0.5
			case attr == "m_Enabled":
				states[ti].renderOn = v > 0.5
			case strings.HasPrefix(attr, "m_Color."):
				switch attr[len("m_Color."):] {
				case "r":
					states[ti].color[0] = v
				case "g":
					states[ti].color[1] = v
				case "b":
					states[ti].color[2] = v
				case "a":
					states[ti].color[3] = v
				}
			}
		}
	}
}
