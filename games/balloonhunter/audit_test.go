package balloonhunter

import (
	"path"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "balloonHunter"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsComponentsCurvesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"slowBalloon":   "BalloonSlow",
		"fastBalloon":   "BalloonFast",
		"balloonFive":   "BalloonFive",
		"bgAnimal":      "BG/AnimalsBG",
		"rock":          "Rock",
		"rockMissCurve": "RockBarelyCurve",
		"hunterAnim":    "Hunter",
		"birdAnim":      "Bird",
		"rockSmear":     "RockSmear",
	}
	for k, want := range wantRoles {
		if got := as.Roles[k]; got != want {
			t.Fatalf("role %s = %q, want %q", k, got, want)
		}
	}

	for name, wantPath := range map[string]string{
		"game":        "",
		"slowBalloon": "BalloonSlow",
		"fastBalloon": "BalloonFast",
		"balloonFive": "BalloonFive",
		"bgAnimal":    "BG/AnimalsBG",
	} {
		c := as.Extra.Components[name]
		if c.Path != wantPath {
			t.Fatalf("component %s path = %q, want %q", name, c.Path, wantPath)
		}
	}
	if got := as.Extra.Components["balloonFive"].Refs["mooseObject"]; got != "BalloonFive/MooseBody" {
		t.Fatalf("balloonFive mooseObject = %q", got)
	}
	if got := as.Extra.Components["bgAnimal"].Refs["rabbitAnim"]; got != "BG/AnimalsBG/BunnyBody" {
		t.Fatalf("bgAnimal rabbitAnim = %q", got)
	}
	if c := as.Extra.Curves["rockMissCurve"]; len(c.Points) != 2 || c.Sampling != 50 {
		t.Fatalf("rockMissCurve = %#v", c)
	}

	for _, snd := range []string{
		"blow", "charge", "miss", "moose_raspberry", "moose_uhoh", "pop",
		"tweetN_base", "tweetN_both", "tweetN_fast1", "tweetN_fast2", "tweetN_slow",
		"tweet_both", "tweet_fast", "tweet_five", "tweet_slow",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimationClipsControllersAndPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for _, clip := range []string{
		"BG/AnimalBoar", "BG/AnimalMoose", "BG/AnimalRabbit", "BG/AnimalScroll", "BG/CloudScroll",
		"Animals/BoarBop", "Animals/BunnyBop", "Animals/MooseBop",
		"Balloon/SlowBalloonMove", "Balloon/FastBalloonMove", "Balloon/BalloonFiveMove", "Balloon/BalloonMiss",
		"Moose/MooseFall", "Moose/MooseRaspberry", "Moose/MooseShock",
		"Bird/BirdBop", "Bird/BirdCover", "Bird/BirdFlap", "Bird/BirdJump", "Bird/BirdScared",
		"Faces/BirdCall", "Faces/BirdCallBig", "Faces/BirdCallScared", "Faces/BirdHappy", "Faces/BirdNeutral", "Faces/BirdSad",
		"Hunter/HunterBlow", "Hunter/HunterBop", "Hunter/HunterMiss", "Hunter/HunterPrepare",
		"Hunter/HunterShoot", "Hunter/HunterToss", "Hunter/HunterTossPrep",
		"Faces/HunterDetermined", "Faces/HunterHappy", "Faces/HunterHold", "Faces/HunterNeutral",
		"Faces/HunterPrepTossFace", "Faces/HunterReady", "Faces/HunterSad", "Faces/HunterShock",
		"Animations/PopEffect", "Animations/RockSmear",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}

	for root, ctrl := range map[string]string{
		"Hunter":                 "HunterAnim",
		"Bird":                   "BirdAnim",
		"BalloonSlow":            "SlowBalloonAnim",
		"BalloonFast":            "FastBalloonAnim",
		"BalloonFive":            "BalloonFiveAnim",
		"PopLeft":                "PopEffectAnim",
		"PopRight":               "PopEffectAnim",
		"PopMiddle":              "PopEffectAnim",
		"RockSmear":              "RockSmearAnim",
		"BG/CloudHolder":         "CloudHolder",
		"BG/AnimalsBG":           "AnimalsBG",
		"BG/AnimalsBG/BunnyBody": "BunnyBody",
		"BG/AnimalsBG/BoarHead":  "BoarHead",
		"BG/AnimalsBG/MooseBody": "MooseBody",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}

	for ctrl, states := range map[string][]string{
		"HunterAnim":      {"Idle", "Bop", "Prepare", "Shoot", "Miss", "HunterTossPrep", "HunterToss", "Neutral", "Hold", "Prep Toss", "Determined", "Shock", "Happy", "Sad"},
		"BirdAnim":        {"Neutral", "Bop", "Flap", "Scared", "Cover", "Call", "CallBig", "CallScared", "Happy", "Sad", "Shock"},
		"SlowBalloonAnim": {"Start", "Move", "Miss"},
		"FastBalloonAnim": {"Start", "Move", "Miss"},
		"BalloonFiveAnim": {"Start", "Move", "Miss", "MooseFall", "MooseRaspberry", "MooseShock"},
		"PopEffectAnim":   {"Start", "Pop"},
		"RockSmearAnim":   {"Idle", "RockSmear"},
		"AnimalsBG":       {"Rabbit", "Boar", "Moose"},
		"BunnyBody":       {"Bop"},
		"BoarHead":        {"Bop"},
		"MooseBody":       {"Bop"},
		"CloudHolder":     {"CloudScroll"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}

	for clip, root := range map[string]string{
		"BG/AnimalBoar": "BG/AnimalsBG", "BG/AnimalMoose": "BG/AnimalsBG", "BG/AnimalRabbit": "BG/AnimalsBG", "BG/AnimalScroll": "BG/AnimalsBG",
		"BG/CloudScroll":  "BG/CloudHolder",
		"Animals/BoarBop": "BG/AnimalsBG/BoarHead", "Animals/BunnyBop": "BG/AnimalsBG/BunnyBody", "Animals/MooseBop": "BG/AnimalsBG/MooseBody",
		"Balloon/SlowBalloonMove": "BalloonSlow", "Balloon/FastBalloonMove": "BalloonFast", "Balloon/BalloonFiveMove": "BalloonFive", "Balloon/BalloonMiss": "BalloonSlow",
		"Moose/MooseFall": "BalloonFive", "Moose/MooseRaspberry": "BalloonFive", "Moose/MooseShock": "BalloonFive",
		"Bird/BirdBop": "Bird", "Bird/BirdCover": "Bird", "Bird/BirdFlap": "Bird", "Bird/BirdJump": "Bird", "Bird/BirdScared": "Bird",
		"Faces/BirdCall": "Bird", "Faces/BirdCallBig": "Bird", "Faces/BirdCallScared": "Bird", "Faces/BirdHappy": "Bird", "Faces/BirdNeutral": "Bird", "Faces/BirdSad": "Bird",
		"Hunter/HunterBlow": "Hunter", "Hunter/HunterBop": "Hunter", "Hunter/HunterMiss": "Hunter", "Hunter/HunterPrepare": "Hunter",
		"Hunter/HunterShoot": "Hunter", "Hunter/HunterToss": "Hunter", "Hunter/HunterTossPrep": "Hunter",
		"Faces/HunterDetermined": "Hunter", "Faces/HunterHappy": "Hunter", "Faces/HunterHold": "Hunter", "Faces/HunterNeutral": "Hunter",
		"Faces/HunterPrepTossFace": "Hunter", "Faces/HunterReady": "Hunter", "Faces/HunterSad": "Hunter", "Faces/HunterShock": "Hunter",
		"Animations/PopEffect": "PopMiddle", "Animations/RockSmear": "RockSmear",
	} {
		assertClipPaths(t, as, clip, root)
	}
}

func TestRuntimeTimingConstants(t *testing.T) {
	b := &balloon{startBeat: 8, speed: 3}
	if target := b.startBeat + b.speed - 1; target != 10 {
		t.Fatalf("normal balloon target = %v, want 10", target)
	}
	five := &balloon{startBeat: 8 + 2, speed: 4, isFive: true}
	if target := five.startBeat + five.speed - 2; target != 12 {
		t.Fatalf("five balloon target = %v, want 12", target)
	}
	if popSprites[0] != "balloonHunterCharacters_33" || len(popSprites) != 3 {
		t.Fatalf("pop particle sprite sequence changed: %#v", popSprites)
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("missing clip %s", clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s curve path %q resolved to missing node %q", clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}

func emptyClip(anim *kmdata.Anim) bool {
	if anim == nil {
		return false
	}
	return len(anim.Pos) == 0 && len(anim.Scale) == 0 && len(anim.Euler) == 0 &&
		len(anim.Sprites) == 0 && len(anim.Floats) == 0
}
