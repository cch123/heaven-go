package main

import (
	"fmt"
	"sort"

	"hsdemo/kmdata"
	uy "hsdemo/unityyaml"
)

func exportParticles(spec sceneSpec, dt *docTable, paths map[int64]string, tables map[string]*spriteTable) {
	materialAssets := scanAssetIndex(spec.gameRoot(), ".mat")
	meshAssets := scanAssetIndex(spec.gameRoot(), ".fbx", ".obj", ".asset")
	data := kmdata.ParticleData{
		Systems: collectParticleSystems(dt, paths, tables, materialAssets, meshAssets),
	}
	if len(data.Systems) == 0 {
		return
	}
	writeJSON("particles.json", data)
	fmt.Printf("particles: %d systems\n", len(data.Systems))
}

func collectParticleSystems(dt *docTable, paths map[int64]string, tables map[string]*spriteTable, materialAssets, meshAssets assetIndex) []kmdata.ParticleSystem {
	systems := map[int64]map[string]any{}
	renderers := map[int64]map[string]any{}
	active := map[int64]bool{}
	for gid := range paths {
		active[gid] = true
	}
	for id, d := range dt.byID {
		gid := uy.I(uy.Get(d.content, "m_GameObject", "fileID"))
		switch d.classID {
		case 1:
			active[id] = uy.I(d.content["m_IsActive"]) != 0
		case 198: // ParticleSystem
			systems[gid] = d.content
		case 199: // ParticleSystemRenderer
			renderers[gid] = d.content
		}
	}

	ids := make([]int64, 0, len(systems))
	for gid := range systems {
		if _, ok := paths[gid]; ok {
			ids = append(ids, gid)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		pi, pj := paths[ids[i]], paths[ids[j]]
		if pi != pj {
			return pi < pj
		}
		return ids[i] < ids[j]
	})

	out := make([]kmdata.ParticleSystem, 0, len(ids))
	seen := map[string]bool{}
	for _, gid := range ids {
		path := paths[gid]
		if seen[path] {
			continue
		}
		seen[path] = true
		sys := systems[gid]
		out = append(out, particleSystemFromYAML(path, active[gid], sys, renderers[gid], tables, materialAssets, meshAssets))
	}
	return out
}

func particleSystemFromYAML(path string, active bool, sys, rend map[string]any, tables map[string]*spriteTable, materialAssets, meshAssets assetIndex) kmdata.ParticleSystem {
	initial := uy.M(sys["InitialModule"])
	return kmdata.ParticleSystem{
		Path:                 path,
		Enabled:              particleEnabled(initial),
		Active:               active,
		Duration:             uy.F(sys["lengthInSec"]),
		SimulationSpeed:      uy.F(sys["simulationSpeed"]),
		Looping:              uy.I(sys["looping"]) != 0,
		Prewarm:              uy.I(sys["prewarm"]) != 0,
		PlayOnAwake:          uy.I(sys["playOnAwake"]) != 0,
		StopAction:           int(uy.I(sys["stopAction"])),
		CullingMode:          int(uy.I(sys["cullingMode"])),
		ScalingMode:          int(uy.I(sys["scalingMode"])),
		EmitterVelocityMode:  int(uy.I(sys["emitterVelocityMode"])),
		StartDelay:           particleCurve(sys["startDelay"]),
		StartLifetime:        particleCurve(initial["startLifetime"]),
		StartSpeed:           particleCurve(initial["startSpeed"]),
		StartSize:            particleCurve(initial["startSize"]),
		StartSizeY:           particleCurve(initial["startSizeY"]),
		StartSizeZ:           particleCurve(initial["startSizeZ"]),
		StartRotation:        particleCurve(initial["startRotation"]),
		StartRotationX:       particleCurve(initial["startRotationX"]),
		StartRotationY:       particleCurve(initial["startRotationY"]),
		StartColor:           particleGradient(initial["startColor"]),
		GravityModifier:      particleCurve(initial["gravityModifier"]),
		MaxParticles:         int(uy.I(initial["maxNumParticles"])),
		Shape:                particleShape(uy.M(sys["ShapeModule"])),
		Emission:             particleEmission(uy.M(sys["EmissionModule"])),
		SizeOverLifetime:     particleAxis(uy.M(sys["SizeModule"])),
		RotationOverLifetime: particleAxis(uy.M(sys["RotationModule"])),
		ColorOverLifetime:    particleColorMod(uy.M(sys["ColorModule"])),
		VelocityOverLifetime: particleAxis(uy.M(sys["VelocityModule"])),
		ForceOverLifetime:    particleAxis(uy.M(sys["ForceModule"])),
		TextureSheet:         particleSheet(uy.M(sys["UVModule"]), tables),
		Renderer:             particleRenderer(rend, materialAssets, meshAssets),
	}
}

