// Package kart 是提取资产（kmdata JSON）的运行时：
// 图集切片绘制、骨架（rig）变换合成、AnimationClip 采样（Hermite 插值 +
// 阶跃 sprite 换帧 + FlipX 浮点曲线）。
//
// 坐标约定：游戏逻辑使用 Unity 单位空间（y 向上），由 proj 仿射投影到屏幕。
package kart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"hsdemo/kmdata"
)

// ---------- 仿射 ----------

// Aff 是 2D 仿射变换：p' = (A*x + C*y + Tx, B*x + D*y + Ty)。
type Aff struct{ A, B, C, D, Tx, Ty float64 }

func Identity() Aff { return Aff{A: 1, D: 1} }

// Apply 变换一个点。
func (m Aff) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.Tx, m.B*x + m.D*y + m.Ty
}

// Mul 返回 m ∘ n（先施加 n，再施加 m）。
func (m Aff) Mul(n Aff) Aff {
	return Aff{
		A: m.A*n.A + m.C*n.B, B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D, D: m.B*n.C + m.D*n.D,
		Tx: m.A*n.Tx + m.C*n.Ty + m.Tx, Ty: m.B*n.Tx + m.D*n.Ty + m.Ty,
	}
}

func Translate(x, y float64) Aff { return Aff{A: 1, D: 1, Tx: x, Ty: y} }
func Scale(x, y float64) Aff     { return Aff{A: x, D: y} }
func Rotate(r float64) Aff {
	s, c := math.Sin(r), math.Cos(r)
	return Aff{A: c, B: s, C: -s, D: c}
}

// TRS 组合 平移*旋转*缩放（骨架节点的局部变换）。
func TRS(x, y, rot, sx, sy float64) Aff {
	s, c := math.Sin(rot), math.Cos(rot)
	return Aff{A: c * sx, B: s * sx, C: -s * sy, D: c * sy, Tx: x, Ty: y}
}

func (m Aff) GeoM() ebiten.GeoM {
	var g ebiten.GeoM
	g.SetElement(0, 0, m.A)
	g.SetElement(0, 1, m.C)
	g.SetElement(0, 2, m.Tx)
	g.SetElement(1, 0, m.B)
	g.SetElement(1, 1, m.D)
	g.SetElement(1, 2, m.Ty)
	return g
}

// ---------- 资产 ----------

type Assets struct {
	Sheet   kmdata.Sheet
	Atlas   *ebiten.Image   // 单图集（legacy karateman 格式）
	Atlases []*ebiten.Image // 多图集（scene 格式）
	Rig     kmdata.Rig      // karateman: rig.json；scene 游戏: scene.json
	Roles   kmdata.Roles    // scene 游戏: 脚本字段 → 节点 path
	Extra   kmdata.Extra    // scene 游戏: 扩展序列化数据（可选）
	Stage   kmdata.Stage
	Anims   map[string]*kmdata.Anim
	// Sounds: 文件主名（无扩展名）→ 解码后的 16-bit LE 立体声裸 PCM
	Sounds map[string][]byte

	// AnimatorController 状态机（controllers.json / animators.json，可选）
	Controllers map[string]kmdata.Controller
	Animators   kmdata.Animators
	// TMP 世界文本（texts.json + fonts/，可选）
	Texts []kmdata.TextNode
	Fonts map[string][]byte // 字体文件名 → 原始字节

	subs map[string]*ebiten.Image
}

