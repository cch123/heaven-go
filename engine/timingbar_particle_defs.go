package engine

import "image/color"

type timingParticleMaterial int

const (
	timingMaterialStar timingParticleMaterial = iota
	timingMaterialAce
)

type timingParticleTexture int

const (
	timingTextureMain timingParticleTexture = iota
	timingTextureCircle
	timingTextureStar
)

type timingCurveKey struct {
	t, v float64
}

type timingParticleSystem struct {
	name            string
	seedSalt        int
	lifetime        float64
	burstTime       float64
	startSpeed      float64
	startSize       float64
	shapeRadius     float64
	count           int
	randomRotation  bool
	angularVelocity float64
	minColor        color.RGBA
	maxColor        color.RGBA
	randomColor     bool
	material        timingParticleMaterial
	texture         timingParticleTexture
	sizeCurve       []timingCurveKey
}

var (
	// Serialized from TimingAccuracy.prefab's Just/OK/NG ParticleSystems.
	// Keeping the prefab names here makes future audits against Unity YAML direct.
	timingParticleJust = []timingParticleSystem{
		{
			name: "Just00", seedSalt: 11,
			lifetime: 0.45, startSpeed: 5, startSize: 0.7, shapeRadius: 0.1, count: 10, randomRotation: true,
			angularVelocity: 0.43633232,
			minColor:        color.RGBA{255, 0, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255}, randomColor: true,
			material: timingMaterialAce, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {0.40679932, 1}, {0.9, 0.5}},
		},
		{
			name: "JustSub", seedSalt: 12,
			lifetime: 0.3, burstTime: 0.45, startSize: 0.6, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255},
			material: timingMaterialAce, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
	timingParticleOK = []timingParticleSystem{
		{
			name: "Just01", seedSalt: 21,
			lifetime: 0.4, startSpeed: 4, startSize: 0.7, shapeRadius: 0.25, count: 10, randomRotation: true,
			angularVelocity: 8.807386,
			minColor:        color.RGBA{255, 0, 255, 255}, maxColor: color.RGBA{0, 255, 255, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {0.5, 1}, {0.9, 0.5}},
		},
		{
			name: "JustSub", seedSalt: 22,
			lifetime: 0.4, burstTime: 0.025, startSize: 0.45, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255},
			material: timingMaterialStar, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
	timingParticleNG = []timingParticleSystem{
		{
			name: "Miss00", seedSalt: 31,
			lifetime: 0.2, startSize: 3, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 0, 0, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureCircle,
			sizeCurve: []timingCurveKey{{0, 0}, {1, 1}},
		},
		{
			name: "MissStar", seedSalt: 32,
			lifetime: 0.2, startSize: 2, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 0, 0, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureStar,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
)

func timingParticleSystems(j Judgment) []timingParticleSystem {
	switch j {
	case JudgeAce:
		return timingParticleJust
	case JudgeJust:
		return timingParticleOK
	default:
		return timingParticleNG
	}
}
