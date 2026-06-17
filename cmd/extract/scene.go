// scene.go：整游戏场景的泛化提取模式（-game rhythmSomen 等）。
// 与 KarateMan 的单骨架模式不同，这里导出 prefab 的完整节点树、
// 全部 AnimationClip、多张图集，以及游戏脚本字段 → 节点 path 的绑定表。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

type sceneSpec struct {
	dir             string            // Assets/Bundled/Games/<dir>
	prefab          string            // prefab 文件名
	prefabPath      string            // 可选：Assets/ 下的显式 prefab 路径（NoGame 等非 Bundled prefab）
	spritesDir      string            // 可选：贴图根（默认 Sprites）
	noSprites       bool              // 3D-only 游戏没有 Sprites 目录；仍导出空 sprites.json
	animsDir        string            // 可选：动画/controller 根（默认 spritesDir）
	importedAnimFPS float64           // FBX import clip 帧率；0 表示按 Unity 常见 60fps 估算
	roleFields      []string          // 游戏 MonoBehaviour 中需要解析的 Animator/GameObject 引用字段
	roleFallbacks   map[string]string // stripped prefab 引用无法唯一解析时的显式 path 兜底

	refArrayFields  []string // 引用数组字段（如对象模板表）
	strArrayFields  []string // 字符串数组字段（如动画名表）
	curveFields     []string // BezierCurve3D 引用字段
	extraScenePaths []string // FBX 内部动画 path 等无法从 YAML hierarchy 暴露的显式补点
	// synthesizeAnimPaths adds scene nodes for AnimationClip paths that live
	// inside imported model/FBX hierarchies. Unity resolves those child
	// Transforms at runtime through the model importer, but they are not YAML
	// GameObjects in the owning prefab.
	synthesizeAnimPaths bool
	objMarkers          []string // 识别"对象模板组件"的字段集合（如 MobTrickObj）
	objRefFields        []string // 模板组件上的单引用字段（→ 节点 path，如 Meat.startPosition）
	objSpriteFields     []string // 模板组件上的 sprite 引用数组字段（→ 切片名，如 Meat.meats）
	wantSequences       bool     // 提取 SoundSequences 组件
	commonSounds        []string // 需要的公共音效（Assets/Resources/Sfx/<name>）
	extraSounds         []extraSound
	wantControllers     bool // 提取 AnimatorController 状态机（controllers.json + animators.json）
	wantTexts           bool // 提取 TMP 世界文本（texts.json + fonts/）

	components []componentSpec // 通用组件 dump（Extra.Components）
	// templatePrefabs 是 C# 运行时 Instantiate 的 prefab 资产引用；这些
	// prefab 不在主场景树里，但运行时必须有完整子树供 kart.Template 复用。
	// 提取时作为隐藏根挂到导出 scene 下，保留原 fileID 以便主脚本字段引用解析。
	templatePrefabs []string
}

// extraSound copies a sound referenced by C# using another minigame namespace
// (for example Hole in One's whale sign cue uses clappyTrio/sign).
type extraSound struct {
	dir string // Assets/Bundled/Games/<dir>/Sounds
	rel string // source path under Sounds
	out string // output path under assets/<game>/sounds; defaults to rel
}

// componentSpec 按字段特征（可选限定 GameObject path）识别一个 MonoBehaviour，
// 全字段通用 dump 到 Extra.Components[name]。
type componentSpec struct {
	name        string
	markers     []string // 必须同时存在的字段
	atPath      string   // 非空时限定组件所在 GameObject 的 path（同字段集脚本如 TCTotem/TCDragon）
	multi       bool     // 匹配多个组件：导出为 name0、name1…（按 path 排序）
	curveFields []string // 组件字段里的 BezierCurve3D 引用，导出到 Extra.Curves

	// curveArrayFields handles BezierCurve3D[] fields such as Hole in One's
	// Ball.curve. Each entry is exported as <component>.<field><index>.
	curveArrayFields []string
}

