package engine

type beatAction struct {
	beat float64
	fn   func()
}

// camEvt 是 vfx/move camera 事件（GameCamera.UpdateCameraTranslate 语义）。
type camEvt struct {
	beat, length float64
	target       [3]float64 // (valA, valB, -valC)
	ease         int
	axis         int // 0=All 1=X 2=Y 3=Z
}

type musicFadeEvt struct {
	beat, length float64
	from, to     float64
}

type flashEvt struct {
	beat, length float64
	c0, c1       [4]float64
}

type gameSwitch struct {
	beat float64
	id   string
}

// viewScaleEvt 是 vfx/scale view 事件（StaticCamera：整张游戏画布的缩放，
// 画布外露出 letterbox 黑场）。
type viewScaleEvt struct {
	beat, length float64
	x, y         float64
	ease         int
	axis         int // 0=All 1=X 2=Y
}
