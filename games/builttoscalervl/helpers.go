package builttoscalervl

import (
	"math"

	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	actionPrimary = 0
	actionAlt     = 1

	dirLeft  = 0
	dirRight = 1

	targetOuterLeft = iota
	targetFirst
	targetSecond
	targetThird
	targetFourth
	targetOuterRight

	widgetSeekBeats = 10.0
)

var curveMap = map[[2]int]int{
	{-1, 0}: 0,
	{0, 1}:  2, {1, 0}: 2,
	{1, 2}: 3, {2, 1}: 3,
	{2, 3}: 5, {3, 2}: 5,
	{4, 3}:  7,
	{0, 0}:  9,
	{1, 1}:  10,
	{2, 2}:  11,
	{3, 3}:  13,
	{-1, 1}: 14,
	{0, 2}:  16, {2, 0}: 16,
	{1, 3}: 18, {3, 1}: 18,
	{4, 2}:  19,
	{-1, 2}: 22,
	{0, 3}:  25, {3, 0}: 25,
	{4, 1}:  26,
	{-1, 3}: 28,
	{4, 0}:  30,
}

var curveMapHigh = map[[2]int]int{
	{1, 2}:  4,
	{3, 2}:  6,
	{2, 2}:  12,
	{0, 2}:  17,
	{4, 2}:  20,
	{-1, 2}: 23,
}

var curveMapOut = map[[2]int]int{
	{0, -1}: 1,
	{3, 4}:  8,
	{1, -1}: 15,
	{2, 4}:  21,
	{2, -1}: 24,
	{1, 4}:  27,
	{3, -1}: 29,
	{0, 4}:  31,
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func inRange(pos int) bool { return pos >= 0 && pos <= 3 }

func followingPos(currentPos, nextPos, nextTime int, items []customBounceItem) int {
	for _, it := range items {
		if it.time == nextTime {
			return it.pos
		}
	}
	switch {
	case nextPos == 0:
		return 1
	case nextPos == 3:
		return 2
	case currentPos <= nextPos:
		return nextPos + 1
	case currentPos > nextPos:
		return nextPos - 1
	default:
		return nextPos
	}
}

func targetToPos(target int) int {
	switch target {
	case targetOuterLeft:
		return -1
	case targetFirst:
		return 0
	case targetSecond:
		return 1
	case targetThird:
		return 2
	case targetFourth:
		return 3
	case targetOuterRight:
		return 4
	default:
		return 0
	}
}

func blockTargetToPos(target int) int {
	switch target {
	case targetFirst:
		return 0
	case targetSecond:
		return 1
	case targetThird:
		return 2
	case targetFourth:
		return 3
	default:
		return 0
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func deg(v float64) float64 { return v * math.Pi / 180 }

func componentByPath(components map[string]kmdata.Component, path string) kmdata.Component {
	for _, c := range components {
		if c.Path == path {
			return c
		}
	}
	return kmdata.Component{}
}