var sceneSpecs = map[string]sceneSpec{
	"agbSamuraiSlice": {
		dir:    "SamuraiSliceAgb",
		prefab: "agbSamuraiSlice.prefab",
		roleFields: []string{
			"samuraiAnim", "yokaiEntity", "mayaFeyFromAceAttorney", "samuraiObject",
			"fireAnim", "fireParent", "fogAnim",
		},
		refArrayFields:  []string{"fogSprite"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"samuraiAnim", "yokaiEntity", "mayaFeyFromAceAttorney", "stepDistance", "startPosition", "fogSprite"}},
			{name: "yokai", markers: []string{"enterCurves", "flyingCurves", "missCurve", "bigYokaiStartPosition", "bigYokaiXDistance", "shadowOffsetX"}, atPath: "yokai1", curveFields: []string{"missCurve"}, curveArrayFields: []string{"enterCurves", "flyingCurves"}},
			{name: "maya", markers: []string{"enterCurves", "flyingCurves", "missCurve", "bigYokaiStartPosition", "bigYokaiXDistance", "shadowOffsetX"}, atPath: "AA1_Maya_0", curveFields: []string{"missCurve"}, curveArrayFields: []string{"enterCurves", "flyingCurves"}},
		},
	},
	"airboarder": {
		dir:      "Airboarder",
		prefab:   "airboarder.prefab",
		animsDir: "Models",
		roleFields: []string{
			"cameraPivot", "cameraPos", "cameraPosLegacy", "cameraPosPrac",
			"archBasic", "wallBasic", "floor",
			"CPU1", "CPU2", "Player", "Dog", "Tail", "Floor",
		},
		wantControllers:     true,
		synthesizeAnimPaths: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"bgMaterial", "fadeMaterial", "floorMaterial", "cloudMaterial",
				"cameraPivot", "cameraPos", "cameraPosLegacy", "cameraPosPrac", "cameraFOV",
				"archBasic", "wallBasic", "floor", "CPU1", "CPU2", "Player", "Dog", "Tail", "Floor",
				"startFloor",
			}},
		},
	},
	"bonOdori": {
		dir:    "BonOdori",
		prefab: "bonOdori.prefab",
		roleFields: []string{
			"darkPlane", "Judge", "JudgeFace",
		},
		refArrayFields: []string{
			"Texts", "TextsBlue", "Donpans",
		},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"nearMiss.ogg"},
	},
	"figureFighter": {
		dir:    "FigureFighter",
		prefab: "figureFighter.prefab",
		roleFields: []string{
			"dollAnim", "crowdAnim", "bagAnim", "bagCopyAnim", "buttonAnim",
			"lightsAnim", "topAnim", "barsAnim", "fartAnim", "bagObject",
			"chainParticles1", "chainParticles2",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"dollAnim", "crowdAnim", "bagAnim", "bagCopyAnim", "buttonAnim", "lightsAnim", "topAnim", "barsAnim", "fartAnim", "bagObject"}},
		},
	},
	"fillbots": {
		dir:    "Fillbots",
		prefab: "fillbots.prefab",
		roleFields: []string{
			"smallBot", "mediumBot", "largeBot", "filler", "conveyerBelt",
			"blackout", "BGPlane",
		},
		refArrayFields:  []string{"gears", "meters", "metersFuel", "fillerRenderer", "otherRenderer"},
		wantControllers: true,
		templatePrefabs: []string{
			"Prefabs/BotSmall.prefab",
			"Prefabs/BotMedium.prefab",
			"Prefabs/BotLarge.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"smallBot", "mediumBot", "largeBot", "filler", "gears", "meters",
				"metersFuel", "impactMaterial", "conveyerBelt", "blackout",
				"fillerRenderer", "otherRenderer", "BGPlane",
			}},
			{name: "bot", markers: []string{
				"size", "limbFallHeight", "flyDistance", "stackDistanceRate",
				"fullBody", "legs", "body", "head", "fuelFill", "fillAnim",
			}, multi: true},
			{name: "fullBody", markers: []string{
				"mask", "sprites", "fullBody",
			}, multi: true},
		},
	},
	"frogHop": {
		dir:    "FrogHop",
		prefab: "frogHop.prefab",
		roleFields: []string{
			"PlayerFrog", "LeaderFrog", "SingerFrog",
			"Darkness", "SpotlightFront", "SpotlightBack",
			"SpotlightFrontColor", "SpotlightBackColor",
			"Mike", "Mike2", "Stage", "StageTop",
			"gradient", "bgLow", "bgHigh",
		},
		refArrayFields:  []string{"OtherFrogs", "_FrogColors"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{
				"PlayerFrog", "OtherFrogs", "LeaderFrog", "SingerFrog",
				"Darkness", "SpotlightFront", "SpotlightBack",
				"SpotlightFrontColor", "SpotlightBackColor",
				"Mike", "Mike2", "Stage", "StageTop",
				"_FrogColors", "gradient", "bgLow", "bgHigh",
			}},
			{name: "frog", markers: []string{
				"FrogAnim", "SpriteParts", "Head", "Belt",
				"BodyMat", "HeadMat",
			}, multi: true},
		},
	},
	"freezeFrame": {
		dir:    "FreezeFrame",
		prefab: "freezeFrame.prefab",
		roleFields: []string{
			"CameraMan", "Photograph1", "Photograph2", "Photograph3", "Results",
			"IntroSign", "Overlay", "Crosshair", "Shutter", "DimRect",
			"StickyLayer", "FarCarSpawn", "NearCarSpawn", "WalkerSpawn",
			"Crowd", "CrowdFarLeft", "CrowdLeft", "CrowdRight", "CrowdFarRight",
			"Billboards",
		},
		refArrayFields:  []string{"Photographs"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "applause.ogg"},
		templatePrefabs: []string{
			"Prefabs/FarCar.prefab",
			"Prefabs/NearCar.prefab",
			"Prefabs/Walker.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"CameraMan", "Photographs", "Photograph1", "Photograph2", "Photograph3", "Results",
				"IntroSign", "Overlay", "Crosshair", "Shutter", "DimRect", "StickyLayer",
				"FarCarSpawn", "FarCarPrefab", "NearCarSpawn", "NearCarPrefab",
				"WalkerSpawn", "WalkerPrefab", "Crowd", "CrowdFarLeft", "CrowdLeft",
				"CrowdRight", "CrowdFarRight", "CrowdSprites", "Billboards",
			}},
			{name: "car", markers: []string{"_Animator", "_ParticleSystem"}, multi: true},
		},
	},
	"moaiDooWop": {
		dir:    "MoaiDooWop",
		prefab: "moaiDooWop.prefab",
		roleFields: []string{
			"cpuMoaiAnim", "playerMoaiAnim", "cpuMoaiMoveAnim", "playerMoaiMoveAnim", "bgAnim",
			"GlassesM", "GlassesF", "MRibbon", "FRibbon", "MFlower", "FFlower",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"cpuMoaiAnim", "playerMoaiAnim", "cpuMoaiMoveAnim", "playerMoaiMoveAnim", "bgAnim",
				"GlassesM", "GlassesF", "MRibbon", "FRibbon", "MFlower", "FFlower",
				"birdAnims", "bgBirdAnims", "poopAnims",
			}},
		},
	},
	"monkeyWatch": {
		dir:    "MonkeyWatch",
		prefab: "monkeyWatch.prefab",
		roleFields: []string{
			"cameraAnchor", "cameraTransform", "cameraMoveable", "monkeyClockArrow",
			"monkeyHandler", "backgroundHandler", "balloonHandler", "middleMonkey",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
		templatePrefabs: []string{
			"Prefabs/YellowMonkey.prefab",
			"Prefabs/PinkMonkey.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"cameraAnchor", "cameraTransform", "cameraMoveable", "monkeyClockArrow",
				"monkeyHandler", "backgroundHandler", "balloonHandler", "middleMonkey",
				"fullZoomOut", "zoomOutBeatLength", "zoomInBeatLength",
			}},
			{name: "clockArrow", markers: []string{
				"anim", "anchorRotateTransform", "playerMonkeyAnim",
				"yellowClap", "pinkClap", "shadowTrans", "camMoveTrans",
				"shadowXRange", "shadowYRange",
			}},
			{name: "monkeyHandler", markers: []string{"yellowMonkeyRef", "pinkMonkeyRef", "maxMonkeys"}},
			{name: "background", markers: []string{"srsIn", "srsOut", "anchorHour", "anchorMinute"}},
			{name: "balloon", markers: []string{"anchor", "target", "balloonTrans", "srs", "shadow", "xOffset", "yOffset"}},
			{name: "monkey", markers: []string{"isPink"}, multi: true},
		},
	},
	"theDazzles": {
		dir:    "TheDazzles",
		prefab: "theDazzles.prefab",
		roleFields: []string{
			"player", "poseEffect", "starsEffect",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"npcGirls", "player", "poseEffect", "starsEffect", "interiorMat", "exteriorMat",
			}},
		},
	},
	"quizShow": {
		dir:    "QuizShow",
		prefab: "quizShow.prefab",
		roleFields: []string{
			"contesteeLeftArmAnim", "contesteeRightArmAnim", "contesteeHead",
			"hostLeftArmAnim", "hostRightArmAnim", "hostHead", "signAnim",
			"timerTransform", "stopWatchRef", "blackOut",
			"firstDigitSr", "secondDigitSr", "hostFirstDigitSr", "hostSecondDigitSr",
			"contCounter", "hostCounter", "contExplosion", "hostExplosion", "signExplosion",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"contesteeLeftArmAnim", "contesteeRightArmAnim", "contesteeHead",
				"hostLeftArmAnim", "hostRightArmAnim", "hostHead", "signAnim",
				"timerTransform", "stopWatchRef", "blackOut",
				"firstDigitSr", "secondDigitSr", "hostFirstDigitSr", "hostSecondDigitSr",
				"contCounter", "hostCounter", "contExplosion", "hostExplosion", "signExplosion",
				"contestantNumberSprites", "hostNumberSprites", "explodedCounter",
			}},
		},
	},
	"rapMen": {
		dir:       "RapMen",
		prefab:    "rapMen.prefab",
		wantTexts: true,
		roleFields: []string{
			"rapperRed", "rapperYellow", "rapperCherry", "rapperBlue",
			"rapperRedObj", "rapperYellowObj", "rapperCherryObj", "rapperBlueObj",
			"rapText", "uhnParticle", "background",
		},
		refArrayFields:  []string{"justParticles"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"rapperRed", "rapperYellow", "rapperCherry", "rapperBlue",
				"rapperRedObj", "rapperYellowObj", "rapperCherryObj", "rapperBlueObj",
				"rapText", "gradients", "justParticles", "uhnParticle",
				"backgroundMaterial", "speakerMaterial", "background",
			}},
		},
	},
	"nightWalkAgb": {
		dir:       "NightWalkAgb",
		prefab:    "nightWalkAgb.prefab",
		wantTexts: true,
		roleFields: []string{
			"playYan", "platformHandler", "starHandler",
			"Text", "TextboxTransform", "TextboxGO", "TextboxSprite",
		},
		templatePrefabs: []string{
			"Prefabs/JumpPlatform.prefab",
			"Prefabs/Star.prefab",
		},
		wantControllers: true,
		commonSounds: []string{
			"count-ins/cowbell.wav",
			"games/nightWalkRvl/highJump1.ogg",
			"games/nightWalkRvl/highJump2.ogg",
			"games/nightWalkRvl/highJump3.ogg",
			"games/nightWalkRvl/highJump4.ogg",
			"games/nightWalkRvl/highJump5.ogg",
			"games/nightWalkRvl/highJump6.ogg",
			"games/nightWalkRvl/highJump7.ogg",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"playYan", "platformHandler", "starHandler",
				"Text", "TextboxTransform", "TextboxGO", "TextboxSprite",
				"StarMat", "PlatformMat", "PlatLightMat", "FishMat", "BGMat",
				"jumpPaths",
			}},
			{name: "platformHandler", markers: []string{
				"platformRef", "starHandler", "defaultYPos", "heightAmount",
				"platformDistance", "playerXPos", "starLength", "starHeight", "platformCount",
			}},
			{name: "platform", markers: []string{
				"platform", "fallYan", "fallYanRoll", "fish",
				"rollPlatform", "rollPlatformLong", "rollPlatformLong2",
			}, atPath: "JumpPlatform"},
			{name: "starHandler", markers: []string{
				"starRef", "boundaryX", "boundaryY", "starCount", "blinkFrequency", "blinkAmount",
			}},
		},
	},
	"samuraiSliceNtr": {
		dir:    "SamuraiSliceNtr",
		prefab: "samuraiSliceNtr.prefab",
		roleFields: []string{
			"player", "launcher", "objectPrefab", "childParent", "objectHolder",
			"background", "fasterWarning", "darknessOverlay", "theMoon", "moonText",
		},
		refArrayFields:  []string{"Effects"},
		curveFields:     []string{"InCurve", "LaunchCurve", "LaunchHighCurve", "NgLaunchCurve", "DebrisLeftCurve", "DebrisRightCurve", "NgDebrisCurve"},
		wantControllers: true,
		wantTexts:       true,
		components: []componentSpec{
			{name: "game", markers: []string{"player", "launcher", "objectPrefab", "childParent", "objectHolder", "InCurve", "LaunchCurve", "LaunchHighCurve", "NgLaunchCurve", "DebrisLeftCurve", "DebrisRightCurve", "NgDebrisCurve", "background", "fasterWarning", "theMoon"}, curveFields: []string{"InCurve", "LaunchCurve", "LaunchHighCurve", "NgLaunchCurve", "DebrisLeftCurve", "DebrisRightCurve", "NgDebrisCurve"}},
			{name: "object", markers: []string{"moneyBurst", "pickelBurst", "pickelBurstSplat", "doubleLaunchPos"}, atPath: "ObjectRoot"},
			{name: "child", markers: []string{"DebrisPosL", "DebrisPosR", "WalkPos0", "WalkPos1"}, atPath: "Child"},
		},
	},
	"rhythmTestGBA": {
		dir:    "RhythmTestGBA",
		prefab: "rhythmTestGBA.prefab",
		roleFields: []string{
			"noteFlash", "screenText", "buttonAnimator", "flashAnimator",
			"numberBGAnimator", "numberAnimator", "textAnimator",
		},
		wantControllers: true,
		wantTexts:       true,
	},
	"rhythmSheriff": {
		dir:    "RhythmSheriff",
		prefab: "rhythmSheriff.prefab",
		roleFields: []string{
			"dogSheriff", "targetObj", "tumbleweedBack", "tumbleweedFront", "tumbleweedOverlay",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"dogSheriff", "ratPitch", "ratLowerPitch", "ratFinalPitch", "catPitch", "catLowerPitch", "catFinalPitch", "targetObj"}},
			{name: "target", markers: []string{"target", "hole"}, atPath: "TargetHolder/Target"},
		},
	},
	"rhythmFighter": {
		dir:    "RhythmFighter",
		prefab: "rhythmFighter.prefab",
		roleFields: []string{
			"fighterR", "fighterL", "holderR", "holderL", "displayHolderAnim",
			"displayHolder", "musicNote", "lightsL", "lightsR", "fightText", "spotLight",
		},
		wantControllers: true,
		templatePrefabs: []string{
			"Prefabs/Note.prefab",
		},
	},
	"gardenDance": {
		dir:             "GardenDance",
		prefab:          "gardenDance.prefab",
		roleFields:      []string{"flowerPlayer", "sunAnim", "birdAnim"},
		refArrayFields:  []string{"flowers"},
		wantControllers: true,
		components: []componentSpec{
			{name: "flower", markers: []string{"anim", "danceRight"}, multi: true},
		},
	},
	"loveRap": {
		dir:    "LoveRap",
		prefab: "loveRap.prefab",
		roleFields: []string{
			"playerRapper", "playerBody", "playerLegs", "playerFace", "playerMouth", "playerFlash", "playerBubble", "playerText",
			"cpuRapper", "cpuBody", "cpuLegs", "cpuFace", "cpuMouth", "cpuFlash", "cpuBubble", "cpuHeadTrans", "cpuText",
			"mcMouth", "mcBody", "mcLegs", "car", "car_lights", "mcBubble", "mcText",
			"bgSR", "bgWetSR",
		},
		roleFallbacks: map[string]string{
			"playerFace":  "Rappers/PlayerHolder/OtherRapper/rap_body/HeadHolder/rap_eye",
			"playerMouth": "Rappers/PlayerHolder/OtherRapper/rap_body/HeadHolder/rap_mouth",
			"cpuFace":     "Rappers/OtherRapperHolder/OtherRapper/rap_body/HeadHolder/rap_eye",
			"cpuMouth":    "Rappers/OtherRapperHolder/OtherRapper/rap_body/HeadHolder/rap_mouth",
		},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"miss.wav"},
	},
	"lumbearjack": {
		dir:    "LumBEARjack",
		prefab: "lumbearjack.prefab",
		roleFields: []string{
			"_bear", "_baby", "_smallObjectPrefab", "_bigObjectPrefab", "_hugeObjectPrefab", "_cutObjectHolder",
			"_catRight", "_catRightMove", "_catLeft", "_catLeftMove",
			"_particleHitPoint", "_particleCutPoint", "_missObjectRef", "_bombRef",
			"_snowParticle",
		},
		refArrayFields: []string{
			"_catRightObjectsSmall", "_catRightObjectsBig", "_catRightObjectsHuge",
			"_catLeftObjectsSmall", "_catLeftObjectsBig", "_catLeftObjectsHuge",
			"_bgCats",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefabs/SmallObject.prefab",
			"Prefabs/BigObject.prefab",
			"Prefabs/HugeObject.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"_bear", "_baby", "_smallObjectPrefab", "_bigObjectPrefab", "_hugeObjectPrefab",
				"_catRight", "_catRightObjectsSmall", "_catLeft", "_catLeftObjectsSmall",
				"_particleHitPoint", "_particleCutPoint", "_missObjectRef", "_bombRef",
				"_catAnimationOffsetStart", "_catAnimationOffsetEnd",
			}},
			{name: "bear", markers: []string{"_anim", "_cameraPoint", "_zoomInLength", "_zoomInPower"}, atPath: "Beast"},
			{name: "smallObject", markers: []string{"_log", "_can", "_bat", "_broom", "_barrel", "_book"}, atPath: "SmallObject"},
			{name: "bigObject", markers: []string{"_logSR", "_logCutSprite", "_ballSR1", "_ballCutSprite", "_ballSR2"}, atPath: "BigObject"},
			{name: "hugeObject", markers: []string{"_logSR", "_logCutSprites", "_freezerSR", "_freezerCutSprites", "_peachSR", "_peachCutSprites"}, atPath: "HugeObject"},
			{name: "rotate", markers: []string{"_rotationLeft", "_rotationRight", "_pivotLeft", "_pivotRight", "_objectsToMove"}, multi: true},
			{name: "catMove", markers: []string{"_otherPoint", "_startAtOther", "_usePoint", "_slideOffset"}, multi: true},
			{name: "baby", markers: []string{"_flySprite", "_standSprite", "_rot", "_addedHeight", "_addedY", "_path"}, atPath: "babyRef"},
			{name: "missObject", markers: []string{"_rot", "_height", "_beatDuration", "_jumpDistanceX", "_jumpDistanceY"}, atPath: "MissObjectRef"},
			{name: "bomb", markers: []string{"_rot", "_path"}, atPath: "bombRef"},
		},
	},
	"ninjaBodyguard": {
		dir:    "NinjaBodyguard",
		prefab: "ninjaBodyguard.prefab",
		roleFields: []string{
			"PlayerAnim", "GuideAnim", "LordAnim", "FirstNinja", "NinjaArrow",
			"LeftSceneObj", "Blackout", "HitParticle",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"PlayerAnim", "GuideAnim", "LordAnim", "FirstNinja", "NinjaArrow", "xDistanceEnemy", "yDistanceEnemy"}},
			{name: "enemy", markers: []string{"anim", "sort", "game"}},
			{name: "arrow", markers: []string{"anim", "sort", "divertPosition", "hitCurve", "currentState"}, curveFields: []string{"hitCurve"}},
		},
	},
	"nipInTheBud": {
		dir:             "NipInTheBud",
		prefab:          "nipInTheBud.prefab",
		spritesDir:      "Models/Sprites",
		animsDir:        "Models/Animations",
		roleFields:      []string{"Leilani", "Bubble", "Mosquito", "Mayfly", "mosquitoStart", "mayflyStart", "bg"},
		wantControllers: true,
		components: []componentSpec{
			{name: "mosquito", markers: []string{"startCurve", "approachCurve", "fleeCurve", "body", "wingA", "wingB", "mosquitoAnim"}, curveFields: []string{"startCurve", "approachCurve", "fleeCurve"}},
			{name: "mayfly", markers: []string{"startCurve", "approachCurve", "fleeCurve", "exitCurve", "body", "wing", "mayflyAnim"}, curveFields: []string{"startCurve", "approachCurve", "fleeCurve", "exitCurve"}},
		},
	},
	"octopusMachine": {
		dir:    "OctopusMachine",
		prefab: "octopusMachine.prefab",
		roleFields: []string{
			"bg", "YouArrow", "YouText", "Text",
		},
		refArrayFields:  []string{"Bubbles", "octopodes"},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"bg", "Bubbles", "YouArrow", "YouText", "Text", "octopodes"}},
			{name: "octopus", markers: []string{"sr", "srAll", "octoNum", "anim"}, multi: true},
		},
	},
	"packingPests": {
		dir:    "PackingPests",
		prefab: "packingPests.prefab",
		roleFields: []string{
			"Candy", "Spider", "boxfront",
			"handAnim", "lowerHandAnim", "upperHandAnim", "signAnim",
			"spiderCrawlAnim", "spiderAnim", "curtainAnim",
			"HandAnimPlayer", "HandAnim1", "HandAnim2", "HandAnim3", "HandAnim4",
			"HandAnim5", "HandAnim6", "HandAnim7", "HandAnim8",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefabs/Objects/Candy.prefab",
			"Prefabs/Objects/Spider.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{"Candy", "Spider", "boxfront", "objectPaths", "HandAnimPlayer"}},
		},
	},
	"bouncyRoad": {
		dir:    "BouncyRoad",
		prefab: "bouncyRoad.prefab",
		roleFields: []string{
			"baseBall", "baseBounceCurve", "CurveHolder", "ThingsTrans",
			"PosCurve", "BGGradient", "BGHigh", "BGLow",
		},
		curveFields:     []string{"baseBounceCurve", "PosCurve"},
		wantControllers: true,
	},
	"blueBirds": {
		dir:    "BlueBirds",
		prefab: "blueBirds.prefab",
		roleFields: []string{
			"captainAnim", "captainHolderAnim",
			"bird1Anim", "bird2Anim", "bird3Anim",
			"effect1Anim", "effect2Anim", "effect3Anim",
			"memoryAnim", "memorySprite", "finText",
			"CaptainTransform", "BirdTransform",
		},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"captainAnim", "bird1Anim", "bird2Anim", "bird3Anim", "memoryImage", "gradientMat", "CaptainTransform", "BirdTransform"}},
		},
	},
	"balloonHunter": {
		dir:    "BalloonHunter",
		prefab: "balloonHunter.prefab",
		roleFields: []string{
			"slowBalloon", "fastBalloon", "balloonFive", "bgAnimal",
			"rock", "rockMissCurve", "hunterAnim", "birdAnim", "rockSmear",
		},
		curveFields:     []string{"rockMissCurve"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"slowBalloon", "fastBalloon", "balloonFive", "bgAnimal", "rock", "rockMissCurve", "hunterAnim", "birdAnim", "rockSmear"}, curveFields: []string{"rockMissCurve"}},
			{name: "slowBalloon", markers: []string{"startBeat", "balloonSpeed", "isFive", "moose", "anim", "hunterAnim", "popEffect", "popParticle", "mooseObject"}, atPath: "BalloonSlow"},
			{name: "fastBalloon", markers: []string{"startBeat", "balloonSpeed", "isFive", "moose", "anim", "hunterAnim", "popEffect", "popParticle", "mooseObject"}, atPath: "BalloonFast"},
			{name: "balloonFive", markers: []string{"startBeat", "balloonSpeed", "isFive", "moose", "anim", "hunterAnim", "popEffect", "popParticle", "mooseObject"}, atPath: "BalloonFive"},
			{name: "bgAnimal", markers: []string{"bgObject", "anim", "rabbitAnim", "boarAnim", "mooseAnim", "type", "startBeat", "flyLength", "right", "startY", "endY"}, atPath: "BG/AnimalsBG"},
		},
	},
	"bigRockFinish": {
		dir:    "BigRockFinish",
		prefab: "bigRockFinish.prefab",
		roleFields: []string{
			"playerGhost", "greenGhost", "drummerGhost", "ghostHandL", "ghostHandR",
			"audience", "spotlightMask", "flash",
			"Bass", "Cymbal", "TomL", "TomR", "Snare", "Hihat", "UnlitArea",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"playerGhost", "greenGhost", "drummerGhost", "ghostHandL", "ghostHandR", "audience", "spotlightMask", "flash", "Bass", "Cymbal", "TomL", "TomR", "Snare", "Hihat", "UnlitArea"}},
		},
	},
	"bossaNova": {
		dir:    "BossaNova",
		prefab: "bossaNova.prefab",
		roleFields: []string{
			"bossaAnim", "novaAnim", "ringL", "ringR", "cloudAnim", "positionAnim",
			"ballShape", "cubeShape", "bgOne", "bgTwo", "bgTwoSR",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"bossaAnim", "novaAnim", "ringL", "ringR", "cloudAnim", "positionAnim", "ballShape", "cubeShape", "bgOne", "bgTwo", "bgTwoSR"}},
			{name: "ballShape", markers: []string{"enterCurve", "hitCurve", "missCurve", "shapeTransform", "Shadow"}, atPath: "BallHolder", curveFields: []string{"enterCurve", "hitCurve", "missCurve"}},
			{name: "cubeShape", markers: []string{"enterCurve", "hitCurve", "missCurve", "shapeTransform", "Shadow"}, atPath: "CubeHolder", curveFields: []string{"enterCurve", "hitCurve", "missCurve"}},
		},
	},
	"dressYourBest": {
		dir:    "DressYourBest",
		prefab: "dressYourBest.prefab",
		roleFields: []string{
			"girlAnim", "monkeyAnim", "sewingAnim", "reactionAnim", "cameoAnim",
			"newBG", "bgSpriteRenderer", "lightRenderer",
		},
		wantControllers: true,
		commonSounds:    []string{"nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"girlAnim", "monkeyAnim", "sewingAnim", "reactionAnim", "cameoAnim", "newBG", "bgSpriteRenderer", "lightRenderer", "lightMaterialTemplate", "lightStates"}},
		},
	},
	"catchyTune": {
		dir:    "CatchyTune",
		prefab: "catchyTune.prefab",
		roleFields: []string{
			"plalinAnim", "alalinAnim", "orangeBase", "pineappleBase",
			"fruitHolder", "heartMessage", "bg2",
		},
		wantControllers: true,
	},
	"chameleon": {
		dir:    "Chameleon",
		prefab: "chameleon.prefab",
		roleFields: []string{
			"baseFly", "chameleonAnim", "chameleonEye", "Crown",
			"gradient", "bgHigh", "bgLow",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"baseFly", "chameleonAnim", "chameleonEye", "Crown"}},
			{name: "fly", markers: []string{"flyAnim", "wingAnim"}, atPath: "Fly"},
		},
	},
	"clapTrap": {
		dir:    "ClapTrap",
		prefab: "clapTrap.prefab",
		roleFields: []string{
			"Background", "bg", "stageLeft", "stageRight", "stageLeftRim", "stageRightRim",
			"spotlight", "doll", "dollHead", "dollArms", "dollBody", "clapEffect",
			"sword", "swordObj", "shadowHead", "shadowLeftArm", "shadowLeftGlove",
			"shadowLeftGloveRim", "shadowRightArm", "shadowRightGlove", "shadowRightGloveRim",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"Background", "spotlight", "dollHead", "swordObj"}},
		},
	},
	"coinToss": {
		dir:    "CoinToss",
		prefab: "coinToss.prefab",
		roleFields: []string{
			"fg", "bg", "imageBG", "handAnimator", "manHand",
			"handHolder", "manHolder", "imageAnim",
		},
		wantControllers: true,
		commonSounds:    []string{"applause.ogg", "audienceSad.ogg"},
	},
	"cropStomp": {
		dir:    "CropStomp",
		prefab: "cropStomp.prefab",
		roleFields: []string{
			"baseVeggie", "baseMole", "legsAnim", "bodyAnim", "farmerTrans",
			"grass", "Dots", "BG", "grassTrans", "dotsTrans", "scrollingHolder",
			"veggieHolder", "farmer", "pickCurve", "moleCurve", "hitParticle",
		},
		curveFields:     []string{"pickCurve", "moleCurve"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"baseVeggie", "baseMole", "legsAnim", "bodyAnim", "farmerTrans", "pickCurve", "moleCurve", "hitParticle"}, curveFields: []string{"pickCurve", "moleCurve"}},
			{name: "farmer", markers: []string{"collectedHolder", "plantLeftRef", "plantRightRef", "plantLastRef", "veggieSprites", "startPlant"}},
			{name: "veggie", markers: []string{"veggieSprites", "veggieSprite", "veggieTrans", "curve"}, atPath: "ScrollingItems/Prefabs/Veggie", curveFields: []string{"curve"}},
			{name: "mole", markers: []string{"isMole", "moleAnim", "veggieSprite", "veggieTrans", "curve"}, atPath: "ScrollingItems/Prefabs/Mole", curveFields: []string{"curve"}},
		},
	},
	"fallingWaffle": {
		dir:             "FallingWaffle",
		prefab:          "fallingWaffle.prefab",
		roleFields:      []string{"waffleAnim", "squareAnim"},
		wantControllers: true,
	},
	"loveLizards": {
		dir:    "LoveLizards",
		prefab: "loveLizards.prefab",
		roleFields: []string{
			"MaleLizard", "FemaleLizard", "Guide",
			"background1", "background2", "background3",
		},
		wantControllers: true,
	},
	"mannequinFactory": {
		dir:    "MannequinFactory",
		prefab: "mannequinFactory.prefab",
		roleFields: []string{
			"HandAnim", "StampAnim", "bg", "SignText", "MannequinHeadObject",
		},
		wantControllers: true,
		wantTexts:       true,
		components: []componentSpec{
			{name: "head", markers: []string{"headSr", "heads", "eyesSr", "eyes", "headAnim"}, atPath: "MannequinHeadHolder/MannequinHead"},
		},
	},
	"cannery": {
		dir:    "Cannery",
		prefab: "cannery.prefab",
		roleFields: []string{
			"can", "blackout", "conveyorBeltAnim", "alarmAnim", "dingAnim", "cannerAnim",
		},
		refArrayFields:  []string{"bgAnims"},
		wantControllers: true,
		components: []componentSpec{
			{name: "can", markers: []string{"anim"}, atPath: "CanParent"},
		},
	},
	"fireworks": {
		dir:             "Fireworks",
		prefab:          "fireworks.prefab",
		wantControllers: true,
		commonSounds:    []string{"applause.ogg"},
	},
	"wizardsWaltz": {
		dir:             "WizardsWaltz",
		prefab:          "wizardsWaltz.prefab",
		roleFields:      []string{"wizard", "girl", "plantHolder", "plantBase"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"wizard", "girl", "plantHolder", "plantBase"}},
			{name: "wizard", markers: []string{"animator", "shadow"}, atPath: "Wizard"},
			{name: "girl", markers: []string{"animator", "flowers"}, atPath: "Girl"},
			{name: "plant", markers: []string{"animator", "spriteRenderer", "createBeat"}, atPath: "Prefabs/Plant"},
		},
	},
	"basketballGirls": {
		dir:             "BasketballGirls",
		prefab:          "basketballGirls.prefab",
		roleFields:      []string{"baseBall", "girlLeftAnim", "girlRightAnim", "goalAnim", "BGPlane"},
		refArrayFields:  []string{"CameraPosition"},
		wantControllers: true,
	},
	"drummingPractice": {
		dir:    "DrummingPractice",
		prefab: "drummingPractice.prefab",
		roleFields: []string{
			"background", "backgroundGradient", "player", "leftDrummer",
			"rightDrummer", "hitPrefab", "NPCDrummers",
		},
		wantControllers: true,
		commonSounds:    []string{"applause.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"background", "backgroundGradient", "streaks", "player", "leftDrummer", "rightDrummer"}},
			{name: "drummer", markers: []string{"animator", "miiFaces", "face"}, multi: true},
		},
	},
	"dogNinja": {
		dir:             "DogNinja",
		prefab:          "dogNinja.prefab",
		roleFields:      []string{"DogAnim", "BirdAnim", "ObjectBase", "CutEverythingText"},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"DogAnim", "BirdAnim", "ObjectBase", "ObjectTypes"}},
			{name: "throwObject", markers: []string{"LeftCurve", "RightCurve", "BarelyLeftCurve", "BarelyRightCurve", "HalvesLeftBase", "HalvesRightBase", "objectLeftHalves", "objectRightHalves"}, curveFields: []string{"LeftCurve", "RightCurve", "BarelyLeftCurve", "BarelyRightCurve"}},
			{name: "halves", markers: []string{"fallLeftCurve", "fallRightCurve", "rotSpeed", "sr"}, multi: true, curveFields: []string{"fallLeftCurve", "fallRightCurve"}},
		},
	},
	"frogPrincess": {
		dir:    "FrogPrincess",
		prefab: "frogPrincess.prefab",
		roleFields: []string{
			"frogAnim", "princessAnim", "Leaves", "Lotuses", "splashEffect", "BGPlane",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"frogAnim", "princessAnim", "Leaves", "Lotuses", "moveDistance", "moveTime"}},
		},
	},
	"holeInOne": {
		dir:    "HoleInOne",
		prefab: "holeInOne.prefab",
		roleFields: []string{
			"baseBall", "MonkeyAnim", "MonkeyHeadAnim", "MandrillAnim", "GolferAnim",
			"Hole", "HoleAnim", "GrassEffectAnim", "BallEffectAnim", "grassEffectPrefab", "grassArea",
		},
		wantControllers: true,
		extraSounds:     []extraSound{{dir: "ClappyTrio", rel: "sign.ogg"}},
		components: []componentSpec{
			{name: "game", markers: []string{"baseBall", "MonkeyAnim", "MonkeyHeadAnim", "MandrillAnim", "GolferAnim", "Hole", "HoleAnim", "GrassEffectAnim", "BallEffectAnim", "grassEffectPrefab", "grassArea"}},
			{name: "ball", markers: []string{"curve", "ballSR", "shadowSR", "bigBallSR", "bigShadowSR"}, atPath: "Golfball", curveArrayFields: []string{"curve"}},
		},
	},
	"forkLifter": {
		dir:    "ForkLifter",
		prefab: "forkLifter.prefab",
		roleFields: []string{
			"ForkLifterHand", "handAnim", "flickedObject", "peaPreview",
			"bg", "gradientFiller", "mmLines", "viewerCircle", "viewerCircleBg",
			"playerShadow", "handShadow", "forkSR",
		},
		refArrayFields:  []string{"Gradients", "forkEffects"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"ForkLifterHand", "handAnim", "flickedObject", "peaPreview", "peaSprites", "peaHitSprites"}},
			{name: "hand", markers: []string{"fastSprite", "fastSprites"}, atPath: "Hand"},
			{name: "player", markers: []string{"hitFX", "hitFXG", "hitFXMiss", "hitFX2", "early", "perfect", "late"}, atPath: "Player"},
		},
	},
	"gleeClub": {
		dir:    "GleeClub",
		prefab: "gleeClub.prefab",
		roleFields: []string{
			"heartAnim", "condAnim", "leftChorusKid", "middleChorusKid", "playerChorusKid",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"heartAnim", "condAnim", "leftChorusKid", "middleChorusKid", "playerChorusKid", "kidMaterial", "bgMaterial"}},
			{name: "kid", markers: []string{"anim", "sr", "player"}, multi: true},
		},
	},
	"clappyTrio": {
		dir:             "ClappyTrio",
		prefab:          "clappyTrio.prefab",
		roleFields:      []string{"customText", "signAnim", "textTrioTiming", "textCustom"},
		wantControllers: true,
		wantTexts:       true,
		components: []componentSpec{
			{name: "game", markers: []string{"Lion", "faces", "signAnim", "textTrioTiming", "textCustom"}},
		},
	},
	"rhythmSomen": {
		dir:    "RhythmSomen",
		prefab: "rhythmSomen.prefab",
		roleFields: []string{
			"SomenPlayer", "FrontArm", "backArm", "EffectHit", "EffectSweat",
			"EffectExclam", "EffectShock", "CloseCrane", "FarCrane",
		},
	},
	"trickClass": {
		dir:            "TrickClass",
		prefab:         "trickClass.prefab",
		roleFields:     []string{"playerAnim", "girlAnim", "warnAnim", "objHolder"},
		refArrayFields: []string{"objPrefab", "objPrefabVariant"},
		strArrayFields: []string{"objWarnAnim", "objWarnAnimVariant", "objThrowAnim"},
		curveFields:    []string{"ballTossCurve", "ballMissCurve", "planeTossCurve", "planeMissCurve", "shockTossCurve"},
		objMarkers:     []string{"flyBeats", "dodgeBeats"},
		wantSequences:  true,
		commonSounds:   []string{"miss.wav"},
	},
	"totemClimb": {
		dir:    "TotemClimb",
		prefab: "totemClimb.prefab",
		roleFields: []string{
			"_cameraTransform", "_scrollTransform", "_jumper", "_totemManager",
			"_birdManager", "_groundHolder", "_fakeTotemHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"_scrollSpeedX", "_scrollTransform"}},
			{name: "jumper", markers: []string{"_jumpHeight", "_initialPoint"}},
			{name: "totemManager", markers: []string{"_xDistance", "_totemTransform"}},
			{name: "birdManager", markers: []string{"_birdRef", "_speedX"}},
			{name: "groundManager", markers: []string{"_groundFirst"}},
			{name: "pillarManager", markers: []string{"_pillarFirst"}},
			{name: "backgroundManager", markers: []string{"_objectsParent"}},
			{name: "totem", markers: []string{"_anim", "_jumperPoint"}, atPath: "Game/Scrollable/Totems/Totem"},
			{name: "dragon", markers: []string{"_anim", "_jumperPoint"}, atPath: "Game/Scrollable/Totems/Dragon"},
			{name: "frog", markers: []string{"_animLeft", "_jumperPointLeft"}},
		},
	},
	"tunnel": {
		dir:    "Tunnel",
		prefab: "tunnel.prefab",
		roleFields: []string{
			"tunnelWall", "tunnelWallRenderer", "frontHand", "cowbellAnimator", "driverAnimator",
		},
		refArrayFields:  []string{"bg"},
		curveFields:     []string{"handCurve"},
		wantControllers: true,
		commonSounds:    []string{"count-ins/cowbell.wav", "miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"tunnelWall", "tunnelWallRenderer", "tunnelChunksPerSec", "tunnelWallChunkSize"}},
		},
	},
	"tramAndPauline": {
		dir:             "TramAndPauline",
		prefab:          "tramAndPauline.prefab",
		roleFields:      []string{"tram", "pauline", "curtainAnim", "audienceAnim"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"tram", "pauline", "curtainAnim", "audienceAnim"}},
			{name: "kid", markers: []string{"rootBody", "trampolineAnim", "bodyAnim", "transformParticle", "smokeParticle", "jumpHeight"}, multi: true},
		},
	},
	"seeSaw": {
		dir:    "SeeSaw",
		prefab: "seeSaw.prefab",
		roleFields: []string{
			"seeSawAnim", "see", "saw", "leftWhiteOrbs", "rightBlackOrbs",
			"gradient", "bgLow", "bgHigh",
		},
		refArrayFields:  []string{"recolors"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"jumpPaths", "seeSawAnim"}},
			{name: "see", markers: []string{"landOutTrans", "deathParticle"}, atPath: "Game/Guys/SeeHolder"},
			{name: "saw", markers: []string{"landOutTrans", "deathParticle"}, atPath: "Game/Guys/SawHolder"},
		},
	},
	"blueBear": {
		dir:    "BlueBear",
		prefab: "blueBear.prefab",
		roleFields: []string{
			"headAndBodyAnim", "bagsAnim", "donutBagAnim", "cakeBagAnim", "windAnim",
			"leftCrumb", "rightCrumb", "_storyAnim", "donutBase", "cakeBase",
			"crumbsBase", "foodHolder", "crumbsHolder", "individualBagHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"_treatCurves", "donutGradient"}},
		},
	},
	"boardMeeting": {
		dir:             "BoardMeeting",
		prefab:          "boardMeeting.prefab",
		roleFields:      []string{"farLeft", "farRight", "assistantAnim"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"cheerReaders": {
		dir:    "CheerReaders",
		prefab: "cheerReaders.prefab",
		roleFields: []string{
			"playerMask", "missPoster", "topPoster", "middlePoster", "bottomPoster",
			"player", "whiteYayParticle", "blackYayParticle",
			"CheerCaption0", "CheerCaption1", "CheerUnderlay0", "CheerUnderlay1",
			"StickyCaptions",
		},
		refArrayFields: []string{
			"firstRow", "secondRow", "thirdRow", "topMasks", "middleMasks", "bottomMasks",
		},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"posters", "topMasks"}},
			{name: "girl", markers: []string{"faceAnim", "blushLeft"}, multi: true},
		},
	},
	"tapTrial": {
		dir:    "TapTrial",
		prefab: "tapTrial.prefab",
		roleFields: []string{
			"player", "monkeyL", "monkeyR", "giraffe",
			"rootPlayer", "rootMonkeyL", "rootMonkeyR", "flash",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"tambourine": {
		dir:    "Tambourine",
		prefab: "tambourine.prefab",
		roleFields: []string{
			"handsAnimator", "bg", "monkeyAnimator", "flowerParticles",
			"happyFace", "sadFace", "sweatAnimator", "frogAnimator",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"sneakySpirits": {
		dir:    "SneakySpirits",
		prefab: "sneakySpirits.prefab",
		roleFields: []string{
			"bowAnim", "bowHolderAnim", "doorAnim", "arrowMissPrefab", "ghostMissPrefab",
			"deathGhostPrefab", "normalRain", "slowRain", "normalTree", "slowTree",
		},
		refArrayFields:  []string{"ghostPositions"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"airRally": {
		dir:    "AirRally",
		prefab: "airRally.prefab",
		roleFields: []string{
			"Baxter", "Forthington", "Shuttlecock", "objHolder",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"spaceDance": {
		dir:    "SpaceDance",
		prefab: "spaceDance.prefab",
		roleFields: []string{
			"bg", "shootingStarAnim", "DancerP", "Dancer1", "Dancer2", "Dancer3",
			"Gramps", "Hit", "Player",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
	},
	"spaceball": {
		dir:    "Spaceball",
		prefab: "spaceball.prefab",
		roleFields: []string{
			"bg", "square", "room", "hole", "shadow", "shadow2",
			"Ball", "BallsHolder", "Dispenser", "Dust", "alien",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{"bg", "square", "room", "hole", "shadow", "shadow2", "BallSprites", "CostumeColors"}},
			{name: "ball", markers: []string{"Holder", "Sprite", "pitchLowCurve", "pitchHighCurve", "pitchQuickCurve", "pitchOffbeatCurve"}, atPath: "Balls/Ball",
				curveFields: []string{"pitchLowCurve", "pitchHighCurve", "pitchQuickCurve", "pitchOffbeatCurve"}},
			{name: "player", markers: []string{"PlayerSprite", "Hat", "Bat", "BatColors", "HatSprites1"}, atPath: "Player"},
		},
	},
	"splashdown": {
		dir:    "Splashdown",
		prefab: "splashdown.prefab",
		roleFields: []string{
			"synchretteHolder", "synchrettePrefab", "crowdAnim",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefabs/SynchretteHolder.prefab",
			"Prefabs/Splashes.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{"synchretteHolder", "synchrettePrefab", "crowdAnim", "synchretteDistance"}},
			{name: "synchrette", markers: []string{"splashPrefab", "anim", "synchretteTransform", "splashHolder", "throwAnim"}, atPath: "SynchretteHolder"},
			{name: "splash", markers: []string{"smallSplashParticles", "bigSplashParticles"}},
		},
	},
	"sumoBrothers": {
		dir:    "SumoBrothers",
		prefab: "sumoBrothers.prefab",
		roleFields: []string{
			"inuSensei", "sumoBrotherP", "sumoBrotherG", "sumoBrotherGHead",
			"sumoBrotherPHead", "impact", "glasses", "dust", "bgMove", "bgStatic",
			"confetti", "bgTop", "bgBtm",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"inuSensei", "sumoBrotherP", "sumoBrotherG", "sumoBrotherGHead",
				"sumoBrotherPHead", "impact", "glasses", "dust", "bgMove", "bgStatic",
				"confetti", "backgroundMaterial", "bgTop", "bgBtm", "mawashiMaterial",
				"cameraX", "cameraXNew", "stompShakeSpeed",
			}},
		},
	},
	"warioDeMambo": {
		dir:    "WarioDeMambo",
		prefab: "warioDeMambo.prefab",
		roleFields: []string{
			"commandText", "endPose", "spotlightLSprite", "spotlightRSprite",
			"spotlightLTrans", "spotlightRTrans", "DancerLSpotPos",
			"DancerRSpotPos", "WarioSpotPos", "textAnimator",
			"dancerLeftAnim", "dancerLeftArmAnim", "dancerLeftHeadAnim", "dancerLeftJumpAnim",
			"dancerRightAnim", "dancerRightArmAnim", "dancerRightHeadAnim", "dancerRightJumpAnim",
			"warioBodyAnim", "warioArmAnim", "warioFaceAnim", "warioJumpAnim",
			"topLightAnim", "leftLightAnim", "rightLightAnim",
		},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{
				"commandText", "endPose", "spotlightLSprite", "spotlightRSprite",
				"spotlightLTrans", "spotlightRTrans", "DancerLSpotPos", "DancerRSpotPos",
				"WarioSpotPos", "mainMat", "lightMat", "floorLightMat",
				"blueAddColor", "redAddColor", "textAnimator",
				"dancerLeftAnim", "dancerLeftArmAnim", "dancerLeftHeadAnim", "dancerLeftJumpAnim",
				"dancerRightAnim", "dancerRightArmAnim", "dancerRightHeadAnim", "dancerRightJumpAnim",
				"warioBodyAnim", "warioArmAnim", "warioFaceAnim", "warioJumpAnim",
				"topLightAnim", "leftLightAnim", "rightLightAnim",
			}},
		},
	},
	"kitties": {
		dir:    "Kitties",
		prefab: "kitties.prefab",
		roleFields: []string{
			"player", "Fish", "background",
		},
		refArrayFields:  []string{"kitties", "Cats"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "playerScript", markers: []string{"Player", "fish"}},
		},
	},
	"lockstep": {
		dir:    "Lockstep",
		prefab: "lockstep.prefab",
		roleFields: []string{
			"stepswitcherPlayer", "stepswitcherLeft", "stepswitcherRight", "bach",
			"masterStepperAnim", "masterStepperSprite", "background",
		},
		refArrayFields:  []string{"slaveSteppers"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"marchingOrders": {
		dir:    "MarchingOrders",
		prefab: "marchingOrders.prefab",
		roleFields: []string{
			"Sarge", "Steam", "CadetPlayer", "CadetHeadPlayer",
		},
		refArrayFields:  []string{"Cadets", "CadetHeads", "BackgroundRecolorable", "RecolorMats", "ConveyorGo"},
		wantControllers: true,
		wantSequences:   true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
	},
	"mrUpbeat": {
		dir:    "MrUpbeat",
		prefab: "mrUpbeat.prefab",
		roleFields: []string{
			"metronomeAnim", "man", "bg",
		},
		refArrayFields:  []string{"shadowSr"},
		wantControllers: true,
		wantTexts:       true,
		commonSounds:    []string{"applause.ogg", "miss.wav", "nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"metronomeAnim", "man", "blipMaterial", "bg", "shadowSr"}},
			{name: "man", markers: []string{"anim", "blipAnim", "antennaLight", "shadows", "blipText"}, atPath: "MrUpbeat"},
		},
	},
	"munchyMonk": {
		dir:    "MunchyMonk",
		prefab: "munchyMonk.prefab",
		roleFields: []string{
			"Baby", "BrowHolder", "StacheHolder", "DumplingObj", "CloudMonkey",
			"OneGiverAnim", "TwoGiverAnim", "ThreeGiverAnim", "BrowAnim",
			"StacheAnim", "MonkHolderAnim", "MonkAnim", "MonkArmsAnim",
		},
		wantControllers: true,
		wantSequences:   true,
		components: []componentSpec{
			{name: "game", markers: []string{"dumplingSprites", "MonkAnim"}},
			{name: "scroll", markers: []string{"XSpeed", "PositiveBounds"}, multi: true},
		},
	},
	"meatGrinder": {
		dir:    "MeatGrinder",
		prefab: "meatGrinder.prefab",
		roleFields: []string{
			"GrinderText", "MeatBase", "MeatSplash",
			"BossAnim", "TackAnim", "CartGuyParentAnim", "CartGuyAnim",
		},
		refArrayFields:  []string{"Gears"},
		objMarkers:      []string{"meatFlyHeight", "meatFlyHeightAlt"}, // Meat.cs
		objRefFields:    []string{"startPosition", "startPositionAlt", "hitPosition", "missPosition"},
		objSpriteFields: []string{"meats"},
		wantControllers: true,
		wantTexts:       true,
	},
	"showtime": {
		dir:    "Showtime",
		prefab: "showtime.prefab",
		roleFields: []string{
			"MonkeyAnim", "ButtonAnim", "LauncherAnim", "blockOneAnim", "blockTwoAnim",
			"penguinStart", "ballStart", "leapStart", "fallStart", "destroyerPoint", "slideStart",
		},
		curveFields: []string{
			"entryCurve", "hopCurve", "leapCurve", "fallCurve", "exitCurve", "chuteCurve",
			"ballUpCurve", "ballDownCurve",
		},
		wantControllers: true,
		extraSounds: []extraSound{
			{dir: "SneakySpirits", rel: "moving.ogg"},
		},
		templatePrefabs: []string{
			"Prefabs/penguinGray.prefab",
			"Prefabs/penguinWhite.prefab",
			"Prefabs/penguinBig.prefab",
			"Prefabs/showtimeBall.prefab",
		},
	},
	"slotMonster": {
		dir:    "SlotMonster",
		prefab: "slotMonster.prefab",
		roleFields: []string{
			"smAnim", "winParticles",
		},
		refArrayFields:  []string{"eyeAnims", "buttons"},
		wantControllers: true,
		commonSounds:    []string{"bassDrumNTR.wav", "snareDrumNTR.wav", "nearMiss.ogg"},
		components: []componentSpec{
			{name: "button", markers: []string{"pressed", "color", "input", "missed", "anim", "srs"}, multi: true},
		},
	},
	"djSchool": {
		dir:             "DJSchool",
		prefab:          "djSchool.prefab",
		roleFields:      []string{"student", "djYellow", "djYellowScript"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{"student", "djYellow", "djYellowScript"}},
			{name: "student", markers: []string{"flash", "flashFX", "flashFXInverse", "TurnTable"}},
			{name: "djYellow", markers: []string{"djYellowHeadSprites", "djYellowHeadSprite"}},
		},
	},
}

func bundlePath(dir string, parts ...string) string {
	return filepath.Join(append([]string{*hsRoot, "Assets", "Bundled", "Games", dir}, parts...)...)
}

func assetsPath(parts ...string) string {
	return filepath.Join(append([]string{*hsRoot, "Assets"}, parts...)...)
}

func (s sceneSpec) prefabFile() string {
	if s.prefabPath != "" {
		return assetsPath(filepath.FromSlash(s.prefabPath))
	}
	return bundlePath(s.dir, s.prefab)
}

func (s sceneSpec) gameRoot() string {
	if s.prefabPath != "" {
		return filepath.Dir(s.prefabFile())
	}
	return bundlePath(s.dir)
}

func (s sceneSpec) soundRoot() string {
	return filepath.Join(s.gameRoot(), "Sounds")
}

// mappingShaderGUIDs 是调色板映射 shader（CellAnime_MappedInvert /
// CellAnime_Mapped）的 guid：贴图 RGB 通道为掩码权重，
// out = ColorAlpha·r + ColorBravo·g + ColorDelta·b。
var mappingShaderGUIDs = []string{
	"d6702951943fe3f48b9e437dd725e76f", // CellAnime_MappedInvert
	"ff54fed5718ccc543808dec1f266d1c8", // CellAnime_Mapped
	"0fd674cb47a44464ab7cd46f6b8e2422", // ChargingChicken ChickenCar/Cell channel mask
	"05b2b41ae5e852e44a848016376434c8", // ChargingChicken ChickenCloud/Star channel mask
	"98ff3747664c4c240a800e54e6f5fdf7", // ChargingChicken ChickenMirage channel mask
	"c3025c1be707f9c45ba76feb4ddfa60b", // ChargingChicken ChickenWater channel mask
}

// scanMappedMats 扫描游戏目录下使用映射 shader 的材质，guid → 文件主名。
func scanMappedMats(root string) map[string]string {
	out := map[string]string{}
	for guid, p := range scanGUIDs(root, ".mat") {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, sg := range mappingShaderGUIDs {
			if strings.Contains(string(raw), sg) {
				out[guid] = strings.TrimSuffix(filepath.Base(p), ".mat")
				break
			}
		}
	}
	if len(out) > 0 {
		fmt.Printf("mapped materials: %d\n", len(out))
	}
	return out
}

func scanMaterialNames(root string) map[string]string {
	out := map[string]string{}
	for guid, p := range scanGUIDs(root, ".mat") {
		out[guid] = strings.TrimSuffix(filepath.Base(p), ".mat")
	}
	return out
}

func extractScene(game string) {
	spec, ok := sceneSpecs[game]
	if !ok {
		log.Fatalf("unknown game %q (known: karateman, rhythmSomen)", game)
	}
	must(os.MkdirAll(filepath.Join(*outDir, "sounds"), 0o755))

	tables := scanSceneSpriteMetas(spec)
	exportSheetMulti(tables)
	idx, docs := buildPrefabIndex(spec.prefabFile(), spec.templatePrefabs)
	idx.mappedMats = scanMappedMats(spec.gameRoot())
	idx.matNames = scanMaterialNames(spec.gameRoot())
	paths, nodeIdx := exportScene(spec, idx, tables)
	exportRoles(spec, docs, idx, paths)
	exportExtra(spec, docs, idx, paths, nodeIdx, tables)
	exportMaterials(spec, docs, paths)
	exportMeshes(spec, docs, idx, paths)
	exportParticles(spec, docs, paths, tables)
	exportAnimDir(spec, tables)
	if spec.wantControllers {
		exportControllers(spec, docs, idx, paths)
	}
	if spec.synthesizeAnimPaths {
		synthesizeSceneAnimationPaths()
	}
	if spec.wantTexts {
		exportTexts(docs, paths)
	}
	copySounds(spec.soundRoot())
	for _, name := range spec.commonSounds {
		b, err := os.ReadFile(filepath.Join(*hsRoot, "Assets", "Resources", "Sfx", name))
		must(err)
		// 公共音效加 common_ 前缀避免与游戏音效重名
		outName := "common_" + strings.NewReplacer("/", "_", "\\", "_").Replace(name)
		must(os.WriteFile(filepath.Join(*outDir, "sounds", outName), b, 0o644))
	}
	for _, snd := range spec.extraSounds {
		b, err := os.ReadFile(bundlePath(snd.dir, "Sounds", filepath.FromSlash(snd.rel)))
		must(err)
		outName := snd.out
		if outName == "" {
			outName = snd.rel
		}
		dst := filepath.Join(*outDir, "sounds", filepath.FromSlash(outName))
		must(os.MkdirAll(filepath.Dir(dst), 0o755))
		must(os.WriteFile(dst, b, 0o644))
	}
	fmt.Println("done.")
}

func scanSceneSpriteMetas(spec sceneSpec) map[string]*spriteTable {
	root := spec.spriteRoot()
	if spec.noSprites {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			fmt.Printf("scanned 0 sprite metas (no Sprites directory for %s)\n", spec.gameRoot())
			return map[string]*spriteTable{}
		}
	}
	return scanSpriteMetas(root)
}

func (s sceneSpec) spriteRoot() string {
	dir := s.spritesDir
	if dir == "" {
		dir = "Sprites"
	}
	return filepath.Join(s.gameRoot(), filepath.FromSlash(dir))
}

func (s sceneSpec) animRoot() string {
	dir := s.animsDir
	if dir == "" {
		dir = s.spritesDir
	}
	if dir == "" {
		dir = "Sprites"
	}
	return filepath.Join(s.gameRoot(), filepath.FromSlash(dir))
}

// ---------- 多图集 ----------

func exportSheetMulti(tables map[string]*spriteTable) {
	guids := make([]string, 0, len(tables))
	for g := range tables {
		guids = append(guids, g)
	}
	sort.Slice(guids, func(i, j int) bool { return tables[guids[i]].pngPath < tables[guids[j]].pngPath })

	sheet := &kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}}
	for _, g := range guids {
		t := tables[g]
		atlasIdx := len(sheet.Atlases)
		name := fmt.Sprintf("atlas%d.png", atlasIdx)
		raw, err := os.ReadFile(t.pngPath)
		must(err)
		must(os.WriteFile(filepath.Join(*outDir, name), raw, 0o644))
		sheet.Atlases = append(sheet.Atlases, name)

		if len(t.sheet) == 0 {
			// 单 sprite 贴图：整图作为一个切片
			base := strings.TrimSuffix(filepath.Base(t.pngPath), ".png")
			sheet.Sprites[base] = kmdata.SpriteInfo{
				X: 0, Y: 0, W: t.texW, H: t.texH,
				PivotX: 0.5, PivotY: 0.5, Atlas: atlasIdx, PPU: t.ppu,
			}
			continue
		}
		base := strings.TrimSuffix(filepath.Base(t.pngPath), ".png")
		for sname, sp := range t.sheet {
			sp.Atlas = atlasIdx
			sp.PPU = t.ppu
			key := sname
			if _, dup := sheet.Sprites[key]; dup {
				// 跨贴图同名切片（cheerReaders 14 张海报均为
				// TopPart/MiddlePart/BottomPart/Miss）：按文件名命名空间。
				key = base + "/" + sname
				for id, n := range t.byID {
					if n == sname {
						t.byID[id] = key
					}
				}
			}
			sheet.Sprites[key] = sp
		}
	}
	writeJSON("sprites.json", sheet)
	fmt.Printf("sheet: %d atlases, %d sprites\n", len(sheet.Atlases), len(sheet.Sprites))
}

