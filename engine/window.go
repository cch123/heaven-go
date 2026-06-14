package engine

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// ConfigureWindow applies the shared desktop window policy for the current
// engine path and the legacy Karate Man path. The logical render size stays
// fixed at 16:9; Ebitengine scales it to the fullscreen monitor.
func ConfigureWindow(title string, fullscreen bool) {
	ebiten.SetWindowSize(ScreenW, ScreenH)
	ebiten.SetWindowTitle(title)
	// On macOS this keeps arbitrary resizing disabled while enabling the native
	// fullscreen button. On other desktop platforms fullscreen is still handled
	// by SetFullscreen and the keyboard shortcut below.
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeOnlyFullscreenEnabled)
	if fullscreen {
		ebiten.SetFullscreen(true)
	}
}

// HandleFullscreenShortcut toggles fullscreen from common desktop shortcuts.
// It returns true when it consumed the frame's input so Enter does not also
// start/skip gameplay on the same frame.
func HandleFullscreenShortcut() bool {
	enter := inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
	alt := ebiten.IsKeyPressed(ebiten.KeyAlt)
	meta := ebiten.IsKeyPressed(ebiten.KeyMeta)
	ctrlMetaF := inpututil.IsKeyJustPressed(ebiten.KeyF) &&
		ebiten.IsKeyPressed(ebiten.KeyControl) &&
		meta

	if inpututil.IsKeyJustPressed(ebiten.KeyF11) || (enter && (alt || meta)) || ctrlMetaF {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
		return true
	}
	return false
}
