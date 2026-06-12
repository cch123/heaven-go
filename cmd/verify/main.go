// verify 是移植验证录制器：autoplay 跑完整关卡，在指定拍抓帧写 PNG，
// 结束时打印判定计数。用于对照原版录屏做交付前审计。
//
//	go run ./cmd/verify -riq "levels/Meat Grinder.riq" -beats 0,8,33,36.5,57 -out /tmp/mg
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"

	// 已移植模块（与主程序保持同步）
	"hsdemo/games/bluebear"
	"hsdemo/games/kitties"
	"hsdemo/games/lockstep"
	"hsdemo/games/marchingorders"
	"hsdemo/games/meatgrinder"
	"hsdemo/games/munchymonk"
	"hsdemo/games/seesaw"
	"hsdemo/games/somen"
	"hsdemo/games/spacedance"
	"hsdemo/games/totemclimb"
	"hsdemo/games/trickclass"
)

type recorder struct {
	app     *engine.App
	targets []float64
	next    int
	out     string
	done    bool
	quit    bool // 抓完所有拍位立即退出（视觉快迭代，不等 RESULT）
}

func (r *recorder) Update() error { return r.app.Update() }

func (r *recorder) Draw(screen *ebiten.Image) {
	r.app.Draw(screen)
	beat := r.app.BeatNow()
	if r.next < len(r.targets) && beat >= r.targets[r.next] {
		path := fmt.Sprintf("%s_beat%g.png", r.out, r.targets[r.next])
		f, err := os.Create(path)
		if err == nil {
			if err := png.Encode(f, screen); err != nil {
				log.Printf("encode %s: %v", path, err)
			}
			f.Close()
			log.Printf("captured %s (beat %.2f)", path, beat)
		}
		r.next++
		if r.quit && r.next >= len(r.targets) {
			os.Exit(0)
		}
	}
	if r.app.Finished() && !r.done {
		r.done = true
		a, j, n, ms, w := r.app.RunCounts()
		log.Printf("RESULT ace=%d just=%d ng=%d miss=%d whiff=%d", a, j, n, ms, w)
		path := r.out + "_result.png"
		if f, err := os.Create(path); err == nil {
			png.Encode(f, screen)
			f.Close()
		}
		os.Exit(0)
	}
}

func (r *recorder) Layout(w, h int) (int, int) { return r.app.Layout(w, h) }

func main() {
	path := flag.String("riq", "", ".riq 谱面路径")
	assetsRoot := flag.String("assets", "assets", "提取资产根目录")
	beats := flag.String("beats", "", "抓帧拍位（逗号分隔）")
	out := flag.String("out", "/tmp/verify", "输出 PNG 前缀")
	quit := flag.Bool("quit", false, "抓完所有拍位后立即退出")
	flag.Parse()

	engine.Register("rhythmSomen", somen.New)
	engine.Register("trickClass", trickclass.New)
	engine.Register("meatGrinder", meatgrinder.New)
	engine.Register("totemClimb", totemclimb.New)
	engine.Register("seeSaw", seesaw.New)
	engine.Register("blueBear", bluebear.New)
	engine.Register("marchingOrders", marchingorders.New)
	engine.Register("kitties", kitties.New)
	engine.Register("lockstep", lockstep.New)
	engine.Register("spaceDance", spacedance.New)
	engine.Register("munchyMonk", munchymonk.New)

	app, err := engine.New(*assetsRoot, *path)
	if err != nil {
		log.Fatal(err)
	}
	app.Autoplay = true

	var targets []float64
	for _, s := range strings.Split(*beats, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			log.Fatalf("bad beat %q", s)
		}
		targets = append(targets, v)
	}
	sort.Float64s(targets)

	ebiten.SetWindowSize(engine.ScreenW, engine.ScreenH)
	ebiten.SetWindowTitle("Heaven Go — verify")
	ebiten.SetTPS(240)
	if err := ebiten.RunGame(&recorder{app: app, targets: targets, out: *out, quit: *quit}); err != nil &&
		err != ebiten.Termination {
		log.Fatal(err)
	}
}
