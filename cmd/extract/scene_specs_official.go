package main

// officialBaseSceneSpecs gives every bundled Heaven Studio prefab a basic
// extraction entry. These entries are intentionally asset-only: gameplay code
// still has to prove event timing, sounds, effects, and inputs against the C#
// source before a game is registered as playable.
var officialBaseSceneSpecs = map[string]sceneSpec{
	"agbSamuraiSlice": basicOfficialSceneSpec("SamuraiSliceAgb", "agbSamuraiSlice.prefab"),
	"airboarder":      basicOfficialSceneSpec("Airboarder", "airboarder.prefab"),
	"animalAcrobat":   basicOfficialSceneSpec("AnimalAcrobat", "animalAcrobat.prefab"),
	"balloonHunter":   basicOfficialSceneSpec("BalloonHunter", "balloonHunter.prefab"),
	"basketballGirls": basicOfficialSceneSpec("BasketballGirls", "basketballGirls.prefab"),
	"bigRockFinish":   basicOfficialSceneSpec("BigRockFinish", "bigRockFinish.prefab"),
	"blueBirds":       basicOfficialSceneSpec("BlueBirds", "blueBirds.prefab"),
	"boardMeeting":    basicOfficialSceneSpec("BoardMeeting", "boardMeeting.prefab"),
	"bonOdori":        basicOfficialSceneSpec("BonOdori", "bonOdori.prefab"),
	"bossaNova":       basicOfficialSceneSpec("BossaNova", "bossaNova.prefab"),
	"bouncyRoad":      basicOfficialSceneSpec("BouncyRoad", "bouncyRoad.prefab"),
	"builtToScaleDS":  basicOfficialSceneSpec("BuiltToScaleDS", "builtToScaleDS.prefab"),
	"builtToScaleRvl": basicOfficialSceneSpec("BuiltToScaleRvl", "builtToScaleRvl.prefab"),
	"cannery":         basicOfficialSceneSpec("Cannery", "cannery.prefab"),
	"catchOfTheDay": {
		dir:    "CatchOfTheDay",
		prefab: "catchOfTheDay.prefab",
		roleFields: []string{
			"Angler", "LakeScenePrefab", "LakeSceneHolder", "AnglerTransform",
		},
		wantControllers: true,
		commonSounds: []string{
			"count-ins/and.wav",
			"count-ins/go1.wav",
			"count-ins/go2.wav",
			"count-ins/one1.wav",
			"count-ins/two1.wav",
			"count-ins/three1.wav",
			"miss.wav",
		},
		templatePrefabs: []string{
			"Prefabs/LakeScene.prefab",
			"Prefabs/SchoolFish.prefab",
			"Prefabs/Bubble.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"Angler", "LakeScenePrefab", "LakeSceneHolder", "_TopColors", "_BottomColors",
				"AnglerTransform", "_StickyCanvas",
			}},
			{name: "lake", markers: []string{
				"FishAnimator", "BGAnimator", "GradientBG", "TopBG", "BottomBG",
				"BGFishes", "BigManta", "SmallManta", "FishSchool", "SchoolFishes",
				"Bubbles", "Renderer", "CrossfadeAnimator", "RenderCamera", "DisplayMesh",
				"SchoolFishPrefab",
			}, atPath: "Main/LakeScene"},
			{name: "bgfish", markers: []string{"_Animator", "_Sprite", "FleeAnim", "FlipSprite"}, multi: true},
		},
	},
	"catchyTune":      basicOfficialSceneSpec("CatchyTune", "catchyTune.prefab"),
	"chameleon":       basicOfficialSceneSpec("Chameleon", "chameleon.prefab"),
	"chargingChicken": basicOfficialSceneSpec("ChargingChicken", "chargingChicken.prefab"),
	"clapTrap":        basicOfficialSceneSpec("ClapTrap", "clapTrap.prefab"),
	"clappyTrio":      basicOfficialSceneSpec("ClappyTrio", "clappyTrio.prefab"),
	"coinToss":        basicOfficialSceneSpec("CoinToss", "coinToss.prefab"),
	"cropStomp":       basicOfficialSceneSpec("CropStomp", "cropStomp.prefab"),
	"djSchool":        basicOfficialSceneSpec("DJSchool", "djSchool.prefab"),
	"dogNinja":        basicOfficialSceneSpec("DogNinja", "dogNinja.prefab"),
	"doubleDate": {
		dir:    "DoubleDate",
		prefab: "doubleDate.prefab",
		roleFields: []string{
			"boyAnim", "girlAnim", "weasels", "treeAnim", "clouds",
			"girlObj", "girlWeaselObj", "girlWeaselShockObj",
			"bgGO", "bushGO",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefab/SoccerBall.prefab",
			"Prefab/BasketBall.prefab",
			"Prefab/Football.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"soccer", "basket", "football", "dropShadow", "leaves",
				"boyAnim", "girlAnim", "weasels", "treeAnim", "clouds",
				"girlObj", "girlWeaselObj", "girlWeaselShockObj",
				"doubleDateCellAnim", "bgSquare", "bgGradient", "bgGO", "bushGO",
				"bgIntro", "bgLong", "cloudSpeed", "cloudDistance",
				"floorHeight", "shadowDepthScaleMin", "shadowDepthScaleMax",
				"ballBouncePaths",
			}},
		},
	},
	"dressYourBest":    basicOfficialSceneSpec("DressYourBest", "dressYourBest.prefab"),
	"drummerDuel":      basicOfficialSceneSpec("DrummerDuel", "drummerDuel.prefab"),
	"drummingPractice": basicOfficialSceneSpec("DrummingPractice", "drummingPractice.prefab"),
	"fallingWaffle":    basicOfficialSceneSpec("FallingWaffle", "fallingWaffle.prefab"),
	"fanClub": {
		dir:    "FanClub",
		prefab: "fanClub.prefab",
		roleFields: []string{
			"StageAnimator", "Arisa", "ArisaRootMotion", "ArisaShadow",
			"spectator", "spectatorAnchor", "Blue", "Orange", "spectatorMat",
		},
		roleFallbacks: map[string]string{
			"StageAnimator":   "Background",
			"Arisa":           "Idol_rootMotion/Idol",
			"ArisaRootMotion": "Idol_rootMotion",
			"ArisaShadow":     "idol_Shadow",
			"spectator":       "Fan",
			"spectatorAnchor": "fan_SpawnAnchor",
			"Blue":            "dancerR_rootMotion/Blue",
			"Orange":          "dancerL_rootMotion/Orange",
		},
		wantControllers: true,
		templatePrefabs: []string{
			"Prefabs/Fan.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"StageAnimator", "Arisa", "ArisaRootMotion", "ArisaShadow",
				"spectator", "spectatorAnchor", "Blue", "Orange", "spectatorMat",
			}},
			{name: "arisa", markers: []string{
				"idolClapEffect", "idolWinkEffect", "idolKissEffect",
				"idolWinkArrEffect", "baseHead", "facePoser", "coreMat",
			}, atPath: "Idol_rootMotion/Idol"},
			{name: "amie", markers: []string{
				"stepDistance", "startPostion", "rootYPos", "clapEffect",
				"winkEffect", "rootTransform", "shadow", "baseHead",
				"facePoser", "coreMat",
			}, multi: true},
			{name: "fan", markers: []string{
				"motionRoot", "headRoot", "sortingGroup", "animator",
				"fanClapEffect", "shadow",
			}, atPath: "Fan"},
		},
	},
	"figureFighter": basicOfficialSceneSpec("FigureFighter", "figureFighter.prefab"),
	"fillbots":      basicOfficialSceneSpec("Fillbots", "fillbots.prefab"),
	"fireworks":     basicOfficialSceneSpec("Fireworks", "fireworks.prefab"),
	"firstContact":  basicOfficialSceneSpec("FirstContact", "firstContact.prefab"),
	"flipperFlop":   basicOfficialSceneSpec("FlipperFlop", "flipperFlop.prefab"),
	"forkLifter":    basicOfficialSceneSpec("ForkLifter", "forkLifter.prefab"),
	"freezeFrame":   basicOfficialSceneSpec("FreezeFrame", "freezeFrame.prefab"),
	"frogHop":       basicOfficialSceneSpec("FrogHop", "frogHop.prefab"),
	"frogPrincess":  basicOfficialSceneSpec("FrogPrincess", "frogPrincess.prefab"),
	"fruitBasket": {
		dir:    "FruitBasket",
		prefab: "fruitBasket.prefab",
		roleFields: []string{
			"fruitHolder", "applePrefab", "lemonPrefab", "melonPrefab",
			"courtneyAnimator", "hoopL", "hoopR", "courtneySprite",
			"courtneyExtendedSprite", "courtneyHoleSprite", "thoughtBubblePrefab",
			"catsAnimator",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		components: []componentSpec{
			{name: "game", markers: []string{
				"fruitHolder", "applePrefab", "lemonPrefab", "melonPrefab",
				"courtneyAnimator", "hoopL", "hoopR", "courtneySprite",
				"courtneyExtendedSprite", "courtneyHoleSprite", "thoughtBubblePrefab",
				"catsAnimator", "fruitPaths",
			}},
			{name: "apple", markers: []string{"sprite", "leftPipe", "rightPipe"}, atPath: "Apple"},
			{name: "lemon", markers: []string{"sprite", "leftPipe", "rightPipe"}, atPath: "Lemon"},
			{name: "melon", markers: []string{"sprite", "leftPipe", "rightPipe", "leftPipeAnim", "rightPipeAnim"}, atPath: "Melon"},
			{name: "daydream", markers: []string{
				"bubbleAnimator", "daydreamContainer", "daydreamAnimator", "bubbleLRenderer",
			}, atPath: "ThoughtBubble"},
		},
	},
	"gardenDance":      basicOfficialSceneSpec("GardenDance", "gardenDance.prefab"),
	"gleeClub":         basicOfficialSceneSpec("GleeClub", "gleeClub.prefab"),
	"holeInOne":        basicOfficialSceneSpec("HoleInOne", "holeInOne.prefab"),
	"launchParty":      basicOfficialSceneSpec("LaunchParty", "launchParty.prefab"),
	"loveLab":          basicOfficialSceneSpec("LoveLab", "loveLab.prefab"),
	"loveLizards":      basicOfficialSceneSpec("LoveLizards", "loveLizards.prefab"),
	"loveRap":          basicOfficialSceneSpec("LoveRap", "loveRap.prefab"),
	"lumbearjack":      basicOfficialSceneSpec("Lumbearjack", "lumbearjack.prefab"),
	"magicGirl":        basicOfficialSceneSpec("MagicGirl", "magicGirl.prefab"),
	"mannequinFactory": basicOfficialSceneSpec("MannequinFactory", "mannequinFactory.prefab"),
	"manzai":           basicOfficialSceneSpec("Manzai", "manzai.prefab"),
	"moaiDooWop":       basicOfficialSceneSpec("MoaiDooWop", "moaiDooWop.prefab"),
	"monkeyWatch":      basicOfficialSceneSpec("MonkeyWatch", "monkeyWatch.prefab"),
	"mrUpbeat":         basicOfficialSceneSpec("MrUpbeat", "mrUpbeat.prefab"),
	"nailCarpenter":    basicOfficialSceneSpec("NailCarpenter", "nailCarpenter.prefab"),
	"nightWalkAgb":     basicOfficialSceneSpec("NightWalkAgb", "nightWalkAgb.prefab"),
	"ninjaBodyguard":   basicOfficialSceneSpec("NinjaBodyguard", "ninjaBodyguard.prefab"),
	"nipInTheBud":      basicOfficialSceneSpec("NipInTheBud", "nipInTheBud.prefab"),
	"octopusMachine":   basicOfficialSceneSpec("OctopusMachine", "octopusMachine.prefab"),
	"packingPests":     basicOfficialSceneSpec("PackingPests", "packingPests.prefab"),
	"pajamaParty": {
		dir:    "PajamaParty",
		prefab: "pajamaParty.prefab",
		roleFields: []string{
			"Mako", "Bed", "MonkeyPrefab", "Castle", "BgAnimator", "BalloonsEffect",
			"SpawnRoot",
		},
		roleFallbacks: map[string]string{
			"Mako":       "Mako_Root",
			"Bed":        "Bed",
			"BgAnimator": "Bg",
			"SpawnRoot":  "Spawn_Root",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefab/Monkey.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"Mako", "Bed", "MonkeyPrefab", "Castle", "BgAnimator",
				"BalloonsEffect", "SpawnRoot", "HighCameraHeight",
				"monkeyNrmColour", "monkeyHighColour", "monkeyColMat",
			}},
		},
	},
	"powerCalligraphy": basicOfficialSceneSpec("PowerCalligraphy", "powerCalligraphy.prefab"),
	"quizShow":         basicOfficialSceneSpec("QuizShow", "quizShow.prefab"),
	"rapMen":           basicOfficialSceneSpec("RapMen", "rapMen.prefab"),
	"rhythmFighter":    basicOfficialSceneSpec("RhythmFighter", "rhythmFighter.prefab"),
	"rhythmRally":      basicOfficialSceneSpec("RhythmRally", "rhythmRally.prefab"),
	"rhythmSheriff":    basicOfficialSceneSpec("RhythmSheriff", "rhythmSheriff.prefab"),
	"rhythmTestGBA":    basicOfficialSceneSpec("RhythmTestGBA", "rhythmTestGBA.prefab"),
	"rhythmTweezers":   basicOfficialSceneSpec("RhythmTweezers", "rhythmTweezers.prefab"),
	"ringside":         basicOfficialSceneSpec("Ringside", "ringside.prefab"),
	"rockers":          basicOfficialSceneSpec("Rockers", "rockers.prefab"),
	"samuraiSliceNtr":  basicOfficialSceneSpec("SamuraiSliceNtr", "samuraiSliceNtr.prefab"),
	"samuraiSliceRvl": {
		dir:    "SamuraiSliceRvl",
		prefab: "samuraiSliceRvl.prefab",
		roleFields: []string{
			"SamuraiAnim", "fgHolder", "demonholder", "flashholder",
			"hordeSlicedPrefab", "smogEffectPrefab", "hordeDemonPrefab",
		},
		curveFields:     []string{"spawnCurve", "missCurve", "walkCurve"},
		refArrayFields:  []string{"slicedDemonPrefabs"},
		wantControllers: true,
		templatePrefabs: []string{
			"Prefabs/SmallDemonSlicedPrefab.prefab",
			"Prefabs/MediumDemonSliced.prefab",
			"Prefabs/BigDemonSliced.prefab",
			"Prefabs/HugeDemonSliced.prefab",
			"Prefabs/SlicedHorde.prefab",
			"Prefabs/HordePrefab.prefab",
			"Prefabs/Smog.prefab",
			"Prefabs/SmogParticlePrefab.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"SamuraiAnim", "fgHolder", "demonholder", "flashholder",
				"slicedDemonPrefabs", "hordeSlicedPrefab", "smogEffectPrefab",
				"hordeDemonPrefab", "spawnCurve", "missCurve", "walkCurve",
				"hordeSpawnPositions",
			}, curveFields: []string{"spawnCurve", "missCurve", "walkCurve"}},
			{name: "demon", markers: []string{
				"SDemonAnim", "MDemonAnim", "LDemonAnim", "XLDemonAnim",
				"Sdemon", "Mdemon", "Ldemon", "XLdemon",
			}, atPath: "DemonHolder"},
			{name: "flash", markers: []string{"LightningAnim", "FlashAnim", "Lightning", "FlashEffect"}, atPath: "LightningHolder"},
			{name: "horde", markers: []string{
				"anim", "sr", "minRushDuration", "maxRushDuration",
				"targetPosition", "gravity", "fallVelocity", "rotationSpeed",
			}, multi: true},
			{name: "sliced", markers: []string{
				"topPart", "botPart", "waitTime", "rotationSpeed",
				"topVelocity", "botVelocity", "gravity",
			}, multi: true},
			{name: "smog", markers: []string{
				"particlePrefab", "pixelScale", "baseOffset",
				"defaultStartPos", "particleZPos", "manualOffset",
			}},
		},
	},
	"shootEmUp":         basicOfficialSceneSpec("ShootEmUp", "shootEmUp.prefab"),
	"showtime":          basicOfficialSceneSpec("Showtime", "showtime.prefab"),
	"sickBeats":         basicOfficialSceneSpec("SickBeats", "sickBeats.prefab"),
	"slotMonster":       basicOfficialSceneSpec("SlotMonster", "slotMonster.prefab"),
	"spaceSoccer":       basicOfficialSceneSpec("SpaceSoccer", "spaceSoccer.prefab"),
	"spaceball":         basicOfficialSceneSpec("SpaceBall", "spaceball.prefab"),
	"splashdown":        basicOfficialSceneSpec("Splashdown", "splashdown.prefab"),
	"sumoBrothers":      basicOfficialSceneSpec("SumoBrothers", "sumoBrothers.prefab"),
	"superSamuraiSlice": basicOfficialSceneSpec("SuperSamuraiSlice", "superSamuraiSlice.prefab"),
	"tapTroupe":         basicOfficialSceneSpec("TapTroupe", "tapTroupe.prefab"),
	"theDazzles":        basicOfficialSceneSpec("TheDazzles", "theDazzles.prefab"),
	"tossBoys":          basicOfficialSceneSpec("TossBoys", "tossBoys.prefab"),
	"tramAndPauline":    basicOfficialSceneSpec("TramAndPauline", "tramAndPauline.prefab"),
	"tunnel":            basicOfficialSceneSpec("Tunnel", "tunnel.prefab"),
	"valiantVolley": {
		dir:             "ValiantVolley",
		prefab:          "valiantVolley.prefab",
		roleFields:      []string{"volleyObject"},
		wantControllers: true,
		commonSounds:    []string{"nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{"ants", "volleyObject", "multiIntervalStartBeat", "hitPitch"}},
			{name: "object", markers: []string{
				"enterCurve", "bounceCurve1", "bounceCurve2", "hitCurve", "barelyCurve",
				"objectTransform", "objectSprite", "fruitSprite", "missImpact",
			}, atPath: "ObjectHolder", curveFields: []string{
				"enterCurve", "bounceCurve1", "bounceCurve2", "hitCurve", "barelyCurve",
			}},
		},
	},
	"warioDeMambo": basicOfficialSceneSpec("WarioDeMambo", "warioDeMambo.prefab"),
	"wizardsWaltz": basicOfficialSceneSpec("WizardsWaltz", "wizardsWaltz.prefab"),
	"workingDough": basicOfficialSceneSpec("WorkingDough", "workingDough.prefab"),
}

func init() {
	for id, spec := range officialBaseSceneSpecs {
		if _, ok := sceneSpecs[id]; !ok {
			sceneSpecs[id] = spec
		}
	}
}

func basicOfficialSceneSpec(dir, prefab string) sceneSpec {
	return sceneSpec{
		dir:             dir,
		prefab:          prefab,
		wantControllers: true,
	}
}
