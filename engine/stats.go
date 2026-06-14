package engine

import "hsdemo/riq"

type Stats struct {
	Inputs   int
	Actions  int
	EndBeat  float64
	StarBeat float64
	Unported []string
}

// LoadStats 返回当前谱面的装载摘要。
func (a *App) LoadStats() Stats {
	return Stats{
		Inputs: len(a.inputs), Actions: len(a.actions),
		EndBeat: a.endBeat, StarBeat: a.starBeat, Unported: a.unported,
	}
}

// BeatNow 返回当前歌曲节拍（录制/验证工具用）。
func (a *App) BeatNow() float64 {
	if a.cond == nil {
		return 0
	}
	return a.cond.Beat()
}

// RunCounts 返回当前判定计数 ace/just/ng/miss/whiff（验证工具用）。
func (a *App) RunCounts() (int, int, int, int, int) {
	return a.aces, a.justs, a.ngs, a.misses, a.whiffs
}

// Finished 报告是否已进入结算画面。
func (a *App) Finished() bool { return a.state == stateResult }

// ---------- 参数辅助 ----------

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func colorParam(e *riq.Entity, key string) [4]float64 {
	out := [4]float64{}
	if m, ok := e.Data[key].(map[string]any); ok {
		get := func(k string) float64 {
			if f, ok := m[k].(float64); ok {
				return f
			}
			return 0
		}
		out = [4]float64{get("r"), get("g"), get("b"), get("a")}
	}
	return out
}
