package engine

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestKageCompile 验证三个后处理 shader 能通过 Kage 编译。
func TestKageCompile(t *testing.T) {
	for name, src := range map[string]string{
		"uber": uberKage, "bloomPre": bloomPreKage, "blur": blurKage,
	} {
		if _, err := ebiten.NewShader([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestWhiteBalanceNeutral 中性参数应得到 ≈1 的系数。
func TestWhiteBalanceNeutral(t *testing.T) {
	r, g, b := whiteBalance(0, 0)
	for _, v := range []float64{r, g, b} {
		if v < 0.95 || v > 1.05 {
			t.Errorf("中性白平衡系数偏离 1: %v %v %v", r, g, b)
		}
	}
}
