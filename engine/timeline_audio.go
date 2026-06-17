package engine

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"hsdemo/riq"
)

func decodeMusicPlayer(r *riq.Riq) (*audio.Player, *pitchPCMReader, error) {
	pcm, err := decodeMusicPCM(r)
	if err != nil {
		return nil, nil, err
	}
	reader := newPitchPCMReader(pcm, 1)
	player, err := audioCtx.NewPlayer(reader)
	if err != nil {
		return nil, nil, err
	}
	player.SetVolume(0.85)
	return player, reader, nil
}

func decodeMusicPCM(r *riq.Riq) ([]byte, error) {
	stream, err := decodeMusicStream(r)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(stream)
}

func decodeMusicStream(r *riq.Riq) (io.Reader, error) {
	return decodeAudioStream(r.Audio, r.AudioFormat, r.AudioName)
}

func decodeEmbeddedAudioPCM(r *riq.Riq) map[string][]byte {
	out := map[string][]byte{}
	for name, snd := range r.CustomSfx {
		stream, err := decodeAudioStream(snd.Data, snd.Format, snd.Name)
		if err != nil {
			log.Printf("engine: 自定义音效 %s 解码失败: %v", snd.Name, err)
			continue
		}
		pcm, err := io.ReadAll(stream)
		if err != nil {
			log.Printf("engine: 自定义音效 %s 读取失败: %v", snd.Name, err)
			continue
		}
		out[name] = pcm
		out[strings.ToLower(name)] = pcm
	}
	return out
}

func decodeAudioStream(raw []byte, format riq.AudioFormat, name string) (io.Reader, error) {
	br := bytes.NewReader(raw)
	var (
		stream io.Reader
		err    error
	)
	switch format {
	case riq.AudioWAV:
		stream, err = wav.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioOGG:
		stream, err = vorbis.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioMP3:
		stream, err = mp3.DecodeWithSampleRate(SampleRate, br)
	default:
		return nil, fmt.Errorf("unsupported audio format (%s)", name)
	}
	if err != nil {
		return nil, fmt.Errorf("decode audio: %w", err)
	}
	return stream, nil
}
