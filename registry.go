package main

import (
	"hsdemo/engine"
	"hsdemo/games/agbsamuraislice"
	"hsdemo/games/airrally"
	"hsdemo/games/balloonhunter"
	"hsdemo/games/basketballgirls"
	"hsdemo/games/bigrockfinish"
	"hsdemo/games/bluebear"
	"hsdemo/games/bluebirds"
	"hsdemo/games/boardmeeting"
	"hsdemo/games/bonodori"
	"hsdemo/games/bossanova"
	"hsdemo/games/bouncyroad"
	"hsdemo/games/cannery"
	"hsdemo/games/catchoftheday"
	"hsdemo/games/catchytune"
	"hsdemo/games/chameleon"
	"hsdemo/games/cheerreaders"
	"hsdemo/games/clappytrio"
	"hsdemo/games/claptrap"
	"hsdemo/games/cointoss"
	"hsdemo/games/cropstomp"
	"hsdemo/games/djschool"
	"hsdemo/games/dogninja"
	"hsdemo/games/doubledate"
	"hsdemo/games/dressyourbest"
	"hsdemo/games/drummingpractice"
	"hsdemo/games/fallingwaffle"
	"hsdemo/games/fanclub"
	"hsdemo/games/figurefighter"
	"hsdemo/games/fireworks"
	"hsdemo/games/firstcontact"
	"hsdemo/games/flipperflop"
	"hsdemo/games/forklifter"
	"hsdemo/games/frogprincess"
	"hsdemo/games/fruitbasket"
	"hsdemo/games/gardendance"
	"hsdemo/games/gleeclub"
	"hsdemo/games/holeinone"
	"hsdemo/games/kitties"
	"hsdemo/games/launchparty"
	"hsdemo/games/lockstep"
	"hsdemo/games/lovelizards"
	"hsdemo/games/loverap"
	"hsdemo/games/mannequinfactory"
	"hsdemo/games/manzai"
	"hsdemo/games/marchingorders"
	"hsdemo/games/meatgrinder"
	"hsdemo/games/moaidoowop"
	"hsdemo/games/mrupbeat"
	"hsdemo/games/munchymonk"
	"hsdemo/games/nailcarpenter"
	"hsdemo/games/ninjabodyguard"
	"hsdemo/games/nipinthebud"
	"hsdemo/games/nogame"
	"hsdemo/games/octopusmachine"
	"hsdemo/games/packingpests"
	"hsdemo/games/pajamaparty"
	"hsdemo/games/powercalligraphy"
	"hsdemo/games/quizshow"
	"hsdemo/games/rhythmfighter"
	"hsdemo/games/rhythmsheriff"
	"hsdemo/games/rhythmtestgba"
	"hsdemo/games/rhythmtweezers"
	"hsdemo/games/ringside"
	"hsdemo/games/samuraislicentr"
	"hsdemo/games/samuraislicervl"
	"hsdemo/games/seesaw"
	"hsdemo/games/shootemup"
	"hsdemo/games/showtime"
	"hsdemo/games/slotmonster"
	"hsdemo/games/sneakyspirits"
	"hsdemo/games/somen"
	"hsdemo/games/spaceball"
	"hsdemo/games/spacedance"
	"hsdemo/games/splashdown"
	"hsdemo/games/tambourine"
	"hsdemo/games/taptrial"
	"hsdemo/games/taptroupe"
	"hsdemo/games/thedazzles"
	"hsdemo/games/totemclimb"
	"hsdemo/games/tramandpauline"
	"hsdemo/games/trickclass"
	"hsdemo/games/tunnel"
	"hsdemo/games/valiantvolley"
	"hsdemo/games/wizardswaltz"
	"hsdemo/games/workingdough"
)

