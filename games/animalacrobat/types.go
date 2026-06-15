// Package animalacrobat ports Animal Acrobat (animalAcrobat).
//
// Unity references:
// Assets/Scripts/Games/AnimalAcrobat/AnimalAcrobat.cs
// Assets/Scripts/Games/AnimalAcrobat/AcrobatObstacle*.cs
// Assets/Scripts/Games/AnimalAcrobat/PlayerAcrobat.cs
package animalacrobat

import (
	"image/color"
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	kindElephant animalKind = iota
	kindGiraffe
	kindMonkeysLong
	kindMonkeyShort
	kindGorilla
)

const (
	animScale = 0.5

	defaultJumpDistance         = 3
	defaultJumpDistanceGiraffe  = 29
	defaultJumpDistanceStart    = 6
	defaultJumpHeight           = 3.6
	defaultJumpHeightGiraffe    = 5.8
	defaultJumpHeightInitial    = 1.3
	defaultJumpStartDistance    = -1.68
	defaultJumpStartCameraDelta = 3
	defaultCameraJumpDistance   = 6.5
	defaultCameraJumpGiraffe    = 32
	defaultGiraffeCameraZoom    = 6.6
)

var (
	defaultBGAlpha = hexColor(0xad, 0xce, 0x96)
	defaultBGBravo = hexColor(0x59, 0x69, 0x48)
)

type animalKind int

type animalCall struct {
	beat, length float64
	kind         animalKind
}

type bopCall struct {
	beat, length float64
}

type bgEvent struct {
	beat, length float64
	fromA, toA   [4]float64
	fromB, toB   [4]float64
	ease         int
}

type bgEase struct {
	beat, length float64
	fromA, toA   [4]float64
	fromB, toB   [4]float64
	ease         int
}

type bgTileRuntime struct {
	firstBase    [2]float64
	secondBase   [2]float64
	tileDistance float64
	ok           bool
}

type playerRotateMode int

const (
	playerRotateNone playerRotateMode = iota
	playerRotateJump
	playerRotateArc
)

type acrobatObstacle struct {
	kind   animalKind
	beat   float64
	length float64
	inst   *kart.Instance
	spec   obstacleSpec
	input  inputSpec
	x      float64
	gripX  float64
	gripY  float64
	endX   float64
	endY   float64
	held   bool
	done   bool
	canHit bool
	end    bool
}

type obstacleSpec struct {
	holdLength       float64
	holdPadding      float64
	holdPaddingStart float64
	fullRotRange     float64
	ease             int
	rotateRoot       string
	gripPoint        string
	endPoint         string
	rotRel           string
}

type inputSpec struct {
	holdLength float64
	monkey     string
	monkeyRel  string
}

type playerJump struct {
	start, dur float64
	fromX      float64
	toX        float64
	fromY      float64
	toY        float64
	height     float64
	land       bool
	rotate     playerRotateMode
	shadowMul  float64
}

type sparkle struct {
	beat float64
	x, y float64
	col  color.NRGBA
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	player       string
	playerShadow string
	shadowScale  [2]float64
	spotlight    string
	partyPoppers string
	bgTileA      string
	bgTileB      string
	bgTiles      bgTileRuntime

	playerNums map[string]float64
	gameNums   map[string]float64

	templates map[animalKind]*kart.Template
	specs     map[animalKind]obstacleSpec
	inputs    map[animalKind]inputSpec

	rawAnimals []animalCall
	animals    []*acrobatObstacle
	bops       []bopCall
	bgEvents   []bgEvent
	bg         bgEase

	jump       playerJump
	jumpActive bool
	playerX    float64
	playerY    float64
	cameraX    float64
	cameraY    float64
	cameraZ    float64
	cameraWX   float64
	cameraWY   float64
	cameraT    float64
	cameraTSet bool

	holding      *acrobatObstacle
	monkeyMissed bool
	lastMissBeat float64
	lastBop      int
	sparkles     []sparkle
	drumrollStop func()
}

func New() engine.Module {
	return &Module{
		templates:    map[animalKind]*kart.Template{},
		specs:        map[animalKind]obstacleSpec{},
		inputs:       map[animalKind]inputSpec{},
		playerNums:   map[string]float64{},
		gameNums:     map[string]float64{},
		bg:           bgEase{fromA: defaultBGAlpha, toA: defaultBGAlpha, fromB: defaultBGBravo, toB: defaultBGBravo},
		shadowScale:  [2]float64{1, 1},
		lastMissBeat: math.Inf(-1),
	}
}
