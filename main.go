package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
)

func main() {
	path := flag.String("riq", "", ".riq 谱面路径（留空则进入标题屏等待拖放）")
	assetsRoot := flag.String("assets", "assets", "提取资产根目录")
	latency := flag.Float64("latency", 0, "输入延迟校准（毫秒，可在游戏内用 [ ] 微调）")
	autoplay := flag.Bool("autoplay", false, "完美自动打击（调试/验证用）")
	fullscreen := flag.Bool("fullscreen", false, "启动时进入全屏；运行中可用 F11 / Alt+Enter 切换")
	flag.Parse()

	registerGames()

	app, err := engine.New(*assetsRoot, *path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	app.LatencyMS = *latency
	app.Autoplay = *autoplay

	engine.ConfigureWindow("Heaven Go", *fullscreen)
	// 提高逻辑帧率，把输入采样量化误差压到 ~±2ms（60Hz 下是 ±8ms）。
	ebiten.SetTPS(240)

	if err := ebiten.RunGame(app); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
