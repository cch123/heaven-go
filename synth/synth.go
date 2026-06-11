// Package synth 提供简单的 PCM 合成：demo 用它生成测试音轨与音效，
// 避免分发任何受版权保护的音频资产。
//
// 约定：内部以 float64 单声道、44100 Hz 处理，输出时编码为
// 16-bit little-endian 立体声 PCM（Ebitengine audio.Context 的默认格式）。
package synth

import (
	"encoding/binary"
	"math"
	"math/rand"
)

const SampleRate = 44100

// Track 是一条可叠加的单声道混音轨。
type Track struct {
	samples []float64
}

func NewTrack(durSec float64) *Track {
	return &Track{samples: make([]float64, int(durSec*SampleRate))}
}

// Add 把片段 s 以 gain 增益叠加到 at 秒处（线性混音，越界部分丢弃）。
func (t *Track) Add(at float64, s []float64, gain float64) {
	start := int(at * SampleRate)
	for i, v := range s {
		idx := start + i
		if idx < 0 || idx >= len(t.samples) {
			continue
		}
		t.samples[idx] += v * gain
	}
}

// PCM16Stereo 编码为 16-bit LE 立体声裸 PCM（双声道复制单声道）。
func (t *Track) PCM16Stereo() []byte {
	out := make([]byte, len(t.samples)*4)
	for i, v := range t.samples {
		s := int16(clamp(v, -1, 1) * 32767)
		binary.LittleEndian.PutUint16(out[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(out[i*4+2:], uint16(s))
	}
	return out
}

// WAV 编码为完整的 WAV 文件字节（RIFF 头 + PCM16 立体声）。
func (t *Track) WAV() []byte {
	pcm := t.PCM16Stereo()
	const (
		numChannels   = 2
		bitsPerSample = 16
	)
	byteRate := SampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8

	buf := make([]byte, 44+len(pcm))
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+len(pcm)))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16) // PCM chunk size
	binary.LittleEndian.PutUint16(buf[20:], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:], numChannels)
	binary.LittleEndian.PutUint32(buf[24:], SampleRate)
	binary.LittleEndian.PutUint32(buf[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:], bitsPerSample)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(len(pcm)))
	copy(buf[44:], pcm)
	return buf
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------- 乐器 / 音效 ----------

func render(dur float64, f func(t float64) float64) []float64 {
	n := int(dur * SampleRate)
	out := make([]float64, n)
	for i := range out {
		out[i] = f(float64(i) / SampleRate)
	}
	return out
}

// Kick 底鼓：正弦扫频 150Hz -> 45Hz，指数衰减。
func Kick() []float64 {
	phase := 0.0
	return render(0.14, func(t float64) float64 {
		freq := 45 + 105*math.Exp(-t*30)
		phase += 2 * math.Pi * freq / SampleRate
		return math.Sin(phase) * math.Exp(-t*18)
	})
}

// Hat 闭镲：白噪声短促衰减。
func Hat() []float64 {
	rng := rand.New(rand.NewSource(2))
	return render(0.05, func(t float64) float64 {
		return (rng.Float64()*2 - 1) * math.Exp(-t*120)
	})
}

// Beep 提示音：带起音/衰减包络的正弦。
func Beep(freq, dur float64) []float64 {
	return render(dur, func(t float64) float64 {
		env := math.Min(t/0.005, 1) * math.Exp(-t*10)
		return math.Sin(2*math.Pi*freq*t) * env
	})
}

// Woosh 投掷风声：噪声 + 升频正弦的混合，给击打点之前的"抛出"提示。
func Woosh() []float64 {
	rng := rand.New(rand.NewSource(7))
	phase := 0.0
	return render(0.25, func(t float64) float64 {
		env := math.Sin(math.Pi * t / 0.25) // 先起后落
		freq := 250 + 1400*t
		phase += 2 * math.Pi * freq / SampleRate
		noise := (rng.Float64()*2 - 1) * 0.6
		return (noise + math.Sin(phase)*0.4) * env
	})
}

// Punch 拳击命中：噪声爆发 + 低频厚度。
func Punch() []float64 {
	rng := rand.New(rand.NewSource(13))
	return render(0.16, func(t float64) float64 {
		noise := (rng.Float64()*2 - 1) * math.Exp(-t*45)
		thump := math.Sin(2*math.Pi*85*t) * math.Exp(-t*25)
		return noise*0.7 + thump*0.8
	})
}

// Ding 完美判定提示：双正弦泛音。
func Ding() []float64 {
	return render(0.22, func(t float64) float64 {
		env := math.Exp(-t * 14)
		return (math.Sin(2*math.Pi*1320*t) + 0.5*math.Sin(2*math.Pi*1980*t)) * env * 0.5
	})
}

// Buzz 失误提示：方波低鸣。
func Buzz() []float64 {
	return render(0.18, func(t float64) float64 {
		s := math.Sin(2 * math.Pi * 110 * t)
		if s > 0 {
			s = 1
		} else {
			s = -1
		}
		return s * math.Exp(-t*12) * 0.35
	})
}
