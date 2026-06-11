// genriq 生成一个自包含的测试谱面 demo.riq（Jukebox v2 布局）：
//
//	Charts/chart0.json  谱面：karateman/hit 事件 + 两段 tempo（120 -> 140 BPM）
//	Music/song0.wav     合成音轨（底鼓/闭镲/起始报数），与谱面共用同一 tempo map
//
// 音轨完全程序合成，不含任何版权资产。
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"hsdemo/riq"
	"hsdemo/synth"
)

// 谱面常量：节拍号见下方 hits 注释。
var (
	tempos = []riq.TempoChange{
		{Beat: 0, BPM: 120},
		{Beat: 48, BPM: 140},
	}

	// karateman/hit 事件（判定拍）。前 8 拍为前奏报数。
	hits = []float64{
		8, 10, 12, 14, // 热身：隔拍
		16, 18, 20, 21, 22, // 加入连击
		24, 26, 28, 30,
		32, 33, 34, 35, 36, // 连续五连
		40, 42, 44, 46,
		48, 50, 52, 53, 54, // 变速后（140 BPM）
		56, 57, 58, 59, 60,
		64, 65, 66, 67, 68, 69, 70, // 收尾长连击
	}

	endBeat = 74.0
)

type v2Entity struct {
	Type    int            `json:"type"`
	Version int            `json:"version"`
	Model   int            `json:"model"`
	Data    map[string]any `json:"data"`
}

type v2Chart struct {
	Version  int               `json:"version"`
	Offset   float64           `json:"offset"`
	SongName string            `json:"songname"`
	Entities []v2Entity        `json:"entities"`
	Models   map[string]string `json:"models"`
	Types    map[string]string `json:"types"`
}

func main() {
	out := flag.String("o", "demo.riq", "输出 .riq 路径")
	flag.Parse()

	bm := &riq.Beatmap{Offset: 0, Tempos: tempos}

	chart := buildChart()
	music := buildMusic(bm)

	if err := writeRiq(*out, chart, music); err != nil {
		log.Fatalf("write riq: %v", err)
	}
	fmt.Printf("wrote %s: %d hits, %.1fs of music, tempo 120->140 BPM at beat 48\n",
		*out, len(hits), bm.BeatToTime(endBeat+2))
}

func buildChart() []byte {
	const (
		modelTempo = 0 // "global/tempo change"
		modelHit   = 1 // "karateman/hit"
		typeTempo  = 0 // riq__TempoChange
		typeEntity = 1 // riq__Entity
	)

	var entities []v2Entity
	for _, tc := range tempos {
		entities = append(entities, v2Entity{
			Type:  typeTempo,
			Model: modelTempo,
			Data:  map[string]any{"beat": tc.Beat, "length": 0.0, "tempo": tc.BPM},
		})
	}
	for _, beat := range hits {
		entities = append(entities, v2Entity{
			Type:  typeEntity,
			Model: modelHit,
			// "type": 0 即原版 Karate Man 的 pot（陶罐）
			Data: map[string]any{"beat": beat, "length": 0.0, "type": 0.0},
		})
	}

	c := v2Chart{
		Version:  2,
		Offset:   0,
		SongName: "song0",
		Entities: entities,
		Models: map[string]string{
			"0": "global/tempo change",
			"1": "karateman/hit",
		},
		Types: map[string]string{
			"0": "riq__TempoChange",
			"1": "riq__Entity",
		},
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Fatalf("marshal chart: %v", err)
	}
	return b
}

// buildMusic 按谱面的 tempo map 合成伴奏，保证音乐与判定拍严格对齐。
func buildMusic(bm *riq.Beatmap) []byte {
	total := bm.BeatToTime(endBeat+2) + 0.5
	tr := synth.NewTrack(total)

	kick, hat := synth.Kick(), synth.Hat()

	for beat := 0.0; beat <= endBeat; beat++ {
		tr.Add(bm.BeatToTime(beat), kick, 0.9)
		tr.Add(bm.BeatToTime(beat+0.5), hat, 0.4)
	}
	// 前奏报数：4 长 1 高，提示玩家入拍
	for _, b := range []float64{4, 5, 6} {
		tr.Add(bm.BeatToTime(b), synth.Beep(440, 0.15), 0.5)
	}
	tr.Add(bm.BeatToTime(7), synth.Beep(880, 0.2), 0.5)

	// 抛出音效（objectOut.ogg）由游戏内在 spawn 时刻播放，不再烘焙进音轨

	return tr.WAV()
}

func writeRiq(path string, chart, music []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{"Charts/chart0.json", chart},
		{"Music/song0.wav", music},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := w.Write(entry.data); err != nil {
			return err
		}
	}
	return zw.Close()
}
