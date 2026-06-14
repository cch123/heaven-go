// postfx.go：ppe/*（Post Processing Effects）的移植。
//
// HS 用 Unity PostProcessing v2（PPv2）+ X-PostProcessing 实现屏幕后处理，
// 事件（VFXManager.cs）按拍序折叠：prog>=0 即应用（钳 [0,1]），后续事件覆盖、
// 终值持久——与 vfx/move camera 相同的语义。
//
// 这里把游戏画面渲染到离屏帧，再经 Kage shader 链复刻 PPv2 公式：
//
//	PixelizeQuad（BeforeStack，UV 网格吸附）
//	→ Lens Distortion（UV 重映射）→ Chromatic Aberration（3 段光谱采样）
//	→ Bloom 叠加（阈值软膝 + 高斯模糊，简化为 1/4 分辨率两轮）
//	→ Vignette（classic 模式）→ Grain（hash 噪声近似胶片颗粒）
//	→ Color Grading LDR（LMS 白平衡/滤色/色相/饱和/亮度/LogC 对比度）
//
// 已知简化（详见 README）：bloom 用固定两轮 1/4 分辨率模糊近似 PPv2 的
// mip 金字塔；grain 用 hash 噪声近似烘焙噪声纹理；anamorphicRatio 未实现
// （全部关卡取 0）；technicolor 未实现（全部关卡未启用）。
package engine

import (
	"log"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

type fxEvt struct {
	beat, length float64
	data         map[string]any
}

// postFX 收集 ppe 事件并执行后处理链。
type postFX struct {
	evts map[string][]fxEvt // kind（vignette/cabb/...）→ 按拍排序

	uber      *ebiten.Shader
	preShader *ebiten.Shader // bloom 阈值预滤
	blur      *ebiten.Shader // 可分离高斯（方向作 uniform）

	frame     *ebiten.Image // 全分辨率游戏画面
	bloomFull *ebiten.Image // 全分辨率 bloom 结果（关闭时为黑）
	q1, q2    *ebiten.Image // 1/4 分辨率工作缓冲
}

func (fx *postFX) add(e *riq.Entity) {
	kind := e.Datamodel[len("ppe/"):]
	switch kind {
	case "vignette", "cabb", "bloom", "lensD", "grain", "colorGrading", "pixelQuad":
		if fx.evts == nil {
			fx.evts = map[string][]fxEvt{}
		}
		fx.evts[kind] = append(fx.evts[kind], fxEvt{e.Beat, e.Length, e.Data})
	default:
		log.Printf("engine: ppe/%s 未实现，跳过（出现时需补 postfx.go）", kind)
	}
}

func (fx *postFX) reset() { fx.evts = nil }

func (fx *postFX) active() bool { return len(fx.evts) > 0 }

func (fx *postFX) sortAll() {
	for _, list := range fx.evts {
		sort.Slice(list, func(i, j int) bool { return list[i].beat < list[j].beat })
	}
}
