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
	animRoots    map[int]string
	meshBindings map[int][]int // 模板内节点下标 → as.Meshes.Bindings 下标
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
	t := &Template{as: as, RootPath: as.Rig.Nodes[rootIdx].Path, animRoots: map[int]string{}, meshBindings: map[int][]int{}}
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
	for bi, b := range as.Meshes.Bindings {
		for ti, tn := range t.Nodes {
			if as.Rig.Nodes[tn.RigIdx].Path == b.Path {
				t.meshBindings[ti] = append(t.meshBindings[ti], bi)
				break
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
	// Rot：实例根的附加旋转（弧度；transform.rotation 直写语义，如收腿翻滚）
	Rot float64
	// Scale：实例根的附加缩放。默认 (1,1)；用于 prefab 实例被代码临时
	// squash / shrink 的场合，避免改共享模板节点。
	Scale [2]float64
	// 叠加变换：实例整体的额外世界仿射（滚动容器等），绘制时左乘
	players    map[int]*instPlayer
	layerOrder []string
	layers     map[string]*instPlayer
	actives    map[int]bool // 模板内下标 → SetActive 覆盖
	sprites    map[int]string
	colors     map[int][4]float64 // SpriteRenderer.color 覆盖（sr.color 直写）
	palettes   map[int]Palette    // mapped material overrides per instance renderer
	matAdd     map[int][4]float64 // material._AddColor 覆盖（screen 混合）
	orders     map[int]int        // SpriteRenderer.sortingOrder 覆盖（sr.sortingOrder 直写）
	groupLayer int                // SortingGroup.sortingLayer；当前提取资源多为默认 layer 0
	groupOrder int                // SortingGroup.sortingOrder；作为组级排序键，不混入子 renderer order
	hasGroup   bool
	pos        map[int][2]float64 // Transform.localPosition 覆盖（脚本每帧写 transform）
	rots       map[int]float64    // Transform.localEulerAngles.z 覆盖（弧度）
	scales     map[int][2]float64 // Transform.localScale 覆盖
}

// NewInstance 创建实例（Offset 先取模板根的 prefab 位置）。
func (t *Template) NewInstance() *Instance {
	root := &t.as.Rig.Nodes[t.Nodes[0].RigIdx]
	in := &Instance{
		T:        t,
		Offset:   root.Pos,
		Scale:    [2]float64{1, 1},
		players:  map[int]*instPlayer{},
		layers:   map[string]*instPlayer{},
		actives:  map[int]bool{},
		sprites:  map[int]string{},
		colors:   map[int][4]float64{},
		palettes: map[int]Palette{},
		matAdd:   map[int][4]float64{},
		orders:   map[int]int{},
		pos:      map[int][2]float64{},
		rots:     map[int]float64{},
		scales:   map[int][2]float64{},
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

func (in *Instance) findNode(relPath string) (int, bool) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			return ti, true
		}
	}
	return -1, false
}

// Play 在实例子树的相对节点上直接播放 AnimationClip。部分 Unity prefab
// 会复用目录外的 AnimatorController（Catchy Tune 的 pineapple），提取器
// 不能安全内联该 controller 时仍要保留剪辑曲线本身，因此提供 raw clip
// 路径复刻 Animator.Play/DoScaledAnimation 的采样语义。
func (in *Instance) Play(relPath, clip string, startBeat, timeScale float64) {
	anim, ok := in.T.as.Anims[clip]
	if !ok {
		return
	}
	ti, ok := in.findNode(relPath)
	if !ok {
		return
	}
	in.players[ti] = &instPlayer{rootTI: ti, anim: anim, startBeat: startBeat, timeScale: timeScale}
}

// PlayLayer 在实例子树上播放独立曲线层。Unity 允许同一 Animator 的
// layer 1 命中动画叠在 layer 0 移动动画上，Cannery 的罐头需要这个语义。
func (in *Instance) PlayLayer(key, relPath, clip string, startBeat, timeScale float64) {
	anim, ok := in.T.as.Anims[clip]
	if !ok {
		return
	}
	ti, ok := in.findNode(relPath)
	if !ok {
		return
	}
	if _, exists := in.layers[key]; !exists {
		in.layerOrder = append(in.layerOrder, key)
	}
	in.layers[key] = &instPlayer{rootTI: ti, anim: anim, startBeat: startBeat, timeScale: timeScale}
}

// PlayNormalized 以固定归一化时间采样实例剪辑（SceneInst.PlayNormalized 的
// prefab-instance 版本）。
func (in *Instance) PlayNormalized(relPath, clip string, t float64) {
	anim, ok := in.T.as.Anims[clip]
	if !ok {
		return
	}
	ti, ok := in.findNode(relPath)
	if !ok {
		return
	}
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	in.players[ti] = &instPlayer{
		rootTI: ti, anim: anim, startBeat: 0, timeScale: 1,
		frozen: true, frozenT: t * anim.Duration,
	}
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

// PlayStateLayer mirrors Animator layer playback for template instances. It
// resolves the state through the controller but keeps transition ownership on
// the base PlayState player, matching SceneInst.PlayStateLayer.
func (in *Instance) PlayStateLayer(key, relPath, stateName string, startBeat, timeScale float64) {
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
	if st.Clip == "" || st.Speed*timeScale == 0 {
		delete(in.layers, key)
		return
	}
	in.PlayLayer(key, relPath, st.Clip, startBeat, timeScale*st.Speed)
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

// SetColor 覆盖子树内节点的颜色（sr.color 直写，如饺子调色）。
func (in *Instance) SetColor(relPath string, c [4]float64) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.colors[ti] = c
			return
		}
	}
}