func particleEnabled(m map[string]any) bool {
	return m == nil || uy.I(m["enabled"]) != 0
}

func particleCurve(v any) kmdata.ParticleCurve {
	m := uy.M(v)
	if m == nil {
		return kmdata.ParticleCurve{}
	}
	return kmdata.ParticleCurve{
		Mode:      int(uy.I(m["minMaxState"])),
		Scalar:    uy.F(m["scalar"]),
		MinScalar: uy.F(m["minScalar"]),
		Max:       particleCurveKeys(uy.M(m["maxCurve"])),
		Min:       particleCurveKeys(uy.M(m["minCurve"])),
	}
}

func particleCurveKeys(curve map[string]any) []kmdata.Key {
	if curve == nil {
		return nil
	}
	keys := uy.L(curve["m_Curve"])
	out := make([]kmdata.Key, 0, len(keys))
	for _, kv := range keys {
		k := uy.M(kv)
		out = append(out, key(uy.F(k["time"]), k["value"], k["inSlope"], k["outSlope"]))
	}
	return out
}

func particleGradient(v any) kmdata.ParticleGradient {
	m := uy.M(v)
	if m == nil {
		return kmdata.ParticleGradient{}
	}
	return kmdata.ParticleGradient{
		Mode:        int(uy.I(m["minMaxState"])),
		MinColor:    particleColor(m["minColor"]),
		MaxColor:    particleColor(m["maxColor"]),
		MinGradient: particleGradKeys(uy.M(m["minGradient"])),
		MaxGradient: particleGradKeys(uy.M(m["maxGradient"])),
	}
}

func particleGradKeys(m map[string]any) kmdata.ParticleGradKeys {
	if m == nil {
		return kmdata.ParticleGradKeys{}
	}
	numColors := int(uy.I(m["m_NumColorKeys"]))
	numAlphas := int(uy.I(m["m_NumAlphaKeys"]))
	out := kmdata.ParticleGradKeys{Mode: int(uy.I(m["m_Mode"]))}
	for i := 0; i < numColors && i < 8; i++ {
		out.ColorKeys = append(out.ColorKeys, kmdata.ParticleGradientColor{
			T:     uy.F(m["ctime"+itoa(i)]) / 65535,
			Color: particleColor(m["key"+itoa(i)]),
		})
	}
	for i := 0; i < numAlphas && i < 8; i++ {
		c := particleColor(m["key"+itoa(i)])
		out.AlphaKeys = append(out.AlphaKeys, kmdata.ParticleGradientAlpha{
			T: uy.F(m["atime"+itoa(i)]) / 65535,
			A: c[3],
		})
	}
	return out
}

func particleColor(v any) [4]float64 {
	m := uy.M(v)
	return [4]float64{uy.F(m["r"]), uy.F(m["g"]), uy.F(m["b"]), uy.F(m["a"])}
}

func particleShape(m map[string]any) kmdata.ParticleShape {
	if m == nil {
		return kmdata.ParticleShape{}
	}
	return kmdata.ParticleShape{
		Enabled:         particleEnabled(m),
		Type:            int(uy.I(m["type"])),
		Angle:           uy.F(m["angle"]),
		Radius:          uy.F(m["radius"]),
		RadiusThickness: uy.F(m["radiusThickness"]),
		Arc:             uy.F(m["arc"]),
		Length:          uy.F(m["length"]),
		Position:        particleVec3(m["m_Position"]),
		Rotation:        particleVec3(m["m_Rotation"]),
		Scale:           particleVec3(m["m_Scale"]),
	}
}