// ---------- prefab 全树 ----------

type docTable struct {
	byID map[int64]*docRef
}
type docRef struct {
	classID int
	content map[string]any
}

func buildPrefabIndex(prefabPath string, templatePrefabs []string) (*prefabIndex, *docTable) {
	// 展开嵌套 prefab（子 prefab 在游戏目录与共享 Prefabs 下扫描）
	gameDir := filepath.Dir(prefabPath)
	prefabGUIDs := scanGUIDs(gameDir, ".prefab")
	docs, err := expandPrefab(prefabPath, prefabGUIDs)
	must(err)
	if len(templatePrefabs) > 0 {
		docs = appendTemplatePrefabs(docs, gameDir, templatePrefabs, prefabGUIDs)
	}
	fmt.Printf("prefab: %d documents（含嵌套展开）\n", len(docs))

	idx := &prefabIndex{
		goName: map[int64]string{}, tfByGO: map[int64]map[string]any{},
		tfByID: map[int64]map[string]any{}, tfOwner: map[int64]int64{},
		rendByGO: map[int64]map[string]any{}, goActive: map[int64]bool{},
		groupByGO: map[int64][]int{},
		maskByGO:  map[int64]map[string]any{},
	}
	dt := &docTable{byID: map[int64]*docRef{}}
	for i := range docs {
		d := &docs[i]
		c := d.Content()
		dt.byID[d.FileID] = &docRef{classID: d.ClassID, content: c}
		switch d.ClassID {
		case 1: // GameObject
			idx.goName[d.FileID] = uy.S(c["m_Name"])
			idx.goActive[d.FileID] = uy.I(c["m_IsActive"]) != 0
		case 4, 224: // Transform / RectTransform（TMP 文本节点）
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.tfByGO[gid] = c
			idx.tfByID[d.FileID] = c
			idx.tfOwner[d.FileID] = gid
		case 212: // SpriteRenderer
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.rendByGO[gid] = c
		case 210: // SortingGroup
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.groupByGO[gid] = []int{int(uy.I(c["m_SortingLayer"])), int(uy.I(c["m_SortingOrder"]))}
		case 331: // SpriteMask
			gid := uy.I(uy.Get(c, "m_GameObject", "fileID"))
			idx.maskByGO[gid] = c
		}
	}
	idx.rebuildChildren()
	return idx, dt
}