// SetPalette 覆盖子树内 mapped renderer 的材质调色板。Unity Instantiate 会给
// Shoot-'Em-Up 敌机各自 new Material；实例级 palette 保留这个非共享语义。
func (in *Instance) SetPalette(relPath string, p Palette) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.palettes[ti] = p
			return
		}
	}
}

// SetOrder 覆盖子树内节点的 sortingOrder。Wizard's Waltz 的花会根据
// z 位置每实例改排序；共享模板节点不能只改全局 rig order。
func (in *Instance) SetOrder(relPath string, order int) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.orders[ti] = order
			return
		}
	}
}

// SetGroupOrder assigns a Unity SortingGroup sortingOrder to a queued prefab
// instance. The group order must stay separate from child renderer order,
// otherwise dynamic templates compare incorrectly with scene SortingGroups.
func (in *Instance) SetGroupOrder(order int) {
	in.groupOrder = order
	in.hasGroup = true
}

// SetPos 覆盖子树内节点的本地坐标。Splashdown 的 NtrSynchrette.Update
// 每帧直接写 PosHolder.localPosition；该类脚本运动不属于 AnimationClip。
func (in *Instance) SetPos(relPath string, x, y float64) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.pos[ti] = [2]float64{x, y}
			return
		}
	}
}

// SetRot 覆盖子树内节点的本地 z 旋转（弧度）。
func (in *Instance) SetRot(relPath string, rot float64) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.rots[ti] = rot
			return
		}
	}
}

