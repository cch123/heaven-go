package engine

import (
	"math"
	"sort"
)

// ---------- 时间轴 / 判定服务（Ctx 转发到这里） ----------

func (a *App) at(beat float64, fn func()) {
	// 运行期插入需保序：找到插入点
	i := sort.Search(len(a.actions), func(i int) bool { return a.actions[i].beat > beat })
	a.actions = append(a.actions, beatAction{})
	copy(a.actions[i+1:], a.actions[i:])
	a.actions[i] = beatAction{beat, fn}
	if i < a.actIdx {
		a.actIdx++ // 插到已执行区前面（理论上不应发生），保持指针不回退
	}
}

func (a *App) scheduleInput(beat float64, release bool, action int, onHit func(state float64, j Judgment), onMiss func()) {
	a.scheduleInputCond(beat, release, action, nil, onHit, onMiss)
}

func (a *App) scheduleInputCond(beat float64, release bool, action int, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) {
	a.scheduleInputFull(beat, release, action, false, canHit, onHit, onMiss)
}

func (a *App) scheduleInputNoScore(beat float64, release bool, action int, onHit func(state float64, j Judgment), onMiss func()) {
	a.scheduleInputFull(beat, release, action, true, nil, onHit, onMiss)
}

func (a *App) scheduleInputFull(beat float64, release bool, action int, noScore bool, canHit func() bool, onHit func(state float64, j Judgment), onMiss func()) {
	weight, category := a.resultSectionAt(beat)
	a.inputs = append(a.inputs, &Input{
		Beat: beat, hitT: a.bm.BeatToTime(beat), Release: release, Action: action,
		Weight: weight, Category: category, NoScore: noScore, OnHit: onHit, OnMiss: onMiss,
		CanHit: canHit,
	})
}

func (a *App) resultSectionAt(beat float64) (float64, int) {
	weight, category := 1.0, 0
	if a.bm == nil {
		return weight, category
	}
	for _, s := range a.bm.Sections {
		if s.Beat > beat {
			break
		}
		weight, category = s.Weight, s.Category
	}
	return weight, category
}

func (a *App) recordInputScore(in *Input, accuracy float64) {
	if in.Weight <= 0 {
		return
	}
	a.scores = append(a.scores, resultScoreInput{
		Beat: in.Beat, Accuracy: math.Max(0, math.Min(1, accuracy)),
		Weight: in.Weight, Category: in.Category,
	})
}

func (a *App) recordMissScore(beat float64) {
	weight, category := a.resultSectionAt(beat)
	if weight <= 0 {
		return
	}
	a.scores = append(a.scores, resultScoreInput{
		Beat: beat, Accuracy: 0, Weight: weight, Category: category,
	})
}
