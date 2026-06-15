package chargingchicken

import "sort"

type drumHit struct {
	timing float64
	typ    int
	vol    float64
}

var drumLoops = [][]drumHit{
	{},
	{
		{4, 0, 1}, {0.5, 0, 1}, {1.75, 0, 1}, {2.5, 0, 1},
		{1, 1, 1}, {3, 1, 1},
		{4, 2, 1}, {1, 2, 1}, {2, 2, 1}, {3, 2, 1},
		{0.5, 2, 0.7}, {1.5, 2, 0.7}, {2.5, 2, 0.7}, {3.5, 2, 0.7},
	},
	{
		{4, 0, 1}, {0.5, 0, 1}, {20.0 / 6.0, 0, 1}, {2.5, 0, 1},
		{1, 1, 1}, {3, 1, 1},
		{4, 2, 1}, {1, 2, 1}, {2, 2, 1}, {3, 2, 1},
		{0.5, 2, 0.7}, {1.5, 2, 0.7}, {2.5, 2, 0.7}, {3.5, 2, 0.7},
		{2.0 / 6.0, 2, 0.5}, {5.0 / 6.0, 2, 0.5}, {8.0 / 6.0, 2, 0.5}, {11.0 / 6.0, 2, 0.5},
		{14.0 / 6.0, 2, 0.5}, {17.0 / 6.0, 2, 0.5}, {20.0 / 6.0, 2, 0.5}, {23.0 / 6.0, 2, 0.5},
	},
	{
		{4, 0, 1}, {2.0 / 3.0, 0, 1}, {5.0 / 3.0, 0, 1}, {8.0 / 3.0, 0, 1},
		{1, 1, 1}, {3, 1, 1},
		{4, 2, 1}, {1, 2, 1}, {2, 2, 1}, {3, 2, 1},
		{2.0 / 3.0, 2, 0.7}, {5.0 / 3.0, 2, 0.7}, {8.0 / 3.0, 2, 0.7}, {11.0 / 3.0, 2, 0.7},
	},
	{
		{4, 0, 1}, {2.0 / 3.0, 0, 1}, {5.0 / 3.0, 0, 1}, {2, 0, 1}, {8.0 / 3.0, 0, 1},
		{4.0 / 3.0, 1, 1}, {3, 1, 1},
		{4, 2, 1}, {4.0 / 3.0, 2, 1}, {2, 2, 1}, {3, 2, 1},
		{1.0 / 3.0, 2, 0.7}, {1, 2, 0.7}, {5.0 / 3.0, 2, 0.7}, {7.0 / 3.0, 2, 0.7}, {8.0 / 3.0, 2, 0.7}, {11.0 / 3.0, 2, 0.7},
	},
	{{2, 3, 0.8}, {0.5, 5, 0.7}, {1, 4, 1.2}, {1.5, 5, 0.7}},
	{{4, 6, 1}, {2, 6, 1}, {1, 7, 1}, {3, 7, 1}, {0.5, 8, 1}, {1.5, 8, 1}, {2.5, 8, 1}, {3.5, 8, 1}, {11.0 / 6.0, 8, 0.4}, {23.0 / 6.0, 7, 0.4}},
	{{2, 9, 1}, {0.75, 9, 1}, {1, 10, 1}, {0.5, 11, 1}, {1.5, 11, 1}},
	{{4, 21, 1}, {0.5, 22, 1}, {1, 23, 1}, {1.5, 24, 1}, {1.75, 25, 1}, {2, 26, 1}, {2.25, 27, 1}, {2.5, 28, 1}, {2.75, 29, 1}, {3, 30, 1}, {3.5, 31, 1}, {3.75, 32, 1}},
	{{4, 41, 1.5}, {0.25, 42, 1.5}, {0.5, 43, 1.5}, {0.75, 44, 1.5}, {1, 45, 1.5}, {1.25, 46, 1.5}, {1.5, 47, 1.5}, {1.75, 48, 1.5}, {2, 49, 1.5}, {2.25, 50, 1.5}, {2.5, 51, 1.5}, {2.75, 52, 1.5}, {3, 53, 1.5}, {3.25, 54, 1.5}, {3.5, 55, 1.5}, {3.75, 56, 1.5}},
	{{4, 12, 2.5}, {1.75, 12, 2.5}, {2.5, 12, 2.5}, {1, 13, 2.5}, {3, 13, 2.5}, {0.5, 14, 2.5}, {1.5, 14, 2.5}, {2.5, 14, 2.5}, {3.5, 14, 2.5}},
}

func drumName(typ int) string {
	switch typ {
	case 0:
		return "kick"
	case 1:
		return "snare"
	case 2:
		return "hihat"
	case 3:
		return "feverkick"
	case 4:
		return "feversnare"
	case 5:
		return "feverhat"
	case 6:
		return "dskick"
	case 7:
		return "dssnare"
	case 8:
		return "dshat"
	case 9:
		return "gbakick"
	case 10:
		return "gbasnare"
	case 11:
		return "gbahat"
	case 12:
		return "practicekick"
	case 13:
		return "practicesnare"
	case 14:
		return "practicehat"
	default:
		return "MISC" + itoa(typ-20)
	}
}

func sortedDrumLoop(which int) []drumHit {
	if which < 0 || which >= len(drumLoops) {
		which = 1
	}
	out := append([]drumHit(nil), drumLoops[which]...)
	sort.Slice(out, func(i, j int) bool { return out[i].timing < out[j].timing })
	return out
}

func loopLength(which int) float64 {
	if which < 0 || which >= len(drumLoops) || len(drumLoops[which]) == 0 {
		return 4
	}
	return drumLoops[which][0].timing
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
