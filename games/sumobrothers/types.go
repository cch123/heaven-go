// Package sumobrothers ports Sumo Brothers' slap/stomp/pose cue flow,
// animator state graph, mapped-material recolors, camera stomp shake, and
// confetti effect from Heaven Studio's CtrSumou implementation.
package sumobrothers

import (
	"math"
	"math/rand"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	gameID    = "sumoBrothers"
	actionAlt = 3 // HS InputAction_Alt uses the South/down-style action.
)

const (
	poseSquat         = 1
	poseStance        = 2
	posePointing      = 3
	poseFinale        = 4
	poseFinaleNoThrow = 5
	poseDab           = 6
)

const (
	bgGreatWave  = 0
	bgOtaniOniji = 1
	bgNone       = 2
	bgNerd       = 3
)

const (
	stompAutomatic = 0
	stompLeft      = 1
	stompRight     = 2
)

const (
	forceStomp = 0
	forceSlap  = 1
)

const (
	stateIdle sumoState = iota
	stateSlap
	stateStomp
	statePose
)

type sumoState int

var (
	defaultBgTop    = [4]float64{1, 1, 2.0 / 255.0, 1}
	defaultBgBottom = [4]float64{1, 1, 0x73 / 255.0, 1}
	defaultMawashiL = [4]float64{0x64 / 255.0, 0x64 / 255.0, 0xfd / 255.0, 1}
	defaultMawashiR = [4]float64{1, 0x64 / 255.0, 0x64 / 255.0, 1}
	defaultWhite    = [4]float64{1, 1, 1, 1}
)

type signalEvt struct {
	beat, length float64
	mute         bool
	look         bool
	direction    int
	slap         bool
}

type bgColorEvt struct {
	beat, length float64
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	ease         int
}

type mawashiEvt struct {
	beat        float64
	left, right [4]float64
}

type colorRun struct {
	bgColorEvt
	active bool
}

type shakeKey struct {
	beat float64
	x    float64
}

type confettiParticle struct {
	born, life float64
	x, y       float64
	vx, vy     float64
	rot, vr    float64
	w, h       float64
	col        [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	inu, pBody, gBody       string
	pHead, gHead            string
	impact, glasses         string
	bgMove, bgStatic        string
	bgTopPath, bgBottomPath string
	bgMat, mawashiMat       string

	signals  []signalEvt
	bgColors []bgColorEvt
	mawashis []mawashiEvt

	goBopSumo, goBopInu       bool
	allowBopSumo, allowBopInu bool
	sumoStompDir              bool
	sumoSlapDir               int
	sumoPoseType              int
	sumoPoseTypeNext          int
	sumoPoseCurrent           string
	sumoPoseConfetti          bool
	bgType, bgTypeNext        int
	sumoState, previousState  sumoState
	lookingAtCamera           bool
	cueActive                 bool
	nextSwitchBeat            float64
	lastPulse                 int

	bgRun                     colorRun
	mawashiLeft, mawashiRight [4]float64
	shakeSpeed                float64
	shakeKeys                 []shakeKey

	poseLoopStop func()
	confetti     []confettiParticle
}

func New() engine.Module {
	return &Module{
		rng:             rand.New(rand.NewSource(0x5e6d0)),
		goBopSumo:       true,
		goBopInu:        true,
		allowBopSumo:    true,
		allowBopInu:     true,
		sumoPoseCurrent: "1",
		bgType:          bgNone,
		bgTypeNext:      bgNone,
		nextSwitchBeat:  math.Inf(1),
		lastPulse:       math.MinInt,
		mawashiLeft:     defaultMawashiL,
		mawashiRight:    defaultMawashiR,
		shakeSpeed:      0.125,
	}
}

func (m *Module) ID() string { return gameID }

func eventName(e *riq.Entity) string {
	for i := len(e.Datamodel) - 1; i >= 0; i-- {
		if e.Datamodel[i] == '/' {
			return e.Datamodel[i+1:]
		}
	}
	return e.Datamodel
}
