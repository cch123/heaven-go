package lovelab

import "math"

var shaderDay = [3][3][4]float64{
	{
		{0.8470589, 0.9725491, 0.8784314, 1},
		{0.01176471, 0.7686275, 0.2431373, 1},
		{0.9686275, 0.7529413, 0.654902, 1},
	},
	{
		{1, 1, 1, 1},
		{0.3921569, 0.4039216, 0.3921569, 1},
		{0.9686275, 0.7254902, 0.5686275, 1},
	},
	{
		{1, 1, 1, 1},
		{0.754717, 0.5786163, 0, 1},
		{0.9686275, 0.3803922, 0.03137255, 1},
	},
}

var shaderSunset = [3][3][4]float64{
	{
		{1, 0.937255, 0.3529412, 1},
		{0.01176471, 0.7686275, 0.2431373, 1},
		{0.9960785, 0.6588235, 0.5176471, 1},
	},
	{
		{1, 0.8392158, 0.4196079, 1},
		{0.6862745, 0.4078432, 0.4196079, 1},
		{0.8901961, 0.4627451, 0.2039216, 1},
	},
	{
		{0.9725491, 0.7843138, 0.4078432, 1},
		{0.972549, 0.7529412, 0.03137255, 1},
		{0.8941177, 0.1882353, 0, 1},
	},
}

func (m *Module) setObjectColors(a, b, c [4]float64) {
	m.boyLiquid, m.girlLiquid, m.weirdLiquid = a, b, c
	m.ctx.Scene.SetPaletteFor("Flask", flaskPalette(a))
	m.ctx.Scene.SetPaletteFor("GirlFlask", flaskPalette(b))
	m.ctx.Scene.SetPaletteFor("WeirdFlask", flaskPalette(c))
	m.ctx.Scene.SetPaletteOver(m.flaskSprite, flaskPalette(a))
	m.ctx.Scene.SetPaletteOver(m.girlFlaskSprite, flaskPalette(b))
	m.ctx.Scene.SetPaletteOver(m.weirdFlaskSprite, flaskPalette(c))
}

func (m *Module) setTimeOfDay(tod int) {
	if tod == timeDay {
		m.isDay = true
		m.ctx.Scene.SetActive(m.sunsetBG, false)
		m.ctx.Scene.SetActive(m.dayBG, true)
		m.ctx.Scene.SetActive(m.girlHeader, false)
		m.ctx.Scene.SetActive(m.boyHeader, false)
		m.ctx.Scene.SetActive(m.weirdHeader, false)
		m.applyShaderPalettes(shaderDay)
		return
	}
	m.isDay = false
	m.ctx.Scene.SetActive(m.sunsetBG, true)
	m.ctx.Scene.SetActive(m.dayBG, false)
	m.ctx.Scene.SetActive(m.girlHeader, true)
	m.ctx.Scene.SetActive(m.boyHeader, true)
	m.ctx.Scene.SetActive(m.weirdHeader, true)
	m.applyShaderPalettes(shaderSunset)
}

func (m *Module) applyShaderPalettes(cols [3][3][4]float64) {
	m.ctx.Scene.SetPaletteFor("GirlShading", characterPalette(cols[0][0], cols[0][1], cols[0][2]))
	m.ctx.Scene.SetPaletteFor("GuyShading", characterPalette(cols[1][0], cols[1][1], cols[1][2]))
	m.ctx.Scene.SetPaletteFor("WeirdShading", characterPalette(cols[2][0], cols[2][1], cols[2][2]))
}

func (m *Module) setSpotlight(active bool, typ, where int) {
	if !active {
		m.ctx.Scene.SetActive(m.spotlight, false)
		m.ctx.Scene.SetActive(m.spotConeRoot, false)
		return
	}
	if typ == spotCone {
		m.ctx.Scene.SetActive(m.spotlight, false)
		m.ctx.Scene.SetActive(m.spotConeRoot, true)
		if where == spotGirl {
			m.ctx.Scene.SetPosOver(m.spotCone, 5, 0)
		} else {
			m.ctx.Scene.SetPosOver(m.spotCone, 0, 0)
		}
		return
	}
	m.ctx.Scene.SetActive(m.spotlight, true)
	m.ctx.Scene.SetActive(m.spotConeRoot, false)
}

func (m *Module) updateClouds() {
	if m.clouds == "" {
		return
	}
	if !m.canCloudsMove {
		m.ctx.Scene.SetPosOver(m.clouds, 0, 0)
		return
	}
	dist := math.Max(m.cloudDistance, 0.001)
	x := -math.Mod(m.ctx.Time()*m.cloudSpeed, dist)
	m.ctx.Scene.SetPosOver(m.clouds, x, 0)
}