// Load 读取提取器输出目录；音效解码到 audioRate 采样率。
// 自动识别两种布局：karateman（rig.json + stage.json + 单图集）
// 与 scene 游戏（scene.json + roles.json + 多图集）。
func Load(dir string, audioRate int) (*Assets, error) {
	a := &Assets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string][]byte{},
		subs:   map[string]*ebiten.Image{},
	}
	if err := readJSON(filepath.Join(dir, "sprites.json"), &a.Sheet); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "anims.json"), &a.Anims); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "scene.json")); err == nil {
		// scene 布局
		if err := readJSON(filepath.Join(dir, "scene.json"), &a.Rig); err != nil {
			return nil, err
		}
		if err := readJSON(filepath.Join(dir, "roles.json"), &a.Roles); err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, "extra.json")); err == nil {
			if err := readJSON(filepath.Join(dir, "extra.json"), &a.Extra); err != nil {
				return nil, err
			}
		}
		for _, name := range a.Sheet.Atlases {
			img, err := loadPNG(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			a.Atlases = append(a.Atlases, img)
		}
		// 可选：AnimatorController 状态机与 TMP 文本
		if _, err := os.Stat(filepath.Join(dir, "controllers.json")); err == nil {
			if err := readJSON(filepath.Join(dir, "controllers.json"), &a.Controllers); err != nil {
				return nil, err
			}
			if err := readJSON(filepath.Join(dir, "animators.json"), &a.Animators); err != nil {
				return nil, err
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "texts.json")); err == nil {
			if err := readJSON(filepath.Join(dir, "texts.json"), &a.Texts); err != nil {
				return nil, err
			}
			a.Fonts = map[string][]byte{}
			fonts, _ := filepath.Glob(filepath.Join(dir, "fonts", "*"))
			for _, p := range fonts {
				b, err := os.ReadFile(p)
				if err != nil {
					return nil, err
				}
				a.Fonts[filepath.Base(p)] = b
			}
		}
	} else {
		// karateman 布局
		if err := readJSON(filepath.Join(dir, "rig.json"), &a.Rig); err != nil {
			return nil, err
		}
		if err := readJSON(filepath.Join(dir, "stage.json"), &a.Stage); err != nil {
			return nil, err
		}
		img, err := loadPNG(filepath.Join(dir, a.Sheet.Atlas))
		if err != nil {
			return nil, err
		}
		a.Atlas = img
	}

	sndRoot := filepath.Join(dir, "sounds")
	if _, err := os.Stat(sndRoot); err == nil {
		err = filepath.WalkDir(sndRoot, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".ogg" && ext != ".wav" {
				return nil // .DS_Store 等杂物
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			pcm, err := decodePCM(raw, ext, audioRate)
			if err != nil {
				return fmt.Errorf("decode %s: %w", p, err)
			}
			rel, _ := filepath.Rel(sndRoot, p)
			key := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(p))
			a.Sounds[key] = pcm
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

func loadPNG(p string) (*ebiten.Image, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", p, err)
	}
	return ebiten.NewImageFromImage(img), nil
}

func readJSON(p string, v any) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("%s: %w（先运行 go run ./cmd/extract）", p, err)
	}
	return json.Unmarshal(b, v)
}

// DecodePCM 把 ogg/wav 字节解码为 16-bit LE 立体声裸 PCM（公共音效加载用）。
func DecodePCM(raw []byte, ext string, rate int) ([]byte, error) {
	return decodePCM(raw, ext, rate)
}

func decodePCM(raw []byte, ext string, rate int) ([]byte, error) {
	br := bytes.NewReader(raw)
	var (
		s   io.Reader
		err error
	)
	switch strings.ToLower(ext) {
	case ".ogg":
		s, err = vorbis.DecodeWithSampleRate(rate, br)
	case ".wav":
		s, err = wav.DecodeWithSampleRate(rate, br)
	default:
		return nil, fmt.Errorf("unsupported sound ext %q", ext)
	}
	if err != nil {
		return nil, err
	}
	return io.ReadAll(s)
}

// ResamplePCM 对 16-bit LE 立体声 PCM 做线性插值重采样实现变调：
// pitch > 1 音高升高（时长缩短）。用于一次性音效（MultiSound 的
// pitch 参数、原版 SoundByte 的随机变调）。
func ResamplePCM(pcm []byte, pitch float64) []byte {
	if pitch == 1 || len(pcm) < 8 {
		return pcm
	}
	frames := len(pcm) / 4
	outFrames := int(float64(frames) / pitch)
	out := make([]byte, outFrames*4)
	for i := 0; i < outFrames; i++ {
		srcPos := float64(i) * pitch
		j := int(srcPos)
		frac := srcPos - float64(j)
		if j >= frames-1 {
			j, frac = frames-2, 1
		}
		for ch := 0; ch < 2; ch++ {
			a := int16(uint16(pcm[j*4+ch*2]) | uint16(pcm[j*4+ch*2+1])<<8)
			b := int16(uint16(pcm[(j+1)*4+ch*2]) | uint16(pcm[(j+1)*4+ch*2+1])<<8)
			v := int16(float64(a) + (float64(b)-float64(a))*frac)
			out[i*4+ch*2] = byte(v)
			out[i*4+ch*2+1] = byte(uint16(v) >> 8)
		}
	}
	return out
}

