// Package riq 实现 Heaven Studio 谱面格式 .riq 的加载。
//
// .riq 是一个 ZIP 容器，存在两代布局：
//
//	v1（旧版 Jukebox）:
//	    remix.json   谱面（实体扁平存储 datamodel/beat/length/dynamicData）
//	    song.bin     音频（扩展名固定 .bin，实际类型由 magic bytes 判定）
//	v2（当前 Jukebox）:
//	    Charts/chart0.json   谱面（models/types 索引表 + data 字典）
//	    Music/song0.*        音频（文件主名 = songname 字段）
//
// 本包把两种布局归一化为同一个 Beatmap 结构，并提供
// 节拍 <-> 时间的分段线性映射（tempo map）。
package riq

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// AudioFormat 是音频容器类型，由内容嗅探得出（v1 的 song.bin 扩展名不可信）。
type AudioFormat int

const (
	AudioUnknown AudioFormat = iota
	AudioWAV
	AudioOGG
	AudioMP3
)

func (f AudioFormat) String() string {
	switch f {
	case AudioWAV:
		return "wav"
	case AudioOGG:
		return "ogg"
	case AudioMP3:
		return "mp3"
	}
	return "unknown"
}

// Entity 是归一化后的谱面事件。
// Datamodel 形如 "karateman/hit"，'/' 前是 minigame 名，后是事件名。
type Entity struct {
	Datamodel string
	Type      string // v2 的类型标签，如 riq__Entity / riq__TempoChange
	Beat      float64
	Length    float64
	Data      map[string]any // 事件的动态参数
}

// Game 返回 datamodel 的 minigame 部分。
func (e *Entity) Game() string {
	if i := strings.IndexByte(e.Datamodel, '/'); i >= 0 {
		return e.Datamodel[:i]
	}
	return e.Datamodel
}

// Float 取动态参数并做宽松的数值转换（JSON 反序列化数值统一为 float64；
// 布尔参数在部分官方谱面序列化为 JSON true/false——Character Select 的
// beat intervals.auto——按 1/0 转换，否则 boolParam 系调用会吞掉 true）。
func (e *Entity) Float(key string, def float64) float64 {
	if v, ok := e.Data[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case bool:
			if n {
				return 1
			}
			return 0
		}
	}
	return def
}

