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
		scores: []resultScoreInput{
			{Beat: 1, Accuracy: 1, Weight: 1, Category: 0},
			{Beat: 9, Accuracy: 0.9, Weight: 1, Category: 1},
		},
		starGot: true,
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
		result: resultSummary{
			Score: 0.95, Rank: resultRankHi, Header: "Rhythm League Notes",
			Message0: "That was great! Really great!", Star: true, NoMiss: true,
		},
		resultT:      resultRankTime,
		resultAssets: loadResultAssets("../assets/common/ratings"),
	}
	setTestFaces(t, app)
	screen := ebiten.NewImage(ScreenW, ScreenH)
	app.drawResult(screen, colorWhite())
	app.resultEpilogue = true
	app.drawResult(screen, colorWhite())
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
	}
	setTestFaces(t, app)
	screen := ebiten.NewImage(ScreenW, ScreenH)
	app.drawLevelSelect(screen, colorWhite(), color.RGBA{170, 170, 180, 255})
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