// RegisterSprite 注册一张运行时生成的贴图为切片（如 TMP 文本渲染结果），
// pivotX/pivotY 为归一化枢轴（Unity 约定 y 从底边算）。
func (a *Assets) RegisterSprite(name string, img *ebiten.Image, ppu, pivotX, pivotY float64) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	a.Sheet.Sprites[name] = kmdata.SpriteInfo{
		X: 0, Y: 0, W: w, H: h, PivotX: pivotX, PivotY: pivotY, PPU: ppu,
	}
	a.subs[name] = img
}

// NodeIndex 返回 path 的首个节点下标。
func (a *Assets) NodeIndex(path string) (int, bool) {
	for i := range a.Rig.Nodes {
		if a.Rig.Nodes[i].Path == path {
			return i, true
		}
	}
	return -1, false
}

// ClipPose 是对单个节点采样剪辑得到的姿态（Has* 标记该剪辑是否驱动了对应通道）。
type ClipPose struct {
	Pos       [2]float64
	HasPos    [2]bool
	Scale     [2]float64
	HasScale  [2]bool
	RotDeg    float64
	HasRot    bool
	Sprite    string
	HasSprite bool
}

// SampleClipNode 在剪辑时间 at（秒）对节点 path 采样（模块自管的模板实例用，
// 如 meatGrinder 的肉块：多实例共用同一剪辑，不经场景树播放器）。
func SampleClipNode(a *kmdata.Anim, path string, at float64) ClipPose {
	var p ClipPose
	if c, ok := a.Pos[path]; ok {
		if len(c.X) > 0 {
			p.Pos[0], p.HasPos[0] = evalKeys(c.X, at), true
		}
		if len(c.Y) > 0 {
			p.Pos[1], p.HasPos[1] = evalKeys(c.Y, at), true
		}
	}
	if c, ok := a.Scale[path]; ok {
		if len(c.X) > 0 {
			p.Scale[0], p.HasScale[0] = evalKeys(c.X, at), true
		}
		if len(c.Y) > 0 {
			p.Scale[1], p.HasScale[1] = evalKeys(c.Y, at), true
		}
	}
	if keys, ok := a.Euler[path]; ok && len(keys) > 0 {
		p.RotDeg, p.HasRot = evalKeys(keys, at), true
	}
	if keys, ok := a.Sprites[path]; ok {
		if name, ok := sampleSwap(keys, at); ok {
			p.Sprite, p.HasSprite = name, true
		}
	}
	return p
}

// Sub 返回切片的图集子图（带缓存）。
func (a *Assets) Sub(name string) *ebiten.Image {
	if img, ok := a.subs[name]; ok {
		return img
	}
	sp, ok := a.Sheet.Sprites[name]
	if !ok {
		return nil
	}
	atlas := a.Atlas
	if len(a.Atlases) > 0 {
		if sp.Atlas < 0 || sp.Atlas >= len(a.Atlases) {
			return nil
		}
		atlas = a.Atlases[sp.Atlas]
	}
	img := atlas.SubImage(image.Rect(sp.X, sp.Y, sp.X+sp.W, sp.Y+sp.H)).(*ebiten.Image)
	a.subs[name] = img
	return img
}

func (a *Assets) ppuOf(sp kmdata.SpriteInfo) float64 {
	if sp.PPU > 0 {
		return sp.PPU
	}
	return a.Sheet.PPU
}

// DrawSprite 在单位空间变换 world 下绘制切片，proj 为单位空间→屏幕投影。
func (a *Assets) DrawSprite(dst *ebiten.Image, name string, world, proj Aff, flipX bool, alpha float32) {
	a.DrawSpriteTint(dst, name, world, proj, flipX, [4]float64{1, 1, 1, float64(alpha)})
}