// SetScale 覆盖子树内节点的本地缩放。
func (in *Instance) SetScale(relPath string, sx, sy float64) {
	for ti, tn := range in.T.Nodes {
		if tn.RelPath == relPath {
			in.scales[ti] = [2]float64{sx, sy}
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
	pos             [2]float64
	rot             float64
	scale           [2]float64
	sprite          string
	flipX           bool
	flipY           bool
	active          bool
	renderOn        bool
	color           [4]float64
	matColor        [4]float64
	hasMatColor     bool
	matAlpha        float64 // material._Alpha：材质级透明度，最终与 SpriteRenderer.color.a 相乘
	matOpacity      float64 // material._Opacity：与 _Alpha 相乘，避免两个 shader 字段互相覆盖
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
	order           int
}

// Queue 采样实例并把可见节点注入 scene 的统一排序绘制。
// baseWorld：实例外层的世界变换（滚动容器等），作用于实例根。
// z：排序深度（透视用，通常 0）。
func (in *Instance) Queue(scene *SceneInst, beat float64, baseWorld Aff, z float64) int {
	t := in.T
	groupKey := -1
	if in.hasGroup {
		// Dynamic instances do not have a scene node index. Use the first queued
		// slot for this frame as a stable group key so all child renderers sort
		// together before falling back to their own local renderer order.
		groupKey = len(scene.state) + len(scene.queued) + len(scene.queuedMeshes)
	}
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
			color: c, matAlpha: 1, matOpacity: 1, order: n.Order,
			palette: scene.paletteOf(n.Mat),
		}
		if v, ok := scene.matFor[n.Mat]; ok && scene.materialForApplies(n.Mat, n.Path) {
			if v.hasColor {
				states[ti].matColor = v.color
				states[ti].hasMatColor = true
			}
			states[ti].matAdd = v.add
			states[ti].matHueShift = v.hueShift
			states[ti].matLinearAdd = v.linearAdd
			states[ti].matDoodle = v.doodle
		}
	}
	states[0].pos = in.Offset
	states[0].rot += in.Rot
	states[0].scale[0] *= in.Scale[0]
	states[0].scale[1] *= in.Scale[1]
	states[0].active = true // 模板本体可能 inactive（Instantiate 后 SetActive(true) 语义）
	for ti, v := range in.actives {
		states[ti].active = v
	}
	for ti, sp := range in.sprites {
		states[ti].sprite = sp
	}
	for ti, c := range in.colors {
		states[ti].color = c
	}
	for ti, c := range in.matAdd {
		states[ti].matAdd = c
	}
	for ti, p := range in.palettes {
		states[ti].palette = p
	}
	for ti, o := range in.orders {
		states[ti].order = o
	}
	for ti, p := range in.pos {
		states[ti].pos = p
	}
	for ti, r := range in.rots {
		states[ti].rot = r
	}
	for ti, s := range in.scales {
		states[ti].scale = s
	}
	// 剪辑采样
	for _, p := range in.players {
		in.samplePlayer(p, states, beat)
	}
	for _, key := range in.layerOrder {
		if p := in.layers[key]; p != nil {
			in.samplePlayer(p, states, beat)
		}
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
		tint := st.color
		tint[3] *= st.matAlpha * st.matOpacity
		n := &t.as.Rig.Nodes[tn.RigIdx]
		if !actives[ti] || !st.renderOn {
			continue
		}
		if st.sprite != "" {
			if n.Mask || tint[3] > 0 {
				e := ExtraSprite{
					Sprite: st.sprite, World: world[ti], Z: z,
					Layer: n.Layer, Order: st.order,
					FlipX: st.flipX, FlipY: st.flipY, Tint: tint, MatColor: st.matColor,
					HueShift: st.matHueShift, LinearAdd: st.matLinearAdd,
					Doodle:       st.matDoodle,
					OutlineWidth: st.outlineWidth,
					Mapped:       n.Mapped, Mat: n.Mat,
					Mask: n.Mask, MaskIn: n.MaskIn,
					Add: st.matAdd, Blend: st.matBlend,
					Threshold: st.matThreshold, HasThreshold: st.hasMatThreshold,
					Progress: st.matProgress, HasProgress: st.hasMatProgress,
				}
				if in.hasGroup {
					e.HasGroup = true
					e.GroupKey = groupKey
					e.GroupLayer = in.groupLayer
					e.GroupOrder = in.groupOrder
					e.GroupZ = z
				}
				if pal, ok := in.palettes[ti]; ok {
					e.HasPalette = true
					e.Palette = pal
				}
				if st.hasPalette {
					e.HasPalette = true
					e.Palette = st.palette
				}
				scene.Queue(e)
			}
		}
		for _, bi := range t.meshBindings[ti] {
			if !scene.meshRenderable(bi) {
				continue
			}
			b := &t.as.Meshes.Bindings[bi]
			mt := scene.meshMaterialTint(b)
			if st.hasMatColor {
				mt = st.matColor
			}
			mt[3] *= st.matAlpha * st.matOpacity
			if mt[3] <= 0 {
				continue
			}
			order := b.Order
			if st.order != n.Order {
				order = st.order
			}
			scene.QueueMesh(ExtraMesh{
				Binding:    bi,
				World:      world[ti],
				Z:          z,
				Layer:      b.Layer,
				Order:      order,
				HasGroup:   in.hasGroup,
				GroupKey:   groupKey,
				GroupLayer: in.groupLayer,
				GroupOrder: in.groupOrder,
				GroupZ:     z,
				Tint:       mt,
			})
		}
	}
	return groupKey
}