// Str 取动态参数字符串。
func (e *Entity) Str(key, def string) string {
	if v, ok := e.Data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// TempoChange 是 tempo map 的一个节点：自 Beat 起以 BPM 推进。
type TempoChange struct {
	Beat float64
	BPM  float64
}

// Beatmap 是归一化后的谱面。
type Beatmap struct {
	Version    int
	Offset     float64 // 秒：beat 0 相对音频开头的偏移
	SongName   string
	Entities   []Entity      // 仅游戏事件（tempo/volume/section 已剥离）
	Tempos     []TempoChange // 升序且保证首节点 Beat <= 0
	Properties map[string]any
	Volumes    []VolumeChange  // 升序；空 = 恒定 1.0
	Sections   []SectionMarker // 升序
}

// VolumeChange 是音量节点：自 Beat 起经 Length 拍线性渐变到 Volume（0~1）。
type VolumeChange struct {
	Beat   float64
	Volume float64
	Length float64
}

// SectionMarker 是关卡分段标记。
type SectionMarker struct {
	Beat float64
	Name string
}

// VolumeAt 返回某拍处的音乐音量（0~1），节点间取该拍所属渐变段的插值。
func (b *Beatmap) VolumeAt(beat float64) float64 {
	vol := 1.0
	for i := range b.Volumes {
		v := &b.Volumes[i]
		if beat < v.Beat {
			break
		}
		if v.Length > 0 && beat < v.Beat+v.Length {
			u := (beat - v.Beat) / v.Length
			return vol + (v.Volume-vol)*u
		}
		vol = v.Volume
	}
	return vol
}

// SectionAt 返回某拍所属分段名（无则空串）。
func (b *Beatmap) SectionAt(beat float64) string {
	name := ""
	for i := range b.Sections {
		if b.Sections[i].Beat > beat {
			break
		}
		name = b.Sections[i].Name
	}
	return name
}

// normVolume 把 HS 编辑器的百分比音量（0~100）归一到 0~1。
func normVolume(v float64) float64 {
	if v > 1.5 {
		return v / 100
	}
	return v
}

// Prop 取关卡元数据字符串（如 remixtitle / remixauthor）。
func (b *Beatmap) Prop(key string) string {
	if v, ok := b.Properties[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Riq 是一次加载的完整结果。
type Riq struct {
	Beatmap     *Beatmap
	Audio       []byte
	AudioFormat AudioFormat
	AudioName   string // 容器内的音频文件名（诊断用）
}

// ---------- tempo map ----------

// normalizeTempos 保证 tempo map 可用：非空、升序、起点覆盖 beat 0。
func (b *Beatmap) normalizeTempos() {
	if len(b.Tempos) == 0 {
		b.Tempos = []TempoChange{{Beat: 0, BPM: 120}}
	}
	sort.Slice(b.Tempos, func(i, j int) bool { return b.Tempos[i].Beat < b.Tempos[j].Beat })
	sort.Slice(b.Volumes, func(i, j int) bool { return b.Volumes[i].Beat < b.Volumes[j].Beat })
	sort.Slice(b.Sections, func(i, j int) bool { return b.Sections[i].Beat < b.Sections[j].Beat })
	if b.Tempos[0].Beat > 0 {
		b.Tempos = append([]TempoChange{{Beat: 0, BPM: b.Tempos[0].BPM}}, b.Tempos...)
	}
}

// BeatToTime 把节拍映射到歌曲时间（秒），分段线性。
func (b *Beatmap) BeatToTime(beat float64) float64 {
	t := b.Offset
	ts := b.Tempos
	for i := range ts {
		spb := 60.0 / ts[i].BPM // seconds per beat
		if i+1 < len(ts) && beat >= ts[i+1].Beat {
			t += (ts[i+1].Beat - ts[i].Beat) * spb
			continue
		}
		return t + (beat-ts[i].Beat)*spb
	}
	return t
}

// TimeToBeat 是 BeatToTime 的逆映射。
func (b *Beatmap) TimeToBeat(t float64) float64 {
	cur := b.Offset
	ts := b.Tempos
	for i := range ts {
		spb := 60.0 / ts[i].BPM
		if i+1 < len(ts) {
			segDur := (ts[i+1].Beat - ts[i].Beat) * spb
			if t >= cur+segDur {
				cur += segDur
				continue
			}
		}
		return ts[i].Beat + (t-cur)/spb
	}
	return 0
}

// BPMAt 返回某节拍处生效的 BPM。
func (b *Beatmap) BPMAt(beat float64) float64 {
	bpm := b.Tempos[0].BPM
	for _, tc := range b.Tempos {
		if tc.Beat > beat {
			break
		}
		bpm = tc.BPM
	}
	return bpm
}

// ---------- 加载 ----------

// Load 打开一个 .riq 文件，自动识别 v1/v2 布局。
func Load(p string) (*Riq, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("open riq: %w", err)
	}
	return LoadBytes(b)
}

// LoadBytes 从内存加载 .riq（拖放导入用）。
func LoadBytes(b []byte) (*Riq, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("open riq: %w", err)
	}

	files := map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		files[f.Name] = data
	}

	if chart, ok := findChart(files); ok {
		return loadV2(files, chart)
	}
	if _, ok := files["remix.json"]; ok {
		return loadV1(files)
	}
	return nil, fmt.Errorf("unrecognized riq layout: neither Charts/*.json nor remix.json found")
}