// DrawSpriteTint 同 DrawSprite，但带 RGBA 调色（SpriteRenderer m_Color 语义）。
func (a *Assets) DrawSpriteTint(dst *ebiten.Image, name string, world, proj Aff, flipX bool, tint [4]float64) {
	img := a.Sub(name)
	if img == nil {
		return
	}
	sp := a.Sheet.Sprites[name]
	ppu := a.ppuOf(sp)
	f := 1.0
	if flipX {
		f = -1
	}
	// 像素空间 → 单位空间：枢轴移到原点（注意 Unity 枢轴 y 从底边算），再缩放并翻转 y
	local := Scale(f/ppu, -1/ppu).
		Mul(Translate(-sp.PivotX*float64(sp.W), -(1-sp.PivotY)*float64(sp.H)))
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM = proj.Mul(world).Mul(local).GeoM()
	op.ColorScale.Scale(float32(tint[0]), float32(tint[1]), float32(tint[2]), float32(tint[3]))
	dst.DrawImage(img, op)
}

// SpriteOpts 是 DrawSpriteOpts 的渲染选项。
type SpriteOpts struct {
	FlipX, FlipY bool
	Tint         [4]float64 // 零值视为白色
	Stretch      [2]float64 // 非零时拉伸到该尺寸（unit，对应 SpriteRenderer sliced/tiled 的 m_Size）
}

// DrawSpriteOpts 按选项绘制切片。Stretch 非零时按 SpriteRenderer
// sliced 语义渲染：有 border 的切片走九宫格（端帽保持原始像素尺寸，
// 仅拉伸中段），无 border 则整体拉伸。
func (a *Assets) DrawSpriteOpts(dst *ebiten.Image, name string, world, proj Aff, o SpriteOpts) {
	img := a.Sub(name)
	if img == nil {
		return
	}
	sp := a.Sheet.Sprites[name]
	ppu := a.ppuOf(sp)
	fx, fy := 1.0, 1.0
	if o.FlipX {
		fx = -1
	}
	if o.FlipY {
		fy = -1
	}
	tint := o.Tint
	if tint == [4]float64{} {
		tint = [4]float64{1, 1, 1, 1}
	}

	stretch := o.Stretch[0] != 0 && o.Stretch[1] != 0
	if !stretch {
		local := Scale(fx/ppu, -fy/ppu).
			Mul(Translate(-sp.PivotX*float64(sp.W), -(1-sp.PivotY)*float64(sp.H)))
		drawTinted(dst, img, proj.Mul(world).Mul(local), tint)
		return
	}

	// 目标矩形（像素），枢轴按矩形比例锚定（Unity sliced 语义）
	tw, th := math.Abs(o.Stretch[0])*ppu, math.Abs(o.Stretch[1])*ppu
	base := Scale(fx/ppu, -fy/ppu).
		Mul(Translate(-sp.PivotX*tw, -(1-sp.PivotY)*th))

	bl, bb, br, bt := sp.Border[0], sp.Border[1], sp.Border[2], sp.Border[3]
	if bl+bb+br+bt == 0 { // 无 border：整体拉伸
		local := base.Mul(Scale(tw/float64(sp.W), th/float64(sp.H)))
		drawTinted(dst, img, proj.Mul(world).Mul(local), tint)
		return
	}
	// 端帽超过目标尺寸时按比例压缩（Unity 同语义）
	if s := tw / (bl + br); bl+br > 0 && s < 1 {
		bl, br = bl*s, br*s
	}
	if s := th / (bt + bb); bt+bb > 0 && s < 1 {
		bt, bb = bt*s, bb*s
	}

	W, H := float64(sp.W), float64(sp.H)
	sxs := [4]float64{0, sp.Border[0], W - sp.Border[2], W} // 源（左→右）
	sys := [4]float64{0, sp.Border[3], H - sp.Border[1], H} // 源（上→下：上边距 w、下边距 y）
	txs := [4]float64{0, bl, tw - br, tw}
	tys := [4]float64{0, bt, th - bb, th}

	for ix := 0; ix < 3; ix++ {
		for iy := 0; iy < 3; iy++ {
			sw, sh := sxs[ix+1]-sxs[ix], sys[iy+1]-sys[iy]
			dw, dh := txs[ix+1]-txs[ix], tys[iy+1]-tys[iy]
			if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
				continue
			}
			sub := img.SubImage(image.Rect(
				sp.X+int(sxs[ix]), sp.Y+int(sys[iy]),
				sp.X+int(sxs[ix+1]), sp.Y+int(sys[iy+1]),
			)).(*ebiten.Image)
			local := base.
				Mul(Translate(txs[ix], tys[iy])).
				Mul(Scale(dw/sw, dh/sh))
			drawTinted(dst, sub, proj.Mul(world).Mul(local), tint)
		}
	}
}

