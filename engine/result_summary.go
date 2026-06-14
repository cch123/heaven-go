package engine

import (
	"fmt"
	"math"
	"sort"
)

func (a *App) buildResultSummary() resultSummary {
	totalWeight, weightedScore := 0.0, 0.0
	noMiss, perfect := len(a.scores) > 0, len(a.scores) > 0
	for _, in := range a.scores {
		totalWeight += in.Weight
		weightedScore += math.Max(0, math.Min(1, in.Accuracy)) * in.Weight
		if in.Accuracy < rankOkThreshold {
			noMiss = false
		}
		if in.Accuracy < 1 {
			perfect = false
		}
	}
	score := 0.0
	if totalWeight > 0 {
		score = weightedScore / totalWeight
	}
	rank := resultRankHi
	suffix := "hi"
	switch {
	case score < rankOkThreshold:
		rank, suffix = resultRankNg, "ng"
	case score < rankHiThreshold:
		rank, suffix = resultRankOk, "ok"
	}

	res := resultSummary{
		Score: score, Rank: rank, Header: a.resultProp("resultcaption"),
		NoMiss: noMiss, Perfect: perfect, Star: a.starGot,
	}
	cats := a.resultCategories()
	catScores := a.resultCategoryScores()
	if len(cats) <= 1 {
		res.Message0 = a.resultProp("resultcommon_" + suffix)
		return res
	}

	switch rank {
	case resultRankOk:
		best, bestScore := cats[0], -1.0
		for _, cat := range cats {
			if catScores[cat] > bestScore {
				best, bestScore = cat, catScores[cat]
			}
		}
		if bestScore >= rankHiThreshold {
			res.SubRank = true
			res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_hi", best))
		} else {
			res.Message0 = a.resultProp("resultcommon_ok")
		}
	case resultRankNg:
		first, second := twoExtremeCategories(cats, catScores, true)
		res.TwoMessage = catScores[second] < rankOkThreshold
		res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_ng", first))
		res.Message1 = res.Message0
		res.Message2 = resultSecondMessage(a.resultProp(fmt.Sprintf("resultcat%d_ng", second)))
	case resultRankHi:
		first, second := twoExtremeCategories(cats, catScores, false)
		res.TwoMessage = catScores[second] >= rankHiThreshold
		res.Message0 = a.resultProp(fmt.Sprintf("resultcat%d_hi", first))
		res.Message1 = res.Message0
		res.Message2 = resultSecondMessage(a.resultProp(fmt.Sprintf("resultcat%d_hi", second)))
	}
	return res
}

func (a *App) resultCategories() []int {
	seen := map[int]bool{}
	var cats []int
	if a.bm != nil && len(a.bm.Sections) > 0 {
		for _, s := range a.bm.Sections {
			if !seen[s.Category] {
				seen[s.Category] = true
				cats = append(cats, s.Category)
			}
		}
	}
	for _, in := range a.scores {
		if !seen[in.Category] {
			seen[in.Category] = true
			cats = append(cats, in.Category)
		}
	}
	if len(cats) == 0 {
		cats = []int{0}
	}
	sort.Ints(cats)
	return cats
}

func (a *App) resultCategoryScores() map[int]float64 {
	type bucket struct{ score, weight float64 }
	buckets := map[int]bucket{}
	for _, in := range a.scores {
		b := buckets[in.Category]
		b.score += math.Max(0, math.Min(1, in.Accuracy)) * in.Weight
		b.weight += in.Weight
		buckets[in.Category] = b
	}
	out := map[int]float64{}
	for _, cat := range a.resultCategories() {
		if b := buckets[cat]; b.weight > 0 {
			out[cat] = b.score / b.weight
		} else {
			out[cat] = 0
		}
	}
	return out
}

func twoExtremeCategories(cats []int, scores map[int]float64, lowest bool) (int, int) {
	first, second := cats[0], cats[0]
	firstScore, secondScore := scores[first], scores[first]
	for i, cat := range cats {
		score := scores[cat]
		if i == 0 || (lowest && score < firstScore) || (!lowest && score > firstScore) {
			second, secondScore = first, firstScore
			first, firstScore = cat, score
			continue
		}
		if second == first || (lowest && score < secondScore) || (!lowest && score > secondScore) {
			second, secondScore = cat, score
		}
	}
	return first, second
}