func (idx *prefabIndex) rebuildChildren() {
	for childID, tf := range idx.tfByID {
		parentID := uy.I(uy.Get(tf, "m_Father", "fileID"))
		if parentID == 0 {
			continue
		}
		parent := idx.tfByID[parentID]
		if parent == nil {
			continue
		}
		kids := uy.L(parent["m_Children"])
		found := false
		for _, kv := range kids {
			if uy.I(uy.Get(uy.M(kv), "fileID")) == childID {
				found = true
				break
			}
		}
		if !found {
			parent["m_Children"] = append(kids, map[string]any{"fileID": childID})
		}
	}
}

func appendTemplatePrefabs(mainDocs []uy.Doc, gameDir string, rels []string, prefabGUIDs map[string]string) []uy.Doc {
	rootTF := findSceneRootTF(mainDocs)
	if rootTF == 0 {
		log.Fatal("prefab root transform not found before template append")
	}
	byID := map[int64]bool{}
	tfDocs := map[int64]*uy.Doc{}
	for i := range mainDocs {
		byID[mainDocs[i].FileID] = true
		switch mainDocs[i].ClassID {
		case 4, 224:
			tfDocs[mainDocs[i].FileID] = &mainDocs[i]
		}
	}
	rootDoc := tfDocs[rootTF]
	if rootDoc == nil {
		log.Fatal("prefab root transform doc missing before template append")
	}
	rootContent := rootDoc.Content()
	for _, rel := range rels {
		path := filepath.Join(gameDir, rel)
		docs, err := expandPrefab(path, prefabGUIDs)
		must(err)
		extRootTF := findSceneRootTF(docs)
		if extRootTF == 0 {
			log.Fatalf("template prefab %s root transform not found", rel)
		}
		if hasDocIDCollision(docs, byID) {
			// Standalone prefab assets often reuse Unity local fileIDs. Main prefab
			// references by GUID+fileID are only needed when a script field is
			// exported as a role/component ref; template-only prefabs can be safely
			// moved into the nested-prefab ID namespace as long as their internal
			// references are rewritten together.
			remap := map[int64]int64{}
			for i := range docs {
				remap[docs[i].FileID] = nestedNextID
				nestedNextID++
			}
			for i := range docs {
				docs[i].FileID = remap[docs[i].FileID]
				remapRefs(docs[i].Content(), remap)
			}
			extRootTF = remap[extRootTF]
		}
		for i := range docs {
			if byID[docs[i].FileID] {
				log.Fatalf("template prefab %s fileID collision after remap on &%d", rel, docs[i].FileID)
			}
			byID[docs[i].FileID] = true
		}
		for i := range docs {
			d := &docs[i]
			if d.ClassID == 4 || d.ClassID == 224 {
				if d.FileID == extRootTF {
					c := d.Content()
					c["m_Father"] = map[string]any{"fileID": rootTF}
					rootContent["m_Children"] = append(uy.L(rootContent["m_Children"]), map[string]any{"fileID": extRootTF})
				}
			}
			if d.ClassID == 1 {
				gid := d.FileID
				for _, td := range docs {
					if td.FileID == extRootTF {
						gid = uy.I(uy.Get(td.Content(), "m_GameObject", "fileID"))
						break
					}
				}
				if gid == d.FileID {
					d.Content()["m_IsActive"] = 0
				}
			}
		}
		mainDocs = append(mainDocs, docs...)
		fmt.Printf("template prefab %s appended (%d docs)\n", rel, len(docs))
	}
	return mainDocs
}