func findChart(files map[string][]byte) (string, bool) {
	if _, ok := files["Charts/chart0.json"]; ok {
		return "Charts/chart0.json", true
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasPrefix(name, "Charts/") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	return names[0], true
}

// ---------- v2 ----------

type v2Chart struct {
	Version  int               `json:"version"`
	Offset   float64           `json:"offset"`
	SongName string            `json:"songname"`
	Entities []v2Entity        `json:"entities"`
	Models   map[string]string `json:"models"`
	Types    map[string]string `json:"types"`
}

type v2Entity struct {
	Type    int            `json:"type"`
	Version int            `json:"version"`
	Model   int            `json:"model"`
	Data    map[string]any `json:"data"`
}

func loadV2(files map[string][]byte, chartName string) (*Riq, error) {
	var c v2Chart
	if err := json.Unmarshal(files[chartName], &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", chartName, err)
	}

	models, err := indexTable(c.Models)
	if err != nil {
		return nil, fmt.Errorf("models table: %w", err)
	}
	types, err := indexTable(c.Types)
	if err != nil {
		return nil, fmt.Errorf("types table: %w", err)
	}

	bm := &Beatmap{Version: c.Version, Offset: c.Offset, SongName: c.SongName}
	if bm.SongName == "" {
		bm.SongName = "song0"
	}

	for _, raw := range c.Entities {
		if raw.Model < 0 || raw.Model >= len(models) || raw.Type < 0 || raw.Type >= len(types) {
			return nil, fmt.Errorf("entity references out-of-range model/type index (%d/%d)", raw.Model, raw.Type)
		}
		e := Entity{
			Datamodel: models[raw.Model],
			Type:      types[raw.Type],
			Data:      raw.Data,
		}
		e.Beat = e.Float("beat", 0)
		e.Length = e.Float("length", 0)

		switch e.Type {
		case "riq__TempoChange":
			bm.Tempos = append(bm.Tempos, TempoChange{Beat: e.Beat, BPM: e.Float("tempo", 120)})
		case "riq__VolumeChange":
			bm.Volumes = append(bm.Volumes, VolumeChange{
				Beat: e.Beat, Volume: normVolume(e.Float("volume", 1)), Length: e.Length,
			})
		case "riq__SectionMarker":
			name := e.Str("sectionName", "")
			if name == "" {
				name = e.Str("name", "")
			}
			bm.Sections = append(bm.Sections, SectionMarker{Beat: e.Beat, Name: name})
		default:
			bm.Entities = append(bm.Entities, e)
		}
	}
	bm.normalizeTempos()
	sort.SliceStable(bm.Entities, func(i, j int) bool { return bm.Entities[i].Beat < bm.Entities[j].Beat })

	audioName, audio, err := findV2Audio(files, bm.SongName)
	if err != nil {
		return nil, err
	}
	return &Riq{Beatmap: bm, Audio: audio, AudioFormat: Sniff(audio), AudioName: audioName}, nil
}

// indexTable 把 {"0": "a", "1": "b"} 形式的 JSON 对象还原为有序切片。
func indexTable(m map[string]string) ([]string, error) {
	out := make([]string, len(m))
	for k, v := range m {
		i, err := strconv.Atoi(k)
		if err != nil || i < 0 || i >= len(m) {
			return nil, fmt.Errorf("bad index key %q", k)
		}
		out[i] = v
	}
	return out, nil
}

func findV2Audio(files map[string][]byte, songName string) (string, []byte, error) {
	var fallback string
	names := make([]string, 0)
	for name := range files {
		if strings.HasPrefix(name, "Music/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		base := path.Base(name)
		stem := strings.TrimSuffix(base, path.Ext(base))
		if stem == songName {
			return name, files[name], nil
		}
		if fallback == "" {
			fallback = name
		}
	}
	if fallback != "" {
		return fallback, files[fallback], nil
	}
	return "", nil, fmt.Errorf("no audio found under Music/ (songname=%q)", songName)
}

// ---------- v1 ----------

type v1Chart struct {
	RiqVersion      string         `json:"riqVersion"`
	Offset          float64        `json:"offset"`
	Properties      map[string]any `json:"properties"`
	Entities        []v1Entity     `json:"entities"`
	TempoChanges    []v1Entity     `json:"tempoChanges"`
	VolumeChanges   []v1Entity     `json:"volumeChanges"`
	BeatmapSections []v1Entity     `json:"beatmapSections"`
}

type v1Entity struct {
	Datamodel   string         `json:"datamodel"`
	Beat        float64        `json:"beat"`
	Length      float64        `json:"length"`
	DynamicData map[string]any `json:"dynamicData"`
}

func loadV1(files map[string][]byte) (*Riq, error) {
	var c v1Chart
	// 官方导出的 remix.json 带 UTF-8 BOM（byte order mark），需剥除
	raw := bytes.TrimPrefix(files["remix.json"], []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse remix.json: %w", err)
	}

	bm := &Beatmap{Version: 1, Offset: c.Offset, SongName: "song", Properties: c.Properties}
	for _, raw := range c.Entities {
		bm.Entities = append(bm.Entities, Entity{
			Datamodel: raw.Datamodel,
			Type:      "riq__Entity",
			Beat:      raw.Beat,
			Length:    raw.Length,
			Data:      raw.DynamicData,
		})
	}
	for _, raw := range c.TempoChanges {
		e := Entity{Data: raw.DynamicData}
		bm.Tempos = append(bm.Tempos, TempoChange{Beat: raw.Beat, BPM: e.Float("tempo", 120)})
	}
	for _, raw := range c.VolumeChanges {
		e := Entity{Data: raw.DynamicData}
		bm.Volumes = append(bm.Volumes, VolumeChange{
			Beat: raw.Beat, Volume: normVolume(e.Float("volume", 1)), Length: raw.Length,
		})
	}
	for _, raw := range c.BeatmapSections {
		e := Entity{Data: raw.DynamicData}
		name := e.Str("sectionName", "")
		if name == "" {
			name = e.Str("name", "")
		}
		bm.Sections = append(bm.Sections, SectionMarker{Beat: raw.Beat, Name: name})
	}
	bm.normalizeTempos()
	sort.SliceStable(bm.Entities, func(i, j int) bool { return bm.Entities[i].Beat < bm.Entities[j].Beat })

	audio, ok := files["song.bin"]
	if !ok {
		return nil, fmt.Errorf("v1 riq missing song.bin")
	}
	return &Riq{Beatmap: bm, Audio: audio, AudioFormat: Sniff(audio), AudioName: "song.bin"}, nil
}

// Sniff 通过 magic bytes 判定音频容器类型。
func Sniff(b []byte) AudioFormat {
	if len(b) < 4 {
		return AudioUnknown
	}
	switch {
	case string(b[:4]) == "RIFF":
		return AudioWAV
	case string(b[:4]) == "OggS":
		return AudioOGG
	case string(b[:3]) == "ID3", b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return AudioMP3
	}
	return AudioUnknown
}