func drawTinted(dst, img *ebiten.Image, m Aff, tint [4]float64) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM = m.GeoM()
	op.ColorScale.Scale(float32(tint[0]), float32(tint[1]), float32(tint[2]), float32(tint[3]))
	dst.DrawImage(img, op)
}

// ---------- 曲线采样 ----------

// evalKeys 对 Hermite 关键帧曲线求值；|斜率| ≥ StepSlope 视为阶跃。
func evalKeys(keys []kmdata.Key, t float64) float64 {
	if len(keys) == 0 {
		return 0
	}
	if t <= keys[0].T {
		return keys[0].V
	}
	last := keys[len(keys)-1]
	if t >= last.T {
		return last.V
	}
	i := 0
	for i+1 < len(keys) && keys[i+1].T <= t {
		i++
	}
	k0, k1 := keys[i], keys[i+1]
	if math.Abs(k0.O) >= kmdata.StepSlope || math.Abs(k1.I) >= kmdata.StepSlope {
		return k0.V
	}
	h := k1.T - k0.T
	if h <= 0 {
		return k0.V
	}
	s := (t - k0.T) / h
	s2, s3 := s*s, s*s*s
	return (2*s3-3*s2+1)*k0.V + (s3-2*s2+s)*h*k0.O + (-2*s3+3*s2)*k1.V + (s3-s2)*h*k1.I
}

func sampleSwap(keys []kmdata.SwapKey, t float64) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	cur := keys[0].Name
	for _, k := range keys[1:] {
		if k.T > t {
			break
		}
		cur = k.Name
	}
	return cur, true
}

// ---------- 骨架实例 ----------

type nodeState struct {
	pos    [2]float64
	rot    float64
	scale  [2]float64
	sprite string
	flipX  bool
}

// RigInst 是一个可播放动画的骨架实例。根节点的 prefab 位移被清零，
// 摆放位置完全由 Draw 的 proj 决定。
type RigInst struct {
	as     *Assets
	byPath map[string]int
	state  []nodeState
	world  []Aff

	anim  *kmdata.Anim
	start float64
}

func NewRig(as *Assets) *RigInst {
	r := &RigInst{
		as:     as,
		byPath: map[string]int{},
		state:  make([]nodeState, len(as.Rig.Nodes)),
		world:  make([]Aff, len(as.Rig.Nodes)),
	}
	for i, n := range as.Rig.Nodes {
		r.byPath[n.Path] = i
	}
	r.reset()
	return r
}

func (r *RigInst) reset() {
	for i, n := range r.as.Rig.Nodes {
		st := nodeState{pos: n.Pos, rot: n.RotZ, scale: n.Scale, sprite: n.Sprite, flipX: n.FlipX}
		if n.Parent < 0 {
			st.pos = [2]float64{0, 0} // 摆放交给 proj
		}
		r.state[i] = st
	}
}

// Play 从歌曲时间 t 起播放动画。
func (r *RigInst) Play(name string, t float64) {
	if a, ok := r.as.Anims[name]; ok {
		r.anim, r.start = a, t
	}
}

// Playing 报告当前动画在时刻 t 是否仍在播放（循环动画恒为 true）。
func (r *RigInst) Playing(t float64) bool {
	return r.anim != nil && (r.anim.Loop || t-r.start < r.anim.Duration)
}