func hasDocIDCollision(docs []uy.Doc, byID map[int64]bool) bool {
	for i := range docs {
		if byID[docs[i].FileID] {
			return true
		}
	}
	return false
}

func findSceneRootTF(docs []uy.Doc) int64 {
	tfByID := map[int64]map[string]any{}
	for i := range docs {
		if docs[i].ClassID != 4 && docs[i].ClassID != 224 {
			continue
		}
		tfByID[docs[i].FileID] = docs[i].Content()
	}
	for id, tf := range tfByID {
		father := uy.I(uy.Get(tf, "m_Father", "fileID"))
		if father == 0 || tfByID[father] == nil {
			return id
		}
	}
	return 0
}

// exportScene 导出整棵节点树，返回 GameObject fileID → 节点 path（供 roles 解析）
// 与 GameObject fileID → 节点下标（path 重名时按下标寻址）。
func exportScene(spec sceneSpec, idx *prefabIndex, tables map[string]*spriteTable) (map[int64]string, map[int64]int) {
	// 根 Transform：m_Father 不在本 prefab 内
	var rootTF map[string]any
	for tfID, tf := range idx.tfByID {
		father := uy.I(uy.Get(tf, "m_Father", "fileID"))
		if father == 0 {
			if rootTF != nil {
				log.Printf("warn: multiple root transforms, keeping first (extra &%d)", tfID)
				continue
			}
			rootTF = tf
		}
	}
	if rootTF == nil {
		for tfID, tf := range idx.tfByID {
			father := uy.I(uy.Get(tf, "m_Father", "fileID"))
			if father != 0 && idx.tfByID[father] == nil {
				if rootTF != nil {
					log.Printf("warn: multiple missing-parent root transforms, keeping first (extra &%d)", tfID)
					continue
				}
				rootTF = tf
			}
		}
	}
	if rootTF == nil {
		log.Fatal("prefab root transform not found")
	}

	scene := &kmdata.Rig{}
	paths := map[int64]string{}
	nodeIdx := map[int64]int{}
	pathIdx := map[string]int{}
	var walk func(tf map[string]any, parent int, path string)
	walk = func(tf map[string]any, parent int, path string) {
		gid := uy.I(uy.Get(tf, "m_GameObject", "fileID"))
		paths[gid] = path
		pos := [2]float64{
			uy.F(uy.Get(tf, "m_LocalPosition", "x")),
			uy.F(uy.Get(tf, "m_LocalPosition", "y")),
		}
		// RectTransform：本地位置由 m_AnchoredPosition 驱动（点锚 + 非 Rect 父
		// 节点时即等于 localPosition.xy；m_LocalPosition 是序列化残留）
		if ap, ok := tf["m_AnchoredPosition"]; ok {
			pos = [2]float64{uy.F(uy.Get(uy.M(ap), "x")), uy.F(uy.Get(uy.M(ap), "y"))}
		}
		n := kmdata.Node{
			Name:   idx.goName[gid],
			Path:   path,
			Parent: parent,
			Pos:    pos,
			PosZ:   uy.F(uy.Get(tf, "m_LocalPosition", "z")),
			RotZ: quatToZ(
				uy.F(uy.Get(tf, "m_LocalRotation", "z")),
				uy.F(uy.Get(tf, "m_LocalRotation", "w")),
			),
			Scale: [2]float64{
				uy.F(uy.Get(tf, "m_LocalScale", "x")),
				uy.F(uy.Get(tf, "m_LocalScale", "y")),
			},
			Inactive: !idx.goActive[gid],
		}
		n.SortGroup = idx.groupByGO[gid]
		if r := idx.rendByGO[gid]; r != nil {
			for _, mv := range uy.L(r["m_Materials"]) {
				guid := uy.S(uy.Get(uy.M(mv), "guid"))
				if n.Mat == "" {
					if name, ok := idx.matNames[guid]; ok {
						n.Mat = name
					}
				}
				if name, ok := idx.mappedMats[guid]; ok {
					n.Mapped = true
					n.Mat = name
				}
			}
			n.Sprite = resolveSprite(tables,
				uy.S(uy.Get(r, "m_Sprite", "guid")), uy.I(uy.Get(r, "m_Sprite", "fileID")))
			n.Order = int(uy.I(r["m_SortingOrder"]))
			n.Layer = int(uy.I(r["m_SortingLayer"]))
			n.Hidden = uy.I(r["m_Enabled"]) == 0
			n.FlipX = uy.I(r["m_FlipX"]) != 0
			n.FlipY = uy.I(r["m_FlipY"]) != 0
			n.DrawMode = int(uy.I(r["m_DrawMode"]))
			n.Size = [2]float64{uy.F(uy.Get(r, "m_Size", "x")), uy.F(uy.Get(r, "m_Size", "y"))}
			n.Color = [4]float64{
				uy.F(uy.Get(r, "m_Color", "r")), uy.F(uy.Get(r, "m_Color", "g")),
				uy.F(uy.Get(r, "m_Color", "b")), uy.F(uy.Get(r, "m_Color", "a")),
			}
			n.MaskIn = int(uy.I(r["m_MaskInteraction"]))
		}
		if mk := idx.maskByGO[gid]; mk != nil {
			n.Mask = true
			n.Sprite = resolveSprite(tables,
				uy.S(uy.Get(mk, "m_Sprite", "guid")), uy.I(uy.Get(mk, "m_Sprite", "fileID")))
			n.Hidden = uy.I(mk["m_Enabled"]) == 0
		}
		self := len(scene.Nodes)
		nodeIdx[gid] = self
		pathIdx[path] = self
		scene.Nodes = append(scene.Nodes, n)

		for _, cv := range uy.L(tf["m_Children"]) {
			cid := uy.I(uy.Get(uy.M(cv), "fileID"))
			ct := idx.tfByID[cid]
			if ct == nil {
				continue
			}
			childName := idx.goName[idx.tfOwner[cid]]
			childPath := childName
			if path != "" {
				childPath = path + "/" + childName
			}
			walk(ct, self, childPath)
		}
	}
	walk(rootTF, -1, "")
	for _, p := range spec.extraScenePaths {
		ensureSyntheticScenePath(scene, pathIdx, p)
	}
	writeJSON("scene.json", scene)
	fmt.Printf("scene: %d nodes\n", len(scene.Nodes))
	return paths, nodeIdx
}