func (in *Instance) samplePlayer(p *instPlayer, states []instNodeState, beat float64) {
	in.stepMachine(p, beat)
	if p.anim == nil {
		return
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

// NodeWorld 返回子树内节点（相对 path）在 baseWorld 下的世界变换。
// It mirrors the runtime TRS overrides used by Queue so script-driven anchors
// such as rotated grab points and ParticleSystem roots do not fall back to the
// prefab bind pose. Animation-clip sampling is still owned by Queue; callers
// should only use this for anchors whose transform is static except for script
// overrides.
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
		pos, rot, scale := n.Pos, n.RotZ, n.Scale
		if ti == 0 {
			pos = in.Offset
			rot += in.Rot
			scale[0] *= in.Scale[0]
			scale[1] *= in.Scale[1]
		}
		if p, ok := in.pos[ti]; ok {
			pos = p
		}
		if r, ok := in.rots[ti]; ok {
			rot = r
		}
		if s, ok := in.scales[ti]; ok {
			scale = s
		}
		aff = aff.Mul(TRS(pos[0], pos[1], rot, scale[0], scale[1]))
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
			case attr == "m_AnchoredPosition.x":
				// RectTransform curves in gameplay prefabs are authored as local
				// offsets, so instances need the same transform mapping as scenes.
				states[ti].pos[0] = v
			case attr == "m_AnchoredPosition.y":
				states[ti].pos[1] = v
			case strings.HasPrefix(attr, "m_Color."), strings.HasPrefix(attr, "m_fontColor."):
				ch := strings.TrimPrefix(attr, "m_Color.")
				ch = strings.TrimPrefix(ch, "m_fontColor.")
				switch ch {
				case "r":
					states[ti].color[0] = v
				case "g":
					states[ti].color[1] = v
				case "b":
					states[ti].color[2] = v
				case "a":
					states[ti].color[3] = v
				}
			case strings.HasPrefix(attr, "material._AddColor."):
				ch := strings.TrimPrefix(attr, "material._AddColor.")
				switch ch {
				case "r":
					states[ti].matAdd[0] = v
				case "g":
					states[ti].matAdd[1] = v
				case "b":
					states[ti].matAdd[2] = v
				case "a":
					states[ti].matAdd[3] = v
				}
			case strings.HasPrefix(attr, "material._Color."):
				states[ti].hasMatColor = true
				ch := strings.TrimPrefix(attr, "material._Color.")
				switch ch {
				case "r":
					states[ti].matColor[0] = v
				case "g":
					states[ti].matColor[1] = v
				case "b":
					states[ti].matColor[2] = v
				case "a":
					states[ti].matColor[3] = v
				}
			case attr == "material._Alpha":
				states[ti].matAlpha = v
			case attr == "material._Opacity":
				states[ti].matOpacity = v
			case attr == "material._Progress":
				states[ti].matProgress = v
				states[ti].hasMatProgress = true
			case attr == "material._HueShift":
				states[ti].matHueShift = v
			case strings.HasPrefix(attr, "material._BlendColor."):
				ch := strings.TrimPrefix(attr, "material._BlendColor.")
				switch ch {
				case "r":
					states[ti].matBlend[0] = v
				case "g":
					states[ti].matBlend[1] = v
				case "b":
					states[ti].matBlend[2] = v
				case "a":
					states[ti].matBlend[3] = v
				}
			case attr == "material._OutlineWidth":
				states[ti].outlineWidth = v
			case attr == "material._Threshold":
				states[ti].matThreshold = v
				states[ti].hasMatThreshold = true
			case strings.HasPrefix(attr, "material._ColorAlpha."):
				states[ti].hasPalette = true
				setPaletteChannel(&states[ti].palette.Alpha, strings.TrimPrefix(attr, "material._ColorAlpha."), v)
			case strings.HasPrefix(attr, "material._ColorBravo."):
				states[ti].hasPalette = true
				setPaletteChannel(&states[ti].palette.Fill, strings.TrimPrefix(attr, "material._ColorBravo."), v)
			case strings.HasPrefix(attr, "material._ColorDelta."):
				states[ti].hasPalette = true
				setPaletteChannel(&states[ti].palette.Outline, strings.TrimPrefix(attr, "material._ColorDelta."), v)
			}
		}
	}
}
