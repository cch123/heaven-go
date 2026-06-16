package main

import (
	"testing"
)

func TestCollectParticleSystemsExportsCoreModules(t *testing.T) {
	const (
		spriteGUID = "sprite-guid"
		matGUID    = "mat-guid"
	)
	dt := &docTable{byID: map[int64]*docRef{
		1: {classID: 1, content: map[string]any{"m_IsActive": float64(1)}},
		198: {classID: 198, content: map[string]any{
			"m_GameObject":        map[string]any{"fileID": float64(1)},
			"lengthInSec":         float64(0.5),
			"simulationSpeed":     float64(2),
			"looping":             float64(1),
			"prewarm":             float64(1),
			"playOnAwake":         float64(0),
			"stopAction":          float64(2),
			"cullingMode":         float64(1),
			"scalingMode":         float64(1),
			"emitterVelocityMode": float64(0),
			"startDelay":          particleTestCurve(0, 0),
			"InitialModule":       particleTestInitial(),
			"ShapeModule":         particleTestShape(),
			"EmissionModule":      particleTestEmission(),
			"SizeModule":          map[string]any{"enabled": float64(1), "curve": particleTestCurve(1, 0)},
			"RotationModule":      map[string]any{"enabled": float64(0), "curve": particleTestCurve(0, 0)},
			"ColorModule":         map[string]any{"enabled": float64(1), "gradient": particleTestGradient()},
			"VelocityModule":      map[string]any{"enabled": float64(1), "x": particleTestCurve(3, 1), "y": particleTestCurve(4, 2)},
			"ForceModule":         map[string]any{"enabled": float64(1), "x": particleTestCurve(5, 3)},
			"UVModule":            particleTestUV(spriteGUID),
		}},
		199: {classID: 199, content: map[string]any{
			"m_GameObject":                 map[string]any{"fileID": float64(1)},
			"m_Enabled":                    float64(1),
			"m_Materials":                  []any{map[string]any{"fileID": float64(2100000), "guid": matGUID}},
			"m_SortingLayer":               float64(2),
			"m_SortingOrder":               float64(25),
			"m_RenderMode":                 float64(0),
			"m_SortMode":                   float64(1),
			"m_RenderAlignment":            float64(4),
			"m_MinParticleSize":            float64(0),
			"m_MaxParticleSize":            float64(0.5),
			"m_CameraVelocityScale":        float64(0.1),
			"m_VelocityScale":              float64(0.2),
			"m_LengthScale":                float64(2),
			"m_SortingFudge":               float64(0.3),
			"m_NormalDirection":            float64(1),
			"m_Pivot":                      map[string]any{"x": float64(1), "y": float64(2), "z": float64(3)},
			"m_Flip":                       map[string]any{"x": float64(0), "y": float64(1), "z": float64(0)},
			"m_AllowRoll":                  float64(1),
			"m_FreeformStretching":         float64(1),
			"m_RotateWithStretchDirection": float64(1),
		}},
	}}
	tables := map[string]*spriteTable{
		spriteGUID: {byID: map[int64]string{99: "spark"}},
	}
	systems := collectParticleSystems(dt, map[int64]string{1: "FX/Spark"}, tables, assetIndex{
		refs: kmdataRef(matGUID, "particle_mat", "Materials/particle_mat.mat"),
	}, assetIndex{})
	if len(systems) != 1 {
		t.Fatalf("systems = %d, want 1", len(systems))
	}
	got := systems[0]
	if got.Path != "FX/Spark" || !got.Active || !got.Enabled || !got.Looping || !got.Prewarm {
		t.Fatalf("system identity wrong: %#v", got)
	}
	if got.Duration != 0.5 || got.SimulationSpeed != 2 || got.StopAction != 2 {
		t.Fatalf("system timing wrong: %#v", got)
	}
	if got.StartLifetime.Mode != 3 || got.StartLifetime.Scalar != 0.2 || got.StartLifetime.MinScalar != 0.1 {
		t.Fatalf("startLifetime curve wrong: %#v", got.StartLifetime)
	}
	if len(got.Emission.Bursts) != 1 || got.Emission.Bursts[0].Count.Scalar != 7 {
		t.Fatalf("emission burst wrong: %#v", got.Emission)
	}
	if got.Shape.Type != 10 || got.Shape.Angle != 25 || got.Shape.Scale != [3]float64{1, 2, 3} {
		t.Fatalf("shape wrong: %#v", got.Shape)
	}
	if len(got.TextureSheet.Sprites) != 1 || got.TextureSheet.Sprites[0] != "spark" || got.TextureSheet.Tiles != [2]int{2, 3} {
		t.Fatalf("texture sheet wrong: %#v", got.TextureSheet)
	}
	if got.Renderer.SortingOrder != 25 || got.Renderer.Materials[0].Name != "particle_mat" || !got.Renderer.RotateWithStretchDirection {
		t.Fatalf("renderer wrong: %#v", got.Renderer)
	}
}

