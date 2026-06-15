package main

// officialBaseSceneSpecs gives every bundled Heaven Studio prefab a basic
// extraction entry. These entries are intentionally asset-only: gameplay code
// still has to prove event timing, sounds, effects, and inputs against the C#
// source before a game is registered as playable.
var officialBaseSceneSpecs = map[string]sceneSpec{
	"agbSamuraiSlice": basicOfficialSceneSpec("SamuraiSliceAgb", "agbSamuraiSlice.prefab"),
	"airboarder":      basicOfficialSceneSpec("Airboarder", "airboarder.prefab"),
	"animalAcrobat": {
		dir:    "AnimalAcrobat",
		prefab: "animalAcrobat.prefab",
		roleFields: []string{
			"_elephant", "_giraffe", "_monkeysLong", "_monkeysShort", "_gorilla",
			"_scroll", "_playerMonkey", "_spotlightMain", "_partyPoppers",
		},
		wantControllers: true,
		commonSounds: []string{
			"nearMiss.ogg",
			"count-ins/three1.wav",
			"count-ins/three2.wav",
			"count-ins/go1.wav",
			"count-ins/go2.wav",
		},
		templatePrefabs: []string{
			"Prefabs/Elephant.prefab",
			"Prefabs/Giraffe.prefab",
			"Prefabs/WhiteMonkeys.prefab",
			"Prefabs/WhiteMonkey.prefab",
			"Prefabs/Gorilla.prefab",
			"Prefabs/FireHoop.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"_elephant", "_giraffe", "_monkeysLong", "_monkeysShort", "_gorilla",
				"_scroll", "_playerMonkey", "_spotlightMain", "_bgMat", "_partyPoppers",
				"_jumpDistance", "_jumpDistanceGiraffe", "_jumpStartCameraDistance",
				"_jumpStartDistance", "_giraffeCameraZoom", "_cameraSmoothSpeed",
			}},
			{name: "obstacle", markers: []string{
				"_fullRotRange", "_holdLength", "_ease", "_holdPadding", "_holdPaddingStart",
				"_rotateRoot", "_gripPoint", "_endPoint",
			}, multi: true},
			{name: "player", markers: []string{
				"_scroll", "_shadow", "_releaseParticle", "_trailParticle", "sweatParticle",
				"_jumpDistanceStart", "_jumpDistance", "_jumpHeight", "_jumpHeightInitial",
				"_jumpDistanceGiraffe", "_jumpHeightGiraffe", "_jumpStartAngle",
			}},
			{name: "obstacleInput", markers: []string{
				"animalType", "_monkey", "_gripShadow", "_endShadow",
				"_holdLength", "_holdParticle", "_sweatParticle",
			}, multi: true},
			{name: "giraffeInput", markers: []string{
				"animalType", "_monkey", "_holdParticle", "_sweatParticle",
				"_gripShadow", "_endShadow", "_fireHoopAnim", "_fireHoopSparkle",
			}, multi: true},
			{name: "whiteMonkeysSwing", markers: []string{
				"_anim", "_animLength", "_beatLength", "_ease",
			}, multi: true},
			{name: "earFlap", markers: []string{
				"_anim", "_animName", "_holdLength",
			}, multi: true},
			{name: "bgTileManager", markers: []string{
				"_bgTileFirst", "_bgTileSecond", "_scroll",
			}, multi: true},
			{name: "spotlightShadows", markers: []string{
				"_perspectiveStrength", "_flatOffset", "_shadowColor",
				"_sortingOrderOffset", "_ignoreList",
			}, multi: true},
		},
	},
	"balloonHunter":   basicOfficialSceneSpec("BalloonHunter", "balloonHunter.prefab"),
	"basketballGirls": basicOfficialSceneSpec("BasketballGirls", "basketballGirls.prefab"),
	"bigRockFinish":   basicOfficialSceneSpec("BigRockFinish", "bigRockFinish.prefab"),
	"blueBirds":       basicOfficialSceneSpec("BlueBirds", "blueBirds.prefab"),
	"boardMeeting":    basicOfficialSceneSpec("BoardMeeting", "boardMeeting.prefab"),
	"bonOdori":        basicOfficialSceneSpec("BonOdori", "bonOdori.prefab"),
	"bossaNova":       basicOfficialSceneSpec("BossaNova", "bossaNova.prefab"),
	"bouncyRoad":      basicOfficialSceneSpec("BouncyRoad", "bouncyRoad.prefab"),
	"builtToScaleDS":  basicOfficialSceneSpec("BuiltToScaleDS", "builtToScaleDS.prefab"),
	"builtToScaleRvl": {
		dir:    "BuiltToScaleRvl",
		prefab: "builtToScaleRvl.prefab",
		roleFields: []string{
			"baseRod", "baseLeftSquare", "baseRightSquare", "baseAssembled", "widgetHolder",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"blocks", "baseRod", "baseLeftSquare", "baseRightSquare",
				"baseAssembled", "widgetHolder", "curve", "missCurve",
			}, curveArrayFields: []string{"curve", "missCurve"}},
			{name: "block", markers: []string{
				"position", "_slideOffset",
			}, multi: true},
			{name: "rod", markers: []string{
				"missAngle", "fallingAngle",
			}, atPath: "prefabs/rod"},
			{name: "square", markers: []string{
				"anim", "CorrectionPos",
			}, multi: true},
		},
	},
	"cannery": basicOfficialSceneSpec("Cannery", "cannery.prefab"),
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
	"dressYourBest": basicOfficialSceneSpec("DressYourBest", "dressYourBest.prefab"),
	"drummerDuel": {
		dir:    "DrummerDuel",
		prefab: "drummerDuel.prefab",
		roleFields: []string{
			"referee", "taikoLeft", "taikoRight", "drummerLeft", "drummerRight",
			"refereeObj", "refereePlatformObj", "cheerLeadersObj",
		},
		refArrayFields:  []string{"cheerLeadersLeft", "cheerLeadersRight"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"referee", "cheerLeadersLeft", "cheerLeadersRight",
				"taikoLeft", "taikoRight", "drummerLeft", "drummerRight",
				"taikoRightStars", "drummerLeftFaceMaterial", "drummerRightFaceMaterial",
				"camera", "cameraLeft", "cameraCenter", "cameraRight",
				"refereeObj", "refereePlatformObj", "cheerLeadersObj",
			}},
		},
	},
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
	"gardenDance": basicOfficialSceneSpec("GardenDance", "gardenDance.prefab"),
	"gleeClub":    basicOfficialSceneSpec("GleeClub", "gleeClub.prefab"),
	"holeInOne":   basicOfficialSceneSpec("HoleInOne", "holeInOne.prefab"),
	"launchParty": basicOfficialSceneSpec("LaunchParty", "launchParty.prefab"),
	"loveLab":     basicOfficialSceneSpec("LoveLab", "loveLab.prefab"),
	"loveLizards": basicOfficialSceneSpec("LoveLizards", "loveLizards.prefab"),
	"loveRap":     basicOfficialSceneSpec("LoveRap", "loveRap.prefab"),
	"lumbearjack": basicOfficialSceneSpec("Lumbearjack", "lumbearjack.prefab"),
	"magicGirl": {
		dir:    "MagicGirl",
		prefab: "magicGirl.prefab",
		roleFields: []string{
			"MakoObject", "TransfComponent", "Mako", "MakoFace",
			"MonsterHands", "backgroundSprite", "bgImage", "jumpEffect",
		},
		refArrayFields:  []string{"Monsters"},
		wantControllers: true,
		commonSounds:    []string{"miss.wav", "nearMiss.ogg"},
		components: []componentSpec{
			{name: "game", markers: []string{
				"MakoObject", "TransfComponent", "Mako", "MakoFace",
				"MonsterHands", "Monsters", "backgroundSprite", "bgImage",
				"gradientMat", "jumpEffect",
			}},
			{name: "monster", markers: []string{
				"relativeBeat", "hasSpawned", "isFleeing", "anim",
				"location", "fleeCurve", "normalLocation", "hitEffect",
			}, multi: true, curveFields: []string{"fleeCurve"}},
		},
	},
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
	"powerCalligraphy": {
		dir:    "PowerCalligraphy",
		prefab: "powerCalligraphy.prefab",
		roleFields: []string{
			"shiftHolder", "paperHolder", "endPaper", "BGPlane",
			"fudePosAnim", "fudeAnim", "shiftAnim", "playerFude",
		},
		refArrayFields:  []string{"basePapers", "Chounin"},
		wantControllers: true,
		templatePrefabs: []string{
			"Prefabs/paper_re.prefab",
			"Prefabs/paper_comma.prefab",
			"Prefabs/paper_chikara.prefab",
			"Prefabs/paper_onore.prefab",
			"Prefabs/paper_sun.prefab",
			"Prefabs/paper_kokoro.prefab",
			"Prefabs/paper_face.prefab",
			"Prefabs/paper_face_kr.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"basePapers", "fudePosCntls", "shiftCntls", "shiftHolder",
				"paperHolder", "endPaper", "Chounin", "BGPlane",
				"fudePosAnim", "fudeAnim", "shiftAnim", "playerFude",
				"scrollSpeed", "chouninSpeed",
			}},
			{name: "writing", markers: []string{"AnimPattern", "startBeat"}, multi: true},
			{name: "fude", markers: []string{
				"handRenderer", "thumbRenderer", "stickRenderer", "tipRenderer",
				"ballRenderer", "REDRATE_1", "REDRATE_2", "sprites",
			}, atPath: "shift/fude/sprite"},
		},
	},
	"quizShow":        basicOfficialSceneSpec("QuizShow", "quizShow.prefab"),
	"rapMen":          basicOfficialSceneSpec("RapMen", "rapMen.prefab"),
	"rhythmFighter":   basicOfficialSceneSpec("RhythmFighter", "rhythmFighter.prefab"),
	"rhythmRally":     basicOfficialSceneSpec("RhythmRally", "rhythmRally.prefab"),
	"rhythmSheriff":   basicOfficialSceneSpec("RhythmSheriff", "rhythmSheriff.prefab"),
	"rhythmTestGBA":   basicOfficialSceneSpec("RhythmTestGBA", "rhythmTestGBA.prefab"),
	"rhythmTweezers":  basicOfficialSceneSpec("RhythmTweezers", "rhythmTweezers.prefab"),
	"ringside":        basicOfficialSceneSpec("Ringside", "ringside.prefab"),
	"rockers":         basicOfficialSceneSpec("Rockers", "rockers.prefab"),
	"samuraiSliceNtr": basicOfficialSceneSpec("SamuraiSliceNtr", "samuraiSliceNtr.prefab"),
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
	"shootEmUp":   basicOfficialSceneSpec("ShootEmUp", "shootEmUp.prefab"),
	"showtime":    basicOfficialSceneSpec("Showtime", "showtime.prefab"),
	"sickBeats":   basicOfficialSceneSpec("SickBeats", "sickBeats.prefab"),
	"slotMonster": basicOfficialSceneSpec("SlotMonster", "slotMonster.prefab"),
	"spaceSoccer": {
		dir:    "SpaceSoccer",
		prefab: "spaceSoccer.prefab",
		roleFields: []string{
			"kickerPrefab", "ballRef", "backgroundSprite", "bgImage", "bg",
		},
		refArrayFields:  []string{"kickers"},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"kickerPrefab", "ballRef", "kickers", "backgroundSprite", "bgImage", "bg",
				"ballPaths", "xBaseSpeed", "yBaseSpeed", "kickerMat", "mouthMat", "platMat", "fireMat",
			}},
			{name: "ball", markers: []string{"holder", "spriteHolder", "startBeat", "state"}, atPath: "BallHolder"},
			{name: "kicker", markers: []string{"canKick", "canHighKick", "enterExitAnim"}, multi: true},
		},
	},
	"spaceball":         basicOfficialSceneSpec("SpaceBall", "spaceball.prefab"),
	"splashdown":        basicOfficialSceneSpec("Splashdown", "splashdown.prefab"),
	"sumoBrothers":      basicOfficialSceneSpec("SumoBrothers", "sumoBrothers.prefab"),
	"superSamuraiSlice": basicOfficialSceneSpec("SuperSamuraiSlice", "superSamuraiSlice.prefab"),
	"tapTroupe":         basicOfficialSceneSpec("TapTroupe", "tapTroupe.prefab"),
	"theDazzles":        basicOfficialSceneSpec("TheDazzles", "theDazzles.prefab"),
	"tossBoys": {
		dir:    "TossBoys",
		prefab: "tossBoys.prefab",
		roleFields: []string{
			"akachan", "aokun", "kiiyan", "hatchAnim", "soshiAnim",
			"ballPrefab", "specialAka", "specialAo", "specialKii",
			"soshi", "bg", "soshiPants",
		},
		wantControllers: true,
		components: []componentSpec{
			{name: "game", markers: []string{
				"akachan", "aokun", "kiiyan", "hatchAnim", "soshiAnim",
				"ballPrefab", "specialAka", "specialAo", "specialKii",
				"soshi", "currentSpecialKid", "bg", "soshiPants",
				"soshiMat", "guitarMat", "ballPaths",
			}},
			{name: "kid", markers: []string{
				"_hitEffect", "arrow", "prefix", "crouch",
			}, multi: true},
			{name: "ball", markers: []string{"willBePopped"}, atPath: "Ball"},
		},
	},
	"tramAndPauline": basicOfficialSceneSpec("TramAndPauline", "tramAndPauline.prefab"),
	"tunnel":         basicOfficialSceneSpec("Tunnel", "tunnel.prefab"),
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
	"workingDough": {
		dir:    "WorkingDough",
		prefab: "workingDough.prefab",
		roleFields: []string{
			"doughDudesNPC", "doughDudesPlayer",
			"ballTransporterRightNPC", "ballTransporterLeftNPC",
			"ballTransporterRightPlayer", "ballTransporterLeftPlayer",
			"npcImpact", "playerImpact",
			"smallBallNPC", "bigBallNPC", "ballHolder",
			"arrowSRLeftNPC", "arrowSRRightNPC", "arrowSRLeftPlayer", "arrowSRRightPlayer",
			"NPCBallTransporters", "PlayerBallTransporters",
			"playerEnterSmallBall", "playerEnterBigBall",
			"missImpact", "breakParticleHolder", "breakParticleEffect",
			"conveyerAnimator", "smallBGBall", "bigBGBall",
			"spaceshipAnimator", "spaceshipLights", "doughDudesHolderAnim", "gandwAnim",
			"backgroundSR", "flashSR", "shipObject",
		},
		wantControllers: true,
		commonSounds:    []string{"miss.wav"},
		templatePrefabs: []string{
			"Prefabs/Small_Ball.prefab",
			"Prefabs/Big_Ball.prefab",
			"Prefabs/PlayerEnterSmallBall.prefab",
			"Prefabs/PlayerEnterBigBall.prefab",
			"Prefabs/BGSmallBall.prefab",
			"Prefabs/BGBigBall.prefab",
			"Prefabs/BreakingParticle.prefab",
		},
		components: []componentSpec{
			{name: "game", markers: []string{
				"doughDudesNPC", "doughDudesPlayer",
				"ballTransporterRightNPC", "ballTransporterLeftNPC",
				"ballTransporterRightPlayer", "ballTransporterLeftPlayer",
				"npcImpact", "playerImpact", "ballHolder",
				"arrowSRLeftNPC", "arrowSRRightNPC", "arrowSRLeftPlayer", "arrowSRRightPlayer",
				"NPCBallTransporters", "PlayerBallTransporters", "missImpact",
				"breakParticleHolder", "conveyerAnimator", "spaceshipAnimator",
				"spaceshipLights", "doughDudesHolderAnim", "gandwAnim",
				"backgroundSR", "flashSR", "bgObjects", "shipObject",
				"ballBouncePaths", "whiteArrowSprite", "redArrowSprite",
			}},
		},
	},
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