// Sample 按歌曲时间 t 采样动画并更新世界变换。
func (r *RigInst) Sample(t float64) {
	r.reset()
	if r.anim != nil {
		at := t - r.start
		if r.anim.Loop && r.anim.Duration > 0 {
			at = math.Mod(at, r.anim.Duration)
		} else if at > r.anim.Duration {
			at = r.anim.Duration // 非循环动画保持末帧（下个 Bop 会接管）
		}
		r.apply(at)
	}
	for i, n := range r.as.Rig.Nodes {
		st := &r.state[i]
		local := TRS(st.pos[0], st.pos[1], st.rot, st.scale[0], st.scale[1])
		if n.Parent < 0 {
			r.world[i] = local
		} else {
			r.world[i] = r.world[n.Parent].Mul(local)
		}
	}
}

func (r *RigInst) apply(at float64) {
	a := r.anim
	for path, c := range a.Pos {
		if i, ok := r.byPath[path]; ok {
			if len(c.X) > 0 {
				r.state[i].pos[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				r.state[i].pos[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Euler {
		if i, ok := r.byPath[path]; ok && len(keys) > 0 {
			r.state[i].rot = evalKeys(keys, at) * math.Pi / 180
		}
	}
	for path, c := range a.Scale {
		if i, ok := r.byPath[path]; ok {
			if len(c.X) > 0 {
				r.state[i].scale[0] = evalKeys(c.X, at)
			}
			if len(c.Y) > 0 {
				r.state[i].scale[1] = evalKeys(c.Y, at)
			}
		}
	}
	for path, keys := range a.Sprites {
		if i, ok := r.byPath[path]; ok {
			if name, ok := sampleSwap(keys, at); ok {
				r.state[i].sprite = name // 空名 = 该帧隐藏
			}
		}
	}
	for path, attrs := range a.Floats {
		i, ok := r.byPath[path]
		if !ok {
			continue
		}
		if keys, ok := attrs["m_FlipX"]; ok && len(keys) > 0 {
			r.state[i].flipX = evalKeys(keys, at) > 0.5
		}
	}
}

// Draw 按 sortingOrder 绘制（需先 Sample）。
func (r *RigInst) Draw(dst *ebiten.Image, proj Aff) {
	type item struct{ idx, order int }
	items := make([]item, 0, len(r.state))
	for i, n := range r.as.Rig.Nodes {
		if n.Hidden || r.state[i].sprite == "" {
			continue
		}
		items = append(items, item{i, n.Order})
	}
	// 稳定插入排序（节点数 18，开销可忽略）
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j-1].order > items[j].order; j-- {
			items[j-1], items[j] = items[j], items[j-1]
		}
	}
	for _, it := range items {
		r.as.DrawSprite(dst, r.state[it.idx].sprite, r.world[it.idx], proj, r.state[it.idx].flipX, 1)
	}
}

// BBox 返回默认姿态的单位空间包围盒（minX, minY, maxX, maxY）。
func (r *RigInst) BBox() (float64, float64, float64, float64) {
	saveAnim := r.anim
	r.anim = nil
	r.Sample(0)
	r.anim = saveAnim

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, n := range r.as.Rig.Nodes {
		st := r.state[i]
		sp, ok := r.as.Sheet.Sprites[st.sprite]
		if !ok || n.Hidden {
			continue
		}
		w, h := float64(sp.W)/r.as.Sheet.PPU, float64(sp.H)/r.as.Sheet.PPU
		for _, c := range [][2]float64{
			{-sp.PivotX * w, -sp.PivotY * h}, {(1 - sp.PivotX) * w, -sp.PivotY * h},
			{-sp.PivotX * w, (1 - sp.PivotY) * h}, {(1 - sp.PivotX) * w, (1 - sp.PivotY) * h},
		} {
			m := r.world[i]
			x := m.A*c[0] + m.C*c[1] + m.Tx
			y := m.B*c[0] + m.D*c[1] + m.Ty
			minX, maxX = math.Min(minX, x), math.Max(maxX, x)
			minY, maxY = math.Min(minY, y), math.Max(maxY, y)
		}
	}
	return minX, minY, maxX, maxY
}