func particleTestInitial() map[string]any {
	return map[string]any{
		"enabled":         float64(1),
		"startLifetime":   particleTestCurve(0.2, 0.1),
		"startSpeed":      particleTestCurve(5, 1),
		"startSize":       particleTestCurve(0.4, 0.2),
		"startRotation":   particleTestCurve(1, 0),
		"startColor":      particleTestGradient(),
		"gravityModifier": particleTestCurve(0.5, 0),
		"maxNumParticles": float64(30),
	}
}

func particleTestShape() map[string]any {
	return map[string]any{
		"enabled":         float64(1),
		"type":            float64(10),
		"angle":           float64(25),
		"radius":          float64(1.5),
		"radiusThickness": float64(0.1),
		"arc":             float64(180),
		"length":          float64(5),
		"m_Position":      map[string]any{"x": float64(1), "y": float64(0), "z": float64(0)},
		"m_Rotation":      map[string]any{"x": float64(0), "y": float64(0), "z": float64(45)},
		"m_Scale":         map[string]any{"x": float64(1), "y": float64(2), "z": float64(3)},
	}
}

func particleTestEmission() map[string]any {
	return map[string]any{
		"enabled":          float64(1),
		"rateOverTime":     particleTestCurve(0, 0),
		"rateOverDistance": particleTestCurve(0, 0),
		"m_Bursts": []any{map[string]any{
			"time":           float64(0),
			"countCurve":     particleTestCurve(7, 0),
			"cycleCount":     float64(1),
			"repeatInterval": float64(0.01),
			"probability":    float64(1),
		}},
	}
}

func particleTestUV(spriteGUID string) map[string]any {
	return map[string]any{
		"enabled":       float64(1),
		"animationType": float64(0),
		"tilesX":        float64(2),
		"tilesY":        float64(3),
		"cycles":        float64(4),
		"sprites": []any{map[string]any{
			"sprite": map[string]any{"fileID": float64(99), "guid": spriteGUID},
		}},
	}
}

func particleTestCurve(scalar, minScalar float64) map[string]any {
	return map[string]any{
		"minMaxState": float64(3),
		"scalar":      scalar,
		"minScalar":   minScalar,
		"maxCurve": map[string]any{"m_Curve": []any{
			map[string]any{"time": float64(0), "value": float64(1), "inSlope": float64(0), "outSlope": float64(0)},
			map[string]any{"time": float64(1), "value": float64(1), "inSlope": "Infinity", "outSlope": "-Infinity"},
		}},
		"minCurve": map[string]any{"m_Curve": []any{
			map[string]any{"time": float64(0), "value": float64(0), "inSlope": float64(0), "outSlope": float64(0)},
			map[string]any{"time": float64(1), "value": float64(0), "inSlope": float64(0), "outSlope": float64(0)},
		}},
	}
}

func particleTestGradient() map[string]any {
	grad := map[string]any{
		"m_Mode":         float64(0),
		"m_NumColorKeys": float64(2),
		"m_NumAlphaKeys": float64(2),
		"key0":           map[string]any{"r": float64(1), "g": float64(0), "b": float64(0), "a": float64(1)},
		"key1":           map[string]any{"r": float64(0), "g": float64(0), "b": float64(1), "a": float64(0.5)},
		"ctime0":         float64(0),
		"ctime1":         float64(65535),
		"atime0":         float64(0),
		"atime1":         float64(65535),
	}
	return map[string]any{
		"minMaxState": float64(0),
		"minColor":    map[string]any{"r": float64(1), "g": float64(1), "b": float64(1), "a": float64(1)},
		"maxColor":    map[string]any{"r": float64(1), "g": float64(0), "b": float64(0), "a": float64(1)},
		"maxGradient": grad,
		"minGradient": grad,
	}
}
