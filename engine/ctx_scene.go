package engine

// CameraAt 返回 beat 时刻相机世界位置（vfx/move camera 时间轴，默认 (0,0,-10)）。
func (c *Ctx) CameraAt(beat float64) [3]float64 { return c.App.CameraAt(beat) }

// FadeMusicVolume fades the chart music multiplier for minigame-specific
// ducking (HS Conductor.FadeMinigameVolume).
func (c *Ctx) FadeMusicVolume(beat, length, target float64) {
	c.App.fadeMusicVolume(beat, length, target)
}

// SampleScene applies the global GameCamera timeline before sampling the scene.
// Remix charts drive cross-game zooms/pans through vfx/move camera, so every
// scene-backed minigame must go through this helper instead of sampling with the
// prefab default camera.
func (c *Ctx) SampleScene(beat float64) [3]float64 {
	return c.SampleSceneZ(beat, 0)
}

// SampleSceneZ is SampleScene with an additional Z offset for game-local punch
// zooms (for example Lockstep and Cheer Readers).
func (c *Ctx) SampleSceneZ(beat, addZ float64) [3]float64 {
	cam := c.App.CameraAt(beat)
	if c.Scene != nil {
		c.Scene.SetCamera(cam[0], cam[1], cam[2]+addZ)
		c.Scene.Sample(beat)
	}
	return cam
}
