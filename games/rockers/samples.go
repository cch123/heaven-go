package rockers

import "math"

type noteSample struct {
	key string
}

const (
	sampleNone = iota
	sampleBendG5
	sampleBendC6
	sampleChordA
	sampleChordAsus4
	sampleChordBm
	sampleChordCSharpm7
	sampleChordDmaj7
	sampleChordDmaj9
	sampleChordFSharp5
	sampleChordG
	sampleChordG5
	sampleChordGdim7
	sampleChordGm
	sampleNoteASharp4
	sampleNoteA5
	samplePracticeChordD
	sampleRemix6ChordA
	sampleRemix10ChordD
	sampleRemix10ChordFSharpm
	sampleDoremiChordA7
	sampleDoremiChordAm7
	sampleDoremiChordC
	sampleDoremiChordC7
	sampleDoremiChordCadd9
	sampleDoremiChordDm
	sampleDoremiChordDm7
	sampleDoremiChordEm
	sampleDoremiChordF
	sampleDoremiChordFadd9
	sampleDoremiChordFm
	sampleDoremiChordG
	sampleDoremiChordG7
	sampleDoremiChordGm
	sampleDoremiChordGsus4
	sampleDoremiNoteA2
	sampleDoremiNoteE2
)

var sampleTable = []noteSample{
	{""},
	{"BendG5"},
	{"BendC6"},
	{"rocker/rockerChordA"},
	{"rocker/rockerChordAsus4"},
	{"rocker/rockerChordBm"},
	{"rocker/rockerChordC#m7"},
	{"rocker/rockerChordDmaj7"},
	{"rocker/rockerChordDmaj9"},
	{"rocker/rockerChordF#5"},
	{"rocker/rockerChordG"},
	{"rocker/rockerChordG5"},
	{"rocker/rockerChordGdim7"},
	{"rocker/rockerChordGm"},
	{"rocker/rockerNoteA#4"},
	{"rocker/rockerNoteA5"},
	{"rocker/rockerPracticeChordD"},
	{"rocker/rockerRemix6ChordA"},
	{"rocker/rockerRemix10ChordD"},
	{"rocker/rockerRemix10ChordF#m"},
	{"doremi/doremiChordA7"},
	{"doremi/doremiChordAm7"},
	{"doremi/doremiChordC"},
	{"doremi/doremiChordC7"},
	{"doremi/doremiChordCadd9"},
	{"doremi/doremiChordDm"},
	{"doremi/doremiChordDm7"},
	{"doremi/doremiChordEm"},
	{"doremi/doremiChordF"},
	{"doremi/doremiChordFadd9"},
	{"doremi/doremiChordFm"},
	{"doremi/doremiChordG"},
	{"doremi/doremiChordG7"},
	{"doremi/doremiChordGm"},
	{"doremi/doremiChordGsus4"},
	{"doremi/doremiNoteA2"},
	{"doremi/doremiNoteE2"},
}

func sampleAt(idx int) noteSample {
	if idx < 0 || idx >= len(sampleTable) {
		return noteSample{}
	}
	return sampleTable[idx]
}

func semitonePitch(semitone int) float64 {
	return math.Exp2(float64(semitone) / 12)
}

func stringVolume(stringSlots int) float64 {
	switch stringSlots {
	case 3:
		return 0.893
	case 4:
		return 0.75
	case 5:
		return 0.686
	case 6:
		return 0.62
	default:
		return 1
	}
}
