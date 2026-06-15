// ctx.go：模块对引擎的访问句柄（HS 中 Minigame 基类提供的服务）。
package engine

import (
	"path/filepath"

	"hsdemo/kart"
)

// Ctx 是模块与引擎之间的接口：资产、场景、时间轴调度与输入注册。
type Ctx struct {
	App    *App
	Assets *kart.Assets
	Scene  *kart.SceneInst

	module Module
}

// LoadAssets 加载模块资产目录（assetsRoot/<id>）并创建场景实例。
func (c *Ctx) LoadAssets(id string) error {
	as, err := kart.Load(filepath.Join(c.App.assetsRoot, id), SampleRate)
	if err != nil {
		return err
	}
	c.Assets = as
	c.Scene = kart.NewScene(as)
	return nil
}

// AssetsRoot returns the app's configured assets directory. Most modules only
// need their own extracted bundle via LoadAssets; games that share common
// runtime resources, such as the Heaven Studio textbox font, use this to avoid
// hard-coding "assets/".
func (c *Ctx) AssetsRoot() string { return c.App.assetsRoot }

// Role 取脚本字段绑定的节点 path（roles.json）。
func (c *Ctx) Role(field string) string { return c.Assets.Roles[field] }

// Play 在子树上播放剪辑（DoScaledAnimationAsync 语义）。
func (c *Ctx) Play(rolePath, clip string, startBeat, timeScale float64) {
	c.Scene.Play(rolePath, clip, startBeat, timeScale)
}

// At 在指定拍执行回调（BeatAction 等价物），载入期与运行期均可调用。
func (c *Ctx) At(beat float64, fn func()) { c.App.at(beat, fn) }

// AwardSkillStar 复刻脚本侧直接调用 SkillStarManager.StarJust 的事件。
// 常规 skill star 仍由 gameManager/skill star + 判定窗口处理；少数游戏
// 例如 Quiz Show 在自己的成功条件满足后显式发星。
func (c *Ctx) AwardSkillStar() {
	if c.App != nil && !c.App.starGot {
		c.App.starGot = true
	}
}
