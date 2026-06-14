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

// pressedN：动作通道 1=左（F/←/↑）、2=右（K/→）、3=替代（L/↓/X）。
func pressedN(action int) bool {
	switch action {
	case 1:
		return inpututil.IsKeyJustPressed(ebiten.KeyF) ||
			inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
			inpututil.IsKeyJustPressed(ebiten.KeyUp)
	case 2:
		return inpututil.IsKeyJustPressed(ebiten.KeyK) ||
			inpututil.IsKeyJustPressed(ebiten.KeyRight)
	case 3:
		return inpututil.IsKeyJustPressed(ebiten.KeyL) ||
			inpututil.IsKeyJustPressed(ebiten.KeyDown) ||
			inpututil.IsKeyJustPressed(ebiten.KeyX)
	}
	return false
}

func released() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeySpace) ||
		inpututil.IsKeyJustReleased(ebiten.KeyJ) ||
		inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}
