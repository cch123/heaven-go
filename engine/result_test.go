package engine

import (
	"bytes"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/riq"
)

func TestResultAccuracyCurveMatchesJudgementThresholds(t *testing.T) {
	tests := []struct {
		name string
		diff float64
		want float64
	}{
		{name: "ace", diff: 0, want: 1},
		{name: "just edge", diff: WinJust, want: rankHiThreshold},
		{name: "ng edge", diff: WinNG, want: 0},
	}
	for _, tt := range tests {
		if got := accuracyForDiff(tt.diff); math.Abs(got-tt.want) > 1e-9 {
			t.Fatalf("%s accuracy = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBuildResultSummaryUsesCategoryMessages(t *testing.T) {
	app := &App{
		chartRuntimeState: chartRuntimeState{
			bm: &riq.Beatmap{
				Properties: map[string]any{
					"resultcaption": "Notes",
					"resultcat0_hi": "Great fundamentals.",
					"resultcat1_hi": "Strong keeping.",
				},
				Sections: []riq.SectionMarker{
					{Beat: 0, Weight: 1, Category: 0},
					{Beat: 8, Weight: 1, Category: 1},
				},
			},
		},
		scoreRuntimeState: scoreRuntimeState{
			scores: []resultScoreInput{
				{Beat: 1, Accuracy: 1, Weight: 1, Category: 0},
				{Beat: 9, Accuracy: 0.9, Weight: 1, Category: 1},
			},
			starGot: true,
		},
	}

	res := app.buildResultSummary()
	if res.Rank != resultRankHi {
		t.Fatalf("rank = %v, want Hi", res.Rank)
	}
	if !res.TwoMessage {
		t.Fatal("expected two category messages")
	}
	if res.Message1 != "Great fundamentals." {
		t.Fatalf("message1 = %q", res.Message1)
	}
	if res.Message2 != "Also... strong keeping." {
		t.Fatalf("message2 = %q", res.Message2)
	}
	if !res.Star || !res.NoMiss {
		t.Fatalf("star/noMiss = %v/%v, want true/true", res.Star, res.NoMiss)
	}
}

func TestDrawResultSmoke(t *testing.T) {
	app := &App{
		resultRuntimeState: resultRuntimeState{
			result: resultSummary{
				Score: 0.95, Rank: resultRankHi, Header: "Rhythm League Notes",
				Message0: "That was great! Really great!", Star: true, NoMiss: true,
			},
			resultT:      resultRankTime,
			resultAssets: loadResultAssets("../assets/common/ratings"),
		},
	}
	setTestFaces(t, app)
	screen := ebiten.NewImage(ScreenW, ScreenH)
	app.drawResult(screen, colorWhite())
	app.resultEpilogue = true
	app.drawResult(screen, colorWhite())
}

func TestReturnToLevelSelectClearsActiveChart(t *testing.T) {
	app := &App{
		chartRuntimeState: chartRuntimeState{
			bm: &riq.Beatmap{},
			r:  &riq.Riq{Beatmap: &riq.Beatmap{}},
		},
		moduleRuntimeState: moduleRuntimeState{
			modules: map[string]Module{},
			switches: []gameSwitch{
				{beat: 0, id: "meatGrinder"},
			},
			actions: []beatAction{
				{beat: 1},
			},
		},
		inputRuntimeState: inputRuntimeState{
			inputs: []*Input{
				{Beat: 1, Result: JudgeAce},
			},
		},
		appFlowState: appFlowState{state: stateResult},
		resultRuntimeState: resultRuntimeState{
			result:         resultSummary{Score: 0.8, Rank: resultRankHi},
			resultT:        2,
			resultEpilogue: true,
		},
		menuRuntimeState: menuRuntimeState{
			levels: []menuLevel{
				{title: "Meat Grinder"},
				{title: "Practice"},
			},
			menuSel: 1,
		},
	}

	app.returnToLevelSelect()

	if app.state != stateTitle {
		t.Fatalf("state = %v, want title/level select", app.state)
	}
	if app.bm != nil || app.r != nil || app.cond != nil || app.player != nil {
		t.Fatalf("active chart was not unloaded: bm=%v r=%v cond=%v player=%v", app.bm, app.r, app.cond, app.player)
	}
	if app.modules != nil || app.switches != nil || app.actions != nil || app.inputs != nil {
		t.Fatalf("runtime timeline was not cleared")
	}
	if app.resultEpilogue || app.resultT != 0 || app.result.Score != 0 {
		t.Fatalf("result state = epilogue %v t %.2f score %.2f, want reset", app.resultEpilogue, app.resultT, app.result.Score)
	}
	if app.menuSel != 1 {
		t.Fatalf("menuSel = %d, want to preserve the selected library item", app.menuSel)
	}
}

func TestResultAudioAssetsDecode(t *testing.T) {
	audio := loadResultAudio("../assets/common/result_sounds")
	for _, name := range []string{
		"resultMessage2", "resultMessage3", "resultGauge", "resultGaugeStop",
		"resultRankNg", "resultRankOk", "resultRankHi", "resultStarGet", "resultNoMiss",
		"mus_tryagain00", "mus_tryagain01", "mus_ok00", "mus_ok01", "mus_superb00", "mus_superb01",
		"jgl_tryagain", "jgl_ok", "jgl_superb",
	} {
		if len(audio.clips[strings.ToLower(name)]) == 0 {
			t.Fatalf("missing decoded result audio %s", name)
		}
	}
}

func TestDrawLevelSelectSmoke(t *testing.T) {
	app := &App{
		menuRuntimeState: menuRuntimeState{
			levels: []menuLevel{
				{
					path:     "levels/Candy Remix.riq",
					fileName: "Candy Remix",
					title:    "Candy Remix",
					author:   "wookywok, saladplainzone",
					desc:     "Ooh, candy? Don't mind if I do!",
					games:    []string{"munchyMonk", "seeSaw", "blueBear", "marchingOrders"},
					bpm:      195,
				},
				{
					path:     "levels/Meat Grinder.riq",
					fileName: "Meat Grinder",
					title:    "Meat Grinder",
					author:   "Seanski2",
					games:    []string{"meatGrinder"},
					bpm:      128,
				},
			},
			libraryAssets: loadLibraryAssets("../assets/common/library"),
		},
	}
	setTestFaces(t, app)
	screen := ebiten.NewImage(ScreenW, ScreenH)
	app.drawLevelSelect(screen, colorWhite(), color.RGBA{170, 170, 180, 255})
}

func TestTextboxDrawSmoke(t *testing.T) {
	var tb textboxFX
	tb.add(&riq.Entity{
		Beat: 1, Length: 5,
		Data: map[string]any{
			"text1": "<align=center>Next, we'll be throwing a chain of smaller pieces of meat at you.",
			"type":  float64(1),
			"valA":  1.1715,
			"valB":  1.0,
		},
	})
	screen := ebiten.NewImage(ScreenW, ScreenH)
	tb.Draw(screen, "../assets", 2)
}

func colorWhite() color.RGBA { return color.RGBA{245, 245, 250, 255} }

func setTestFaces(t *testing.T, app *App) {
	t.Helper()
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatalf("load test font: %v", err)
	}
	app.faceBig = &text.GoTextFace{Source: src, Size: 44}
	app.faceMid = &text.GoTextFace{Source: src, Size: 24}
	app.faceSmall = &text.GoTextFace{Source: src, Size: 15}
}