func ensureSyntheticScenePath(scene *kmdata.Rig, pathIdx map[string]int, path string) int {
	if i, ok := pathIdx[path]; ok {
		return i
	}
	parentPath := ""
	name := path
	if cut := strings.LastIndex(path, "/"); cut >= 0 {
		parentPath = path[:cut]
		name = path[cut+1:]
	}
	parent := -1
	if parentPath != "" {
		parent = ensureSyntheticScenePath(scene, pathIdx, parentPath)
	}
	i := len(scene.Nodes)
	scene.Nodes = append(scene.Nodes, kmdata.Node{
		Name:   name,
		Path:   path,
		Parent: parent,
		Scale:  [2]float64{1, 1},
	})
	pathIdx[path] = i
	return i
}

func synthesizeSceneAnimationPaths() {
	var scene kmdata.Rig
	readGeneratedJSON("scene.json", &scene)
	var anims map[string]*kmdata.Anim
	readGeneratedJSON("anims.json", &anims)
	var ctrls map[string]kmdata.Controller
	readGeneratedJSON("controllers.json", &ctrls)
	var animators kmdata.Animators
	readGeneratedJSON("animators.json", &animators)

	pathIdx := map[string]int{}
	for i, n := range scene.Nodes {
		if _, ok := pathIdx[n.Path]; !ok {
			pathIdx[n.Path] = i
		}
	}

	added := 0
	for root, ctrlName := range animators {
		ctrl, ok := ctrls[ctrlName]
		if !ok {
			continue
		}
		for _, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := anims[st.Clip]
			if anim == nil {
				continue
			}
			for _, rel := range animCurvePaths(anim) {
				if rel == "" {
					continue
				}
				full := rel
				if root != "" {
					full = root + "/" + rel
				}
				if _, ok := pathIdx[full]; ok {
					continue
				}
				ensureSyntheticScenePath(&scene, pathIdx, full)
				added++
			}
		}
	}
	if added == 0 {
		return
	}
	writeJSON("scene.json", &scene)
	fmt.Printf("scene: synthesized %d animator model paths\n", added)
}