func registerGames() {
	engine.Register("rhythmSomen", somen.New)
	engine.Register("agbSamuraiSlice", agbsamuraislice.New)
	engine.Register("airRally", airrally.New)
	engine.Register("basketballGirls", basketballgirls.New)
	engine.Register("balloonHunter", balloonhunter.New)
	engine.Register("bigRockFinish", bigrockfinish.New)
	engine.Register("bossaNova", bossanova.New)
	engine.Register("bonOdori", bonodori.New)
	engine.Register("boardMeeting", boardmeeting.New)
	engine.Register("bouncyRoad", bouncyroad.New)
	engine.Register("cannery", cannery.New)
	engine.Register("catchOfTheDay", catchoftheday.New)
	engine.Register("catchyTune", catchytune.New)
	engine.Register("clappyTrio", clappytrio.New)
	engine.Register("chameleon", chameleon.New)
	engine.Register("clapTrap", claptrap.New)
	engine.Register("coinToss", cointoss.New)
	engine.Register("cropStomp", cropstomp.New)
	engine.Register("djSchool", djschool.New)
	engine.Register("dogNinja", dogninja.New)
	engine.Register("doubleDate", doubledate.New)
	engine.Register("dressYourBest", dressyourbest.New)
	engine.Register("drummingPractice", drummingpractice.New)
	engine.Register("figureFighter", figurefighter.New)
	engine.Register("fireworks", fireworks.New)
	engine.Register("fallingWaffle", fallingwaffle.New)
	engine.Register("fanClub", fanclub.New)
	engine.Register("firstContact", firstcontact.New)
	engine.Register("flipperFlop", flipperflop.New)
	engine.Register("forkLifter", forklifter.New)
	engine.Register("fruitBasket", fruitbasket.New)
	engine.Register("frogPrincess", frogprincess.New)
	engine.Register("gardenDance", gardendance.New)
	engine.Register("gleeClub", gleeclub.New)
	engine.Register("holeInOne", holeinone.New)
	engine.Register("tambourine", tambourine.New)
	engine.Register("tapTrial", taptrial.New)
	engine.Register("tapTroupe", taptroupe.New)
	engine.Register("theDazzles", thedazzles.New)
	engine.Register("trickClass", trickclass.New)
	engine.Register("meatGrinder", meatgrinder.New)
	engine.Register("totemClimb", totemclimb.New)
	engine.Register("seeSaw", seesaw.New)
	engine.Register("sneakySpirits", sneakyspirits.New)
	engine.Register("slotMonster", slotmonster.New)
	engine.Register("blueBear", bluebear.New)
	engine.Register("blueBirds", bluebirds.New)
	engine.Register("marchingOrders", marchingorders.New)
	engine.Register("cheerReaders", cheerreaders.New)
	engine.Register("kitties", kitties.New)
	engine.Register("launchParty", launchparty.New)
	engine.Register("lockstep", lockstep.New)
	engine.Register("loveLizards", lovelizards.New)
	engine.Register("loveRap", loverap.New)
	engine.Register("manzai", manzai.New)
	engine.Register("mannequinFactory", mannequinfactory.New)
	engine.Register("moaiDooWop", moaidoowop.New)
	engine.Register("spaceball", spaceball.New)
	engine.Register("spaceDance", spacedance.New)
	engine.Register("splashdown", splashdown.New)
	engine.Register("mrUpbeat", mrupbeat.New)
	engine.Register("munchyMonk", munchymonk.New)
	engine.Register("nailCarpenter", nailcarpenter.New)
	engine.Register("ninjaBodyguard", ninjabodyguard.New)
	engine.Register("nipInTheBud", nipinthebud.New)
	engine.Register("noGame", nogame.New)
	engine.Register("octopusMachine", octopusmachine.New)
	engine.Register("packingPests", packingpests.New)
	engine.Register("pajamaParty", pajamaparty.New)
	engine.Register("powerCalligraphy", powercalligraphy.New)
	engine.Register("quizShow", quizshow.New)
	engine.Register("rhythmFighter", rhythmfighter.New)
	engine.Register("rhythmTestGBA", rhythmtestgba.New)
	engine.Register("rhythmSheriff", rhythmsheriff.New)
	engine.Register("rhythmTweezers", rhythmtweezers.New)
	engine.Register("ringside", ringside.New)
	engine.Register("samuraiSliceNtr", samuraislicentr.New)
	engine.Register("samuraiSliceRvl", samuraislicervl.New)
	engine.Register("shootEmUp", shootemup.New)
	engine.Register("showtime", showtime.New)
	engine.Register("tramAndPauline", tramandpauline.New)
	engine.Register("tunnel", tunnel.New)
	engine.Register("valiantVolley", valiantvolley.New)
	engine.Register("workingDough", workingdough.New)
	engine.Register("wizardsWaltz", wizardswaltz.New)
}
