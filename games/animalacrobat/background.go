package animalacrobat

import "math"

func (m *Module) initBGTiles() {
	if m.bgTileA == "" || m.bgTileB == "" || m.ctx == nil || m.ctx.Assets == nil {
		return
	}
	ax, ay := nodePos(m.ctx.Assets, m.bgTileA)
	bx, by := nodePos(m.ctx.Assets, m.bgTileB)
	dist := bx - ax
	if dist <= 0 {
		return
	}
	m.bgTiles = bgTileRuntime{
		firstBase:    [2]float64{ax, ay},
		secondBase:   [2]float64{bx, by},
		tileDistance: dist,
		ok:           true,
	}
}

func (m *Module) updateBGTiles() {
	if !m.bgTiles.ok {
		return
	}
	first, second := bgTilePositions(m.cameraX, m.bgTiles)
	m.ctx.Scene.SetPosOver(m.bgTileA, first[0], first[1])
	m.ctx.Scene.SetPosOver(m.bgTileB, second[0], second[1])
}

func bgTilePositions(cameraX float64, rt bgTileRuntime) ([2]float64, [2]float64) {
	first, second := rt.firstBase, rt.secondBase
	if !rt.ok || rt.tileDistance <= 0 {
		return first, second
	}

	// Unity BGTileManager starts with reachTileDistance == tileDistance and
	// alternates moving the first/second tile forward by two tile widths every
	// time GameCamera.AdditionalPosition.x crosses the next threshold. Computing
	// the crossing count directly keeps seeks deterministic instead of replaying
	// LateUpdate steps from beat zero.
	crossings := int(math.Floor(cameraX / rt.tileDistance))
	if crossings <= 0 {
		return first, second
	}
	first[0] += rt.tileDistance * 2 * float64((crossings+1)/2)
	second[0] += rt.tileDistance * 2 * float64(crossings/2)
	return first, second
}