func animCurvePaths(a *kmdata.Anim) []string {
	seen := map[string]bool{}
	add := func(path string) { seen[path] = true }
	for p := range a.Pos {
		add(p)
	}
	for p := range a.Scale {
		add(p)
	}
	for p := range a.Euler {
		add(p)
	}
	for p := range a.Sprites {
		add(p)
	}
	for p := range a.Floats {
		add(p)
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func readGeneratedJSON(name string, out any) {
	b, err := os.ReadFile(filepath.Join(*outDir, name))
	must(err)
	must(json.Unmarshal(b, out))
}

// ---------- roles ----------

func exportRoles(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string) {
	// 找到包含全部 role 字段的 MonoBehaviour（即游戏主脚本）
	var script map[string]any
	for _, d := range dt.byID {
		if d.classID != 114 {
			continue
		}
		hits := 0
		for _, f := range spec.roleFields {
			if _, ok := d.content[f]; ok {
				hits++
			}
		}
		if hits == len(spec.roleFields) {
			script = d.content
			break
		}
	}
	if script == nil {
		log.Fatalf("game script with fields %v not found in prefab", spec.roleFields)
	}

	roles := kmdata.Roles{}
	for _, f := range spec.roleFields {
		fid := uy.I(uy.Get(uy.M(script[f]), "fileID"))
		ref := dt.byID[fid]
		if ref == nil {
			if p := spec.roleFallbacks[f]; p != "" {
				roles[f] = p
			} else {
				log.Printf("warn: role %s -> &%d not found", f, fid)
			}
			continue
		}
		gid := fid
		if ref.classID != 1 { // 组件引用（如 Animator）→ 其 GameObject
			gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
		}
		p, ok := paths[gid]
		if !ok {
			if p := spec.roleFallbacks[f]; p != "" {
				roles[f] = p
			} else {
				log.Printf("warn: role %s GameObject &%d not in scene tree", f, gid)
			}
			continue
		}
		roles[f] = p
	}
	writeJSON("roles.json", roles)
	for _, f := range spec.roleFields {
		fmt.Printf("role %-13s -> %q\n", f, roles[f])
	}
}

// ---------- extra（数组引用 / 字符串表 / 曲线 / 对象模板 / 音效序列） ----------

// goPathOf 把组件或 GameObject 引用解析为场景节点 path。
func goPathOf(dt *docTable, paths map[int64]string, fid int64) (string, bool) {
	ref := dt.byID[fid]
	if ref == nil {
		return "", false
	}
	gid := fid
	if ref.classID != 1 {
		gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
	}
	p, ok := paths[gid]
	return p, ok
}

func extractCurveRef(dt *docTable, idx *prefabIndex, field string, rv any) (kmdata.Curve, bool) {
	fid := uy.I(uy.Get(uy.M(rv), "fileID"))
	curveDoc := dt.byID[fid]
	if curveDoc == nil {
		log.Printf("warn: curve %s -> &%d missing", field, fid)
		return kmdata.Curve{}, false
	}
	if curveDoc.classID == 1 {
		// Some Heaven Studio prefabs serialize a BezierCurve3D field as the
		// owning GameObject. Resolve that to the MonoBehaviour carrying
		// keyPoints so runtime ports still use Unity-authored path data.
		for _, d := range dt.byID {
			if d.classID != 114 {
				continue
			}
			if uy.I(uy.Get(d.content, "m_GameObject", "fileID")) != fid {
				continue
			}
			if uy.L(d.content["keyPoints"]) != nil || uy.L(d.content["KeyPoints"]) != nil {
				curveDoc = d
				break
			}
		}
	}
	kps := uy.L(curveDoc.content["keyPoints"])
	if kps == nil {
		kps = uy.L(curveDoc.content["KeyPoints"])
	}
	if len(kps) == 0 {
		log.Printf("warn: curve %s -> &%d has no keyPoints", field, fid)
		return kmdata.Curve{}, false
	}
	curve := kmdata.Curve{Sampling: int(uy.I(curveDoc.content["sampling"]))}
	for _, kv := range kps {
		pid := uy.I(uy.Get(uy.M(kv), "fileID"))
		pd := dt.byID[pid]
		if pd == nil {
			continue
		}
		gid := uy.I(uy.Get(pd.content, "m_GameObject", "fileID"))
		var tfID int64
		for id, owner := range idx.tfOwner {
			if owner == gid {
				tfID = id
				break
			}
		}
		lhl := [3]float64{
			uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "x")),
			uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "y")),
			uy.F(uy.Get(pd.content, "leftHandleLocalPosition", "z")),
		}
		rhl := [3]float64{
			uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "x")),
			uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "y")),
			uy.F(uy.Get(pd.content, "rightHandleLocalPosition", "z")),
		}
		curve.Points = append(curve.Points, kmdata.CurvePoint{
			P:  idx.transformPoint3D(tfID, [3]float64{}),
			LH: idx.transformPoint3D(tfID, lhl),
			RH: idx.transformPoint3D(tfID, rhl),
		})
	}
	return curve, true
}

func exportComponentCurves(extra *kmdata.Extra, dt *docTable, idx *prefabIndex, key string, fields []string, arrayFields []string, content map[string]any) {
	for _, f := range fields {
		if curve, ok := extractCurveRef(dt, idx, key+"."+f, content[f]); ok {
			extra.Curves[key+"."+f] = curve
		}
	}
	for _, f := range arrayFields {
		for i, rv := range uy.L(content[f]) {
			ck := fmt.Sprintf("%s.%s%d", key, f, i)
			if curve, ok := extractCurveRef(dt, idx, ck, rv); ok {
				extra.Curves[ck] = curve
			}
		}
	}
}

func exportExtra(spec sceneSpec, dt *docTable, idx *prefabIndex, paths map[int64]string, nodeIdx map[int64]int, tables map[string]*spriteTable) {
	if len(spec.refArrayFields)+len(spec.strArrayFields)+len(spec.curveFields)+len(spec.objMarkers)+len(spec.components) == 0 && !spec.wantSequences {
		return
	}
	// 游戏主脚本（与 exportRoles 相同的定位方式）
	var script map[string]any
	for _, d := range dt.byID {
		if d.classID != 114 {
			continue
		}
		hits := 0
		for _, f := range spec.roleFields {
			if _, ok := d.content[f]; ok {
				hits++
			}
		}
		if hits == len(spec.roleFields) && len(spec.roleFields) > 0 {
			script = d.content
			break
		}
	}
	if script == nil {
		log.Fatal("game script not found for extra extraction")
	}

	extra := &kmdata.Extra{
		RefArrays:   map[string][]string{},
		Strings:     map[string][]string{},
		Curves:      map[string]kmdata.Curve{},
		ObjNums:     map[string]map[string]float64{},
		ObjStrs:     map[string]map[string]string{},
		Sequences:   map[string][]kmdata.SeqClip{},
		RefArrayIdx: map[string][]int{},
		ObjRefs:     map[string]map[string]string{},
		ObjSprites:  map[string]map[string][]string{},
	}

	// goIdxOf 把组件或 GameObject 引用解析为场景节点下标。
	goIdxOf := func(fid int64) (int, bool) {
		ref := dt.byID[fid]
		if ref == nil {
			return -1, false
		}
		gid := fid
		if ref.classID != 1 {
			gid = uy.I(uy.Get(ref.content, "m_GameObject", "fileID"))
		}
		i, ok := nodeIdx[gid]
		return i, ok
	}

	for _, f := range spec.refArrayFields {
		for _, rv := range uy.L(script[f]) {
			fid := uy.I(uy.Get(uy.M(rv), "fileID"))
			if mname, isMat := idx.matNames[uy.S(uy.Get(uy.M(rv), "guid"))]; isMat {
				extra.RefArrays[f] = append(extra.RefArrays[f], mname)
				extra.RefArrayIdx[f] = append(extra.RefArrayIdx[f], -1)
				continue
			}
			p, ok := goPathOf(dt, paths, fid)
			if !ok {
				log.Printf("warn: refArray %s -> &%d not in scene", f, fid)
			}
			extra.RefArrays[f] = append(extra.RefArrays[f], p)
			i, ok := goIdxOf(fid)
			if !ok {
				i = -1
			}
			extra.RefArrayIdx[f] = append(extra.RefArrayIdx[f], i)
		}
	}
	for _, f := range spec.strArrayFields {
		for _, sv := range uy.L(script[f]) {
			extra.Strings[f] = append(extra.Strings[f], uy.S(sv))
		}
	}

	for _, f := range spec.curveFields {
		if curve, ok := extractCurveRef(dt, idx, f, script[f]); ok {
			extra.Curves[f] = curve
		}
	}

	// 对象模板组件（按字段特征识别）
	if len(spec.objMarkers) > 0 {
		for _, d := range dt.byID {
			if d.classID != 114 {
				continue
			}
			all := true
			for _, k := range spec.objMarkers {
				if _, ok := d.content[k]; !ok {
					all = false
					break
				}
			}
			if !all {
				continue
			}
			gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
			p, ok := paths[gid]
			if !ok {
				continue
			}
			nums, strs := map[string]float64{}, map[string]string{}
			for k, v := range d.content {
				if strings.HasPrefix(k, "m_") {
					continue
				}
				switch tv := v.(type) {
				case int, int64, uint64, float64:
					nums[k] = uy.F(v)
				case string:
					strs[k] = tv
				}
			}
			extra.ObjNums[p] = nums
			extra.ObjStrs[p] = strs
			// 单引用字段（Transform/GameObject → 节点 path）
			for _, f := range spec.objRefFields {
				rv, ok := d.content[f]
				if !ok {
					continue
				}
				fid := uy.I(uy.Get(uy.M(rv), "fileID"))
				rp, ok := goPathOf(dt, paths, fid)
				if !ok {
					log.Printf("warn: objRef %s.%s -> &%d not in scene", p, f, fid)
					continue
				}
				if extra.ObjRefs[p] == nil {
					extra.ObjRefs[p] = map[string]string{}
				}
				extra.ObjRefs[p][f] = rp
			}
			// sprite 引用数组字段（→ 图集切片名）
			for _, f := range spec.objSpriteFields {
				rv, ok := d.content[f]
				if !ok {
					continue
				}
				var names []string
				for _, sv := range uy.L(rv) {
					s := uy.M(sv)
					name := resolveSprite(tables, uy.S(s["guid"]), uy.I(s["fileID"]))
					if name == "" {
						log.Printf("warn: objSprite %s.%s 切片解析失败 guid=%s fileID=%d",
							p, f, uy.S(s["guid"]), uy.I(s["fileID"]))
					}
					names = append(names, name)
				}
				if extra.ObjSprites[p] == nil {
					extra.ObjSprites[p] = map[string][]string{}
				}
				extra.ObjSprites[p][f] = names
			}
		}
	}

	// 通用组件 dump
	if len(spec.components) > 0 {
		extra.Components = map[string]kmdata.Component{}
		for _, cs := range spec.components {
			type hit struct {
				p       string
				content map[string]any
			}
			var hits []hit
			for _, d := range dt.byID {
				if d.classID != 114 {
					continue
				}
				ok := true
				for _, mk := range cs.markers {
					if _, has := d.content[mk]; !has {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
				p, inScene := paths[gid]
				if !inScene {
					continue
				}
				if cs.atPath != "" && p != cs.atPath {
					continue
				}
				hits = append(hits, hit{p, d.content})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].p < hits[j].p })
			switch {
			case len(hits) == 0:
				log.Fatalf("组件 %s（markers %v）未在 prefab 中找到", cs.name, cs.markers)
			case cs.multi:
				for i, h := range hits {
					key := fmt.Sprintf("%s%d", cs.name, i)
					extra.Components[key] = dumpComponent(dt, paths, tables, idx.matNames, h.p, h.content)
					exportComponentCurves(extra, dt, idx, key, cs.curveFields, cs.curveArrayFields, h.content)
				}
			default:
				if len(hits) > 1 {
					log.Printf("warn: 组件 %s 匹配 %d 个，保留 path 最小者 %q（用 atPath/multi 限定）", cs.name, len(hits), hits[0].p)
				}
				extra.Components[cs.name] = dumpComponent(dt, paths, tables, idx.matNames, hits[0].p, hits[0].content)
				exportComponentCurves(extra, dt, idx, cs.name, cs.curveFields, cs.curveArrayFields, hits[0].content)
			}
		}
	}

	if spec.wantSequences {
		for _, d := range dt.byID {
			if d.classID != 114 {
				continue
			}
			seqs := uy.L(d.content["SoundSequences"])
			if seqs == nil {
				continue
			}
			for _, sv := range seqs {
				s := uy.M(sv)
				name := uy.S(s["name"])
				for _, cv := range uy.L(uy.Get(s, "sequence", "clips")) {
					c := uy.M(cv)
					clip := uy.S(c["clip"])
					if i := strings.LastIndexByte(clip, '/'); i >= 0 {
						clip = clip[i+1:]
					}
					vol := uy.F(c["volume"])
					if vol == 0 {
						vol = 1
					}
					extra.Sequences[name] = append(extra.Sequences[name], kmdata.SeqClip{
						Clip: clip, Beat: uy.F(c["beat"]), Volume: vol,
					})
				}
			}
		}
	}

	writeJSON("extra.json", extra)
	fmt.Printf("extra: %d refArrays, %d curves, %d obj templates, %d sequences\n",
		len(extra.RefArrays), len(extra.Curves), len(extra.ObjNums), len(extra.Sequences))
}

