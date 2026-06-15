// Package wariodemambo ports Wario de Mambo's call-and-response turn/jump
// interval, spotlight choreography, command text, lights, and final dance flow.
package wariodemambo

import (
	"math"
	"math/rand"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	gameID = "warioDeMambo"

	actionLeft  = 1
	actionRight = 2
	actionJump  = 3 // HS jump is Pad East / squeeze / flick; use the existing alt lane.
)

const (
	bopNormal bopState = iota
	bopThink
	bopReady
	bopHappy
	bopFail
)

const (
	danceStationary = iota
	dancePose1
	dancePose2
	danceEnd
)

const (
	lightsOff = iota
	lightsStage1
	lightsStage2
	lightsStage3
)

const (
	spotsDancers = iota
	spotsWario
	spotsRandom
)

const (
	easeLinear     = 0
	easeInQuad     = 2
	easeOutQuint   = 12
	spotRandMinLen = 0.35
	spotRandMaxLen = 0.75
)

type bopState int

type bopEvt struct {
	beat, length         float64
	auto, mambo, dancers bool
	lights               bool
}

type intervalEvt struct {
	idx          int
	beat, length float64
	autoPass     bool
	memorize     bool
	numbers      bool
	text         bool
	left         bool
	resetColor   bool
}

type inputEvt struct {
	beat float64
	jump bool
}

type passEvt struct {
	beat, length float64
	numbers      bool
	text         bool
}

type textEvt struct {
	beat, length float64
	text         string
}

type reactionEvt struct {
	beat, length float64
	resetColor   bool
}

type lightEvt struct {
	beat  float64
	stage int
}

type danceEvt struct {
	beat, length float64
	typ          int
}

type colorEvt struct {
	beat     float64
	red, dim bool
}

type crEvent struct {
	beat, relative float64
	tag            string
}

type spotEase struct {
	beat, length float64
	ease         int
	from, to     [2]float64
	active       bool
}

type danceEase struct {
	beat, length float64
	clip         string
	active       bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	commandText string
	endPose     string
	spotL       string
	spotR       string
	spotLTarget string
	spotRTarget string
	spotWario   string
	textAnim    string

	dancerL, dancerR         string
	dancerLArm, dancerRArm   string
	dancerLHead, dancerRHead string
	dancerLJump, dancerRJump string
	warioBody, warioArm      string
	warioFace, warioJump     string
	topLight, leftLight      string
	rightLight               string

	tintPaths            []string
	blueAdd, redAdd      [4]float64
	gameRed, gameDim     bool
	spotLPos, spotRPos   [2]float64
	spotLEase, spotREase spotEase
	danceEase            danceEase
	currentText          string

	bops      []bopEvt
	intervals []intervalEvt
	inputs    []inputEvt
	passes    []passEvt
	texts     []textEvt
	reactions []reactionEvt
	lights    []lightEvt
	dances    []danceEvt
	colors    []colorEvt

	startedIntervals map[int]bool
	pendingPasses    []passEvt
	crEvents         []crEvent
	crStart          float64
	expectedInputs   []float64

	shouldBeLeft       bool
	warioLeft          bool
	dancerLeft         bool
	dancerArmCentered  bool
	armCentered        bool
	canBop             bool
	autoBop            bool
	isDancing          bool
	armControlsEnabled bool
	hasFlicked         bool
	misses             int
	bopState           bopState
	dancerBopState     bopState
	lightsStage        int
	spotsPos           int
	lastPulse          int
}

func New() engine.Module {
	return &Module{
		rng:                rand.New(rand.NewSource(0x005b64)),
		startedIntervals:   map[int]bool{},
		crStart:            math.Inf(1),
		dancerArmCentered:  true,
		armCentered:        true,
		canBop:             true,
		autoBop:            true,
		armControlsEnabled: true,
		lightsStage:        lightsStage1,
		spotsPos:           spotsWario,
		lastPulse:          math.MinInt,
		blueAdd:            [4]float64{0, 0.09019608, 0.09411765, 1},
		redAdd:             [4]float64{0.28627452, 0.105882354, 0.08627451, 1},
	}
}

func (m *Module) ID() string { return gameID }
