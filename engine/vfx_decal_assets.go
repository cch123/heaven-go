package engine

import (
	"bytes"
	"image"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

func decodeEmbeddedSprites(r *riq.Riq) map[string]*ebiten.Image {
	out := map[string]*ebiten.Image{}
	for name, spr := range r.CustomSprites {
		img, _, err := image.Decode(bytes.NewReader(spr.Data))
		if err != nil {
			log.Printf("engine: 自定义贴图 %s 解码失败: %v", spr.Name, err)
			continue
		}
		eimg := ebiten.NewImageFromImage(img)
		out[name] = eimg
		out[strings.ToLower(name)] = eimg
	}
	return out
}
