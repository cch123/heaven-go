// Package lovelab ports Love Lab's interval, flask pass, heart merge, box guy,
// time-of-day, cloud, and spotlight choreography.
package lovelab

import (
	"math"

	"hsdemo/engine"
	"hsdemo/games/internal/particlefx"
	"hsdemo/kart"
)

const gameID = "loveLab"

const (
	flaskFast = iota
	flaskSlow
	flaskMidSlow
)

const (
	timeSunset = iota
	timeDay
)

const (
	boxTakeAway = iota
	boxPutBack
	boxNoBox
	boxInstaBox
)

const (
	spotNormal = iota
	spotCone
)

const (
	spotBoy = iota
	spotGirl
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type intervalEvt struct {
	idx          int
	beat, length float64
	autoPass     bool
}

type shakeEvt struct {
	beat, length float64
	speed        int
}

type colorEvt struct {
	beat    float64
	a, b, c [4]float64
}

type timeEvt struct {
	beat float64
	typ  int
}

type cloudEvt struct {
	beat float64
	on   bool
}

type spotEvt struct {
	beat       float64
	active     bool
	typ, where int
}

type boxEvt struct {
	beat   float64
	action int
}

type blushEvt struct {
	beat float64
	auto bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	labGuy, labGuyHead, labGuyArm                   string
	labGirl, labGirlHead, labGirlArm                string
	labAssistant, labAssistantHead, labAssistantArm string
	flaskSprite, girlFlaskSprite, weirdFlaskSprite  string
	boyFlaskBreak, girlFlaskBreak                   string
	heartBox, boxPerson, boxPersonDay               string
	spotlight, spotConeRoot, spotCone               string
	clouds, sunsetBG, dayBG                         string
	girlHeader, boyHeader, weirdHeader              string
	endPoint                                        string

	paths         map[string]flaskPath
	flaskArcsBoy  []string
	flaskArcsGirl []string

	flaskTemplate         *kart.Template
	girlFlaskTemplate     *kart.Template
	guyHeartTemplate      *kart.Template
	girlHeartTemplate     *kart.Template
	completeHeartTemplate *kart.Template

	bops      []bopEvt
	intervals []intervalEvt
	shakes    []shakeEvt
	colors    []colorEvt
	times     []timeEvt
	cloudEvts []cloudEvt
	spots     []spotEvt
	boxes     []boxEvt
	blushes   []blushEvt

	started map[int]bool

	flasks         []*flaskObj
	guyHearts      []*heartObj
	girlHearts     []*heartObj
	completeHearts []*heartObj
	particles      []particleObj
	breakParticles *particlefx.Runtime
	breakEffects   []particlefx.Effect
	currentHearts  []int

	boyLiquid, girlLiquid, weirdLiquid [4]float64
	isDay                              bool
	canCloudsMove                      bool
	canBop                             bool
	bopRight                           bool
	hasMissed                          bool
	hasStartedInterval                 bool
	isHolding                          bool
	isHoldingFlask                     bool
	hasShakenUp                        bool
	releaseValid                       bool
	lastPulse                          int

	cloudSpeed, cloudDistance float64
}

func New() engine.Module {
	return &Module{
		paths:         map[string]flaskPath{},
		started:       map[int]bool{},
		boyLiquid:     [4]float64{0.02909997, 0.4054601, 0.97, 1},
		girlLiquid:    [4]float64{0.972549, 0.3764706, 0.03137255, 1},
		weirdLiquid:   [4]float64{0.8313726, 0.2039216, 0.5058824, 1},
		canCloudsMove: true,
		canBop:        true,
		releaseValid:  true,
		lastPulse:     math.MinInt,
		cloudSpeed:    0.06,
		cloudDistance: 30,
	}
}

func (m *Module) ID() string { return gameID }