func particleEmission(m map[string]any) kmdata.ParticleEmission {
	if m == nil {
		return kmdata.ParticleEmission{}
	}
	out := kmdata.ParticleEmission{
		Enabled:          particleEnabled(m),
		RateOverTime:     particleCurve(m["rateOverTime"]),
		RateOverDistance: particleCurve(m["rateOverDistance"]),
	}
	for _, bv := range uy.L(m["m_Bursts"]) {
		b := uy.M(bv)
		out.Bursts = append(out.Bursts, kmdata.ParticleBurst{
			Time:           uy.F(b["time"]),
			Count:          particleCurve(b["countCurve"]),
			CycleCount:     int(uy.I(b["cycleCount"])),
			RepeatInterval: uy.F(b["repeatInterval"]),
			Probability:    uy.F(b["probability"]),
		})
	}
	return out
}

func particleAxis(m map[string]any) kmdata.ParticleAxis {
	if m == nil {
		return kmdata.ParticleAxis{}
	}
	return kmdata.ParticleAxis{
		Enabled: particleEnabled(m),
		X:       particleCurve(m["x"]),
		Y:       particleCurve(m["y"]),
		Z:       particleCurve(m["z"]),
		Curve:   particleCurve(m["curve"]),
	}
}

func particleColorMod(m map[string]any) kmdata.ParticleColorMod {
	if m == nil {
		return kmdata.ParticleColorMod{}
	}
	return kmdata.ParticleColorMod{
		Enabled: particleEnabled(m),
		Color:   particleGradient(m["gradient"]),
	}
}

func particleSheet(m map[string]any, tables map[string]*spriteTable) kmdata.ParticleSheet {
	if m == nil {
		return kmdata.ParticleSheet{}
	}
	mode := int(uy.I(m["mode"]))
	if _, ok := m["animationType"]; ok {
		mode = int(uy.I(m["animationType"]))
	}
	tx, ty := int(uy.I(uy.Get(m, "tiles", "x"))), int(uy.I(uy.Get(m, "tiles", "y")))
	if _, ok := m["tilesX"]; ok {
		tx, ty = int(uy.I(m["tilesX"])), int(uy.I(m["tilesY"]))
	}
	out := kmdata.ParticleSheet{
		Enabled: particleEnabled(m),
		Mode:    mode,
		Tiles:   [2]int{tx, ty},
		Cycles:  int(uy.I(m["cycles"])),
	}
	for _, sv := range uy.L(m["sprites"]) {
		s := uy.M(sv)
		if nested := uy.M(s["sprite"]); nested != nil {
			s = nested
		}
		if name := resolveSprite(tables, uy.S(s["guid"]), uy.I(s["fileID"])); name != "" {
			out.Sprites = append(out.Sprites, name)
		}
	}
	return out
}

func particleRenderer(m map[string]any, materialAssets, meshAssets assetIndex) kmdata.ParticleRenderer {
	if m == nil {
		return kmdata.ParticleRenderer{}
	}
	return kmdata.ParticleRenderer{
		Enabled:                    uy.I(m["m_Enabled"]) != 0,
		Materials:                  materialRefs(m, materialAssets),
		Mesh:                       assetRefFromYAML(uy.M(m["m_Mesh"]), meshAssets),
		SortingLayer:               int(uy.I(m["m_SortingLayer"])),
		SortingOrder:               int(uy.I(m["m_SortingOrder"])),
		RenderMode:                 int(uy.I(m["m_RenderMode"])),
		SortMode:                   int(uy.I(m["m_SortMode"])),
		RenderAlignment:            int(uy.I(m["m_RenderAlignment"])),
		MinParticleSize:            uy.F(m["m_MinParticleSize"]),
		MaxParticleSize:            uy.F(m["m_MaxParticleSize"]),
		CameraVelocityScale:        uy.F(m["m_CameraVelocityScale"]),
		VelocityScale:              uy.F(m["m_VelocityScale"]),
		LengthScale:                uy.F(m["m_LengthScale"]),
		SortingFudge:               uy.F(m["m_SortingFudge"]),
		NormalDirection:            uy.F(m["m_NormalDirection"]),
		Pivot:                      particleVec3(m["m_Pivot"]),
		Flip:                       particleVec3(m["m_Flip"]),
		AllowRoll:                  uy.I(m["m_AllowRoll"]) != 0,
		FreeformStretching:         uy.I(m["m_FreeformStretching"]) != 0,
		RotateWithStretchDirection: uy.I(m["m_RotateWithStretchDirection"]) != 0,
	}
}

func particleVec3(v any) [3]float64 {
	m := uy.M(v)
	return [3]float64{uy.F(m["x"]), uy.F(m["y"]), uy.F(m["z"])}
}

func itoa(i int) string {
	return string(rune('0' + i))
}
