package engine

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"hsdemo/riq"
)

func decodeMusicPlayer(r *riq.Riq) (*audio.Player, error) {
	stream, err := decodeMusicStream(r)
	if err != nil {
		return nil, err
	}
	player, err := audioCtx.NewPlayer(stream)
	if err != nil {
		return nil, err
	}
	player.SetVolume(0.85)
	return player, nil
}

func decodeMusicStream(r *riq.Riq) (io.Reader, error) {
	br := bytes.NewReader(r.Audio)
	var (
		stream io.Reader
		err    error
	)
	switch r.AudioFormat {
	case riq.AudioWAV:
		stream, err = wav.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioOGG:
		stream, err = vorbis.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioMP3:
		stream, err = mp3.DecodeWithSampleRate(SampleRate, br)
	default:
		return nil, fmt.Errorf("unsupported audio format (%s)", r.AudioName)
	}
	if err != nil {
		return nil, fmt.Errorf("decode music: %w", err)
	}
	return stream, nil
}