// dumpComponent 通用 dump 一个 MonoBehaviour 的全部序列化字段：
// 数值/字符串直存；{fileID} 引用 → 节点 path；{fileID, guid} → 图集切片名
// （解析失败回退节点 path）；x/y/z 向量按分量展开；结构体数组逐项解析。
func dumpComponent(dt *docTable, paths map[int64]string, tables map[string]*spriteTable,
	mats map[string]string, p string, content map[string]any) kmdata.Component {
	c := kmdata.Component{
		Path: p,
		Nums: map[string]float64{}, Strs: map[string]string{},
		Refs: map[string]string{}, Sprites: map[string]string{},
		RefArrays: map[string][]string{}, SpriteArrays: map[string][]string{},
		Lists: map[string][]kmdata.ComponentItem{},
	}
	resolveRef := func(field string, m map[string]any) (string, bool) {
		fid := uy.I(m["fileID"])
		if fid == 0 {
			return "", false
		}
		if g := uy.S(m["guid"]); g != "" {
			if name := resolveSprite(tables, g, fid); name != "" {
				return name, true // sprite
			}
			if name, ok := mats[g]; ok {
				return name, false // 映射材质 → 文件主名
			}
		}
		rp, ok := goPathOf(dt, paths, fid)
		if !ok {
			log.Printf("warn: 组件字段 %s.%s 引用 &%d 无法解析", p, field, fid)
			return "", false
		}
		return rp, false
	}
	for k, v := range content {
		if strings.HasPrefix(k, "m_") || k == "SoundSequences" {
			continue
		}
		switch tv := v.(type) {
		case int, int64, uint64, float64:
			c.Nums[k] = uy.F(v)
		case string:
			c.Strs[k] = tv
		case map[string]any:
			if _, hasID := tv["fileID"]; hasID {
				val, isSprite := resolveRef(k, tv)
				if val == "" {
					continue
				}
				if isSprite {
					c.Sprites[k] = val
				} else {
					c.Refs[k] = val
				}
			} else if _, hasX := tv["x"]; hasX {
				for _, axis := range []string{"x", "y", "z", "w"} {
					if av, ok := tv[axis]; ok {
						c.Nums[k+"."+axis] = uy.F(av)
					}
				}
			} else if _, hasR := tv["r"]; hasR {
				for _, axis := range []string{"r", "g", "b", "a"} {
					if av, ok := tv[axis]; ok {
						c.Nums[k+"."+axis] = uy.F(av)
					}
				}
			} else if _, hasKey0 := tv["key0"]; hasKey0 {
				// Unity Gradient：key0..7 颜色 + ctime0..7（0..65535 归一化时刻）
				nkeys := int(uy.F(tv["m_NumColorKeys"]))
				for ki := 0; ki < nkeys && ki < 8; ki++ {
					kv := uy.M(tv[fmt.Sprintf("key%d", ki)])
					item := kmdata.ComponentItem{Nums: map[string]float64{
						"r": uy.F(kv["r"]), "g": uy.F(kv["g"]), "b": uy.F(kv["b"]), "a": uy.F(kv["a"]),
						"t": uy.F(tv[fmt.Sprintf("ctime%d", ki)]) / 65535,
					}}
					c.Lists[k] = append(c.Lists[k], item)
				}
			}
		case []any:
			for _, iv := range tv {
				im := uy.M(iv)
				if im == nil {
					continue
				}
				if _, hasID := im["fileID"]; hasID && len(im) <= 3 { // 纯引用数组（fileID[+guid+type]）
					val, isSprite := resolveRef(k, im)
					if isSprite {
						c.SpriteArrays[k] = append(c.SpriteArrays[k], val)
					} else {
						c.RefArrays[k] = append(c.RefArrays[k], val)
					}
					continue
				}
				c.Lists[k] = append(c.Lists[k], dumpItem(k, im, resolveRef, true))
			}
		}
	}
	return c
}

// dumpItem 解析结构体数组的一项；nest=true 时再下钻一层嵌套结构数组。
// SuperCurveObject.Path 会形成 paths[].positions[].values[]，因此第二层
// 的纯结构数组也保留，但不继续递归，避免丢掉每个轨迹点上的 rot 等曲线值。
func dumpItem(field string, im map[string]any,
	resolveRef func(string, map[string]any) (string, bool), nest bool) kmdata.ComponentItem {
	item := kmdata.ComponentItem{
		Nums: map[string]float64{}, Strs: map[string]string{}, Refs: map[string]string{},
	}
	for ik, ivv := range im {
		switch itv := ivv.(type) {
		case int, int64, uint64, float64:
			item.Nums[ik] = uy.F(ivv)
		case string:
			item.Strs[ik] = itv
		case map[string]any:
			if _, hasID := itv["fileID"]; hasID {
				if val, isSprite := resolveRef(field+"."+ik, itv); val != "" && !isSprite {
					item.Refs[ik] = val
				}
			} else if _, hasX := itv["x"]; hasX {
				for _, axis := range []string{"x", "y", "z", "w"} {
					if av, ok := itv[axis]; ok {
						item.Nums[ik+"."+axis] = uy.F(av)
					}
				}
			} else if _, hasR := itv["r"]; hasR {
				for _, axis := range []string{"r", "g", "b", "a"} {
					if av, ok := itv[axis]; ok {
						item.Nums[ik+"."+axis] = uy.F(av)
					}
				}
			}
		case []any:
			if !nest && !scalarStructArray(itv) {
				continue
			}
			for _, nv := range itv {
				nm := uy.M(nv)
				if nm == nil {
					continue
				}
				if item.Items == nil {
					item.Items = map[string][]kmdata.ComponentItem{}
				}
				item.Items[ik] = append(item.Items[ik], dumpItem(field+"."+ik, nm, resolveRef, false))
			}
		}
	}
	return item
}

func scalarStructArray(items []any) bool {
	if len(items) == 0 {
		return false
	}
	for _, v := range items {
		m := uy.M(v)
		if m == nil {
			return false
		}
		for _, vv := range m {
			switch tv := vv.(type) {
			case []any:
				return false
			case map[string]any:
				if _, hasID := tv["fileID"]; hasID {
					continue
				}
				if _, hasX := tv["x"]; hasX {
					continue
				}
				if _, hasR := tv["r"]; hasR {
					continue
				}
				return false
			}
		}
	}
	return true
}

// ---------- anims / sounds ----------

// exportAnimDir 导出全部剪辑。同名 .anim 可能分属不同 Animator（如
// Arisa/FacePoser/MouthA 与 BackDancers/FacePoser/MouthA），因此每个剪辑
// 都以动画根相对路径为命名空间 key；文件名全局唯一时再额外写裸名 key
// （向后兼容只有单 Animator 的游戏）。
type importedClip struct {
	Key      string
	Duration float64
	Loop     bool
}

func scanImportedClips(root string, fps float64) map[string]map[int64]importedClip {
	if fps <= 0 {
		fps = 60
	}
	out := map[string]map[int64]importedClip{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || (!strings.HasSuffix(p, ".fbx.meta") && !strings.HasSuffix(p, ".dae.meta")) {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m, err := uy.ParseSingle(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		guid := uy.S(m["guid"])
		if guid == "" {
			return nil
		}
		fbxPath := strings.TrimSuffix(p, ".meta")
		rel, err := filepath.Rel(root, fbxPath)
		if err != nil {
			rel = filepath.Base(fbxPath)
		}
		rel = strings.TrimSuffix(strings.TrimSuffix(filepath.ToSlash(rel), ".fbx"), ".dae")
		for _, cv := range uy.L(uy.Get(m, "ModelImporter", "animations", "clipAnimations")) {
			c := uy.M(cv)
			id := uy.I(c["internalID"])
			if id == 0 {
				continue
			}
			name := uy.S(c["name"])
			if name == "" {
				name = fmt.Sprintf("clip_%d", id)
			}
			dur := (uy.F(c["lastFrame"]) - uy.F(c["firstFrame"])) / fps
			if dur < 0 {
				dur = 0
			}
			if out[guid] == nil {
				out[guid] = map[int64]importedClip{}
			}
			out[guid][id] = importedClip{
				Key:      rel + "/" + name,
				Duration: dur,
				Loop:     uy.I(c["loopTime"]) != 0,
			}
		}
		// Some imported model animations (Rhythm Rally's Paddler.fbx) expose no
		// clipAnimations entries, but Unity still records the AnimationClip
		// fileIDs in internalIDToNameTable. Preserve those names so controller
		// states can bind to their imported motions instead of becoming blank.
		for _, tv := range uy.L(uy.Get(m, "ModelImporter", "internalIDToNameTable")) {
			t := uy.M(tv)
			id := uy.I(mapValueByStringKey(t["first"], "74"))
			if id == 0 {
				continue
			}
			if out[guid] != nil {
				if _, exists := out[guid][id]; exists {
					continue
				}
			}
			if out[guid] == nil {
				out[guid] = map[int64]importedClip{}
			}
			out[guid][id] = importedClip{
				Key: rel + "/" + importedClipTableName(uy.S(t["second"]), id),
			}
		}
		return nil
	})
	return out
}

func mapValueByStringKey(m any, key string) any {
	if mm := uy.M(m); mm != nil {
		return mm[key]
	}
	rv := reflect.ValueOf(m)
	if !rv.IsValid() || rv.Kind() != reflect.Map {
		return nil
	}
	for _, k := range rv.MapKeys() {
		if fmt.Sprint(k.Interface()) == key {
			return rv.MapIndex(k).Interface()
		}
	}
	return nil
}

func importedClipTableName(raw string, id int64) string {
	name := strings.TrimSpace(raw)
	if i := strings.LastIndex(name, "|"); i >= 0 && i+1 < len(name) {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("clip_%d", id)
	}
	return name
}

func exportAnimDir(spec sceneSpec, tables map[string]*spriteTable) {
	dir := spec.animRoot()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		writeJSON("anims.json", map[string]*kmdata.Anim{})
		fmt.Printf("anims: 0 clip files, 0 imported clips (missing %s)\n", dir)
		return
	}
	type clipFile struct {
		base, nsKey string
		clip        *kmdata.Anim
	}
	var clips []clipFile
	baseCount := map[string]int{}
	must(filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".anim") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		docs, err := uy.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		for i := range docs {
			if docs[i].ClassID == 74 {
				base := strings.TrimSuffix(filepath.Base(p), ".anim")
				ns := animNSKey(dir, p)
				clips = append(clips, clipFile{base, ns, convertClip(docs[i].Content(), tables)})
				baseCount[base]++
				break
			}
		}
		return nil
	}))
	anims := map[string]*kmdata.Anim{}
	for _, c := range clips {
		anims[c.nsKey] = c.clip
		if baseCount[c.base] == 1 {
			anims[c.base] = c.clip
		} else {
			fmt.Printf("anim %q 有 %d 个同名文件，仅按命名空间 key 导出（如 %q）\n", c.base, baseCount[c.base], c.nsKey)
		}
	}
	imported := scanImportedClips(spec.gameRoot(), spec.importedAnimFPS)
	importedCount := 0
	for _, byID := range imported {
		for _, c := range byID {
			anims[c.Key] = &kmdata.Anim{Duration: c.Duration, Loop: c.Loop}
			importedCount++
		}
	}
	if *game == "bossaNova" {
		fixBossaNovaAnimPaths(anims)
	}
	writeJSON("anims.json", anims)
	fmt.Printf("anims: %d clip files, %d imported clips\n", len(clips), importedCount)
}

func fixBossaNovaAnimPaths(anims map[string]*kmdata.Anim) {
	for key, a := range anims {
		if a == nil || !(strings.HasPrefix(key, "Nova/") || strings.HasPrefix(key, "Nova")) {
			continue
		}
		moveXYCurve(a.Pos, "Head/Sprout", "Head/Flower")
		moveXYCurve(a.Scale, "Head/Sprout", "Head/Flower")
		moveKeys(a.Euler, "Head/Sprout", "Head/Flower")
		moveSwaps(a.Sprites, "Head/Sprout", "Head/Flower")
		moveFloatAttrs(a.Floats, "Head/Sprout", "Head/Flower")
	}
}

func moveXYCurve(m map[string]kmdata.XYCurve, src, dst string) {
	v, ok := m[src]
	if !ok {
		return
	}
	if old, exists := m[dst]; exists {
		if len(old.X) == 0 {
			old.X = v.X
		}
		if len(old.Y) == 0 {
			old.Y = v.Y
		}
		m[dst] = old
	} else {
		m[dst] = v
	}
	delete(m, src)
}

func moveKeys(m map[string][]kmdata.Key, src, dst string) {
	v, ok := m[src]
	if !ok {
		return
	}
	if len(m[dst]) == 0 {
		m[dst] = v
	}
	delete(m, src)
}

func moveSwaps(m map[string][]kmdata.SwapKey, src, dst string) {
	v, ok := m[src]
	if !ok {
		return
	}
	if len(m[dst]) == 0 {
		m[dst] = v
	}
	delete(m, src)
}

func moveFloatAttrs(m map[string]map[string][]kmdata.Key, src, dst string) {
	v, ok := m[src]
	if !ok {
		return
	}
	if m[dst] == nil {
		m[dst] = map[string][]kmdata.Key{}
	}
	for attr, keys := range v {
		if len(m[dst][attr]) == 0 {
			m[dst][attr] = keys
		}
	}
	delete(m, src)
}

func copySounds(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("sounds: 0 copied (missing %s)\n", dir)
		return
	}
	n := 0
	must(filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(d.Name(), ".meta") {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".ogg" && ext != ".wav" && ext != ".flac" {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if ext == ".flac" {
			rel = strings.TrimSuffix(rel, filepath.Ext(rel)) + ".wav"
		}
		// 子目录音效（cheerReaders 的 Solo/Girls/All）保留相对路径作 key
		dst := filepath.Join(*outDir, "sounds", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		var b []byte
		if ext == ".flac" {
			// Ebitengine has built-in decoders for wav/ogg in this project. Keep
			// extraction reproducible by converting Unity FLAC clips into wav at
			// asset build time instead of silently dropping reaction sounds.
			if _, err := exec.LookPath("ffmpeg"); err != nil {
				return fmt.Errorf("copy %s: ffmpeg required to convert flac: %w", p, err)
			}
			if err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", p, dst).Run(); err != nil {
				return err
			}
			b, err = os.ReadFile(dst)
			must(err)
		} else {
			b, err = os.ReadFile(p)
			must(err)
			must(os.WriteFile(dst, b, 0o644))
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 2 && parts[0] == "en" {
			// Heaven Studio 的 localized SoundByte 会用裸 clip 名引用当前语言音效。
			// 保留 en/foo.ogg 给审计看源目录，同时写 foo.ogg 作为运行时默认英文 key。
			alias := filepath.Join(*outDir, "sounds", parts[1])
			if _, err := os.Stat(alias); os.IsNotExist(err) {
				must(os.WriteFile(alias, b, 0o644))
			}
		}
		n++
		return nil
	}))
	fmt.Printf("sounds: %d copied\n", n)
}
