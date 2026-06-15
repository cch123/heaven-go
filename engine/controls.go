package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func menuConfirmPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func titlePressed() bool {
	return pressed() ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func pressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// pressedN：动作通道 1=左（F/A/←）、2=右（K/D/→）、
// 3=下/替代（L/S/↓/X）、4=上（W/↑）。
func pressedN(action int) bool {
	switch action {
	case 1:
		return inpututil.IsKeyJustPressed(ebiten.KeyF) ||
			inpututil.IsKeyJustPressed(ebiten.KeyA) ||
			inpututil.IsKeyJustPressed(ebiten.KeyLeft)
	case 2:
		return inpututil.IsKeyJustPressed(ebiten.KeyK) ||
			inpututil.IsKeyJustPressed(ebiten.KeyD) ||
			inpututil.IsKeyJustPressed(ebiten.KeyRight)
	case 3:
		return inpututil.IsKeyJustPressed(ebiten.KeyL) ||
			inpututil.IsKeyJustPressed(ebiten.KeyS) ||
			inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
			inpututil.IsKeyJustPressed(ebiten.KeyX)
	case 4:
		return inpututil.IsKeyJustPressed(ebiten.KeyW) ||
			inpututil.IsKeyJustPressed(ebiten.KeyUp)
	}
	return false
}

func released() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeySpace) ||
		inpututil.IsKeyJustReleased(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}

func releasedN(action int) bool {
	switch action {
	case 1:
		return inpututil.IsKeyJustReleased(ebiten.KeyF) ||
			inpututil.IsKeyJustReleased(ebiten.KeyA) ||
			inpututil.IsKeyJustReleased(ebiten.KeyLeft)
	case 2:
		return inpututil.IsKeyJustReleased(ebiten.KeyK) ||
			inpututil.IsKeyJustReleased(ebiten.KeyD) ||
			inpututil.IsKeyJustReleased(ebiten.KeyRight)
	case 3:
		return inpututil.IsKeyJustReleased(ebiten.KeyL) ||
			inpututil.IsKeyJustReleased(ebiten.KeyS) ||
			inpututil.IsKeyJustReleased(ebiten.KeyDown) ||
			inpututil.IsKeyJustReleased(ebiten.KeyX)
	case 4:
		return inpututil.IsKeyJustReleased(ebiten.KeyW) ||
			inpututil.IsKeyJustReleased(ebiten.KeyUp)
	}
	return false
}
