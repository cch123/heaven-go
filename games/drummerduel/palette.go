package drummerduel

import "hsdemo/kart"

var (
	leftBlush     = rgba(0.9372549, 0.42745098, 0.42745098)
	leftSkin      = rgba(1, 0.9372549, 0.78039217)
	leftHeadband  = rgba(1, 0.5529412, 0.07058824)
	rightBlush    = rgba(0.9372549, 0.654902, 0.654902)
	rightSkin     = rgba(1, 1, 1)
	rightHeadband = rgba(0, 0.9843137, 1)
	angrySkin     = rgba(1, 0.50588235, 0.50588235)
)

var defaultPalettes = map[string]kart.Palette{
	"groundGradient": pal(rgba(1, 0.86666673, 0.8705883), rgba(1, 1, 1), rgba(0.5372549, 0.8313726, 1)),
	"areaLeft":       pal(rgba(1, 0.86666673, 0.8705883), rgba(1, 0.9529412, 0.90588236), rgba(1, 0.60784316, 0.46274513)),
	"pompomLeft":     pal(rgba(1, 0.7686275, 0.56078434), rgba(1, 1, 1), rgba(1, 0.5529412, 0.07058824)),
	"drumsticks":     pal(rgba(0.5764706, 0.40000004, 0.078431375), rgba(1, 1, 1), rgba(1, 0.43921572, 0.11764707)),
	"referee":        pal(rgba(1, 0.86666673, 0.8705883), rgba(1, 1, 1), rgba(0.5372549, 0.8313726, 1)),
	"charRight":      pal(rgba(1, 1, 0), rgba(1, 1, 1), rgba(0, 0.9843138, 1)),
	"faceRightMad":   pal(rgba(1, 0.34117648, 0.2), rgba(0.9372549, 0.654902, 0.654902), rgba(0, 0.9843137, 1)),
	"pantsLeft":      pal(rgba(0.50980395, 0.25882354, 0), rgba(1, 1, 1), rgba(1, 0.5529412, 0.07058824)),
	"pantsRight":     pal(rgba(0, 0.25490198, 0.7803922), rgba(1, 1, 1), rgba(0, 0.9843138, 1)),
	"faceLeftMad":    pal(rgba(1, 0.34117648, 0.2), rgba(0.9372549, 0.654902, 0.654902), rgba(1, 0.5529412, 0.07058824)),
	"areaRight":      pal(rgba(0.5372549, 0.8313726, 1), rgba(0.8078432, 0.93725497, 1), rgba(0.21568629, 0.61960787, 0.85098046)),
	"faceRight":      pal(rightBlush, rightSkin, rightHeadband),
	"sweat":          pal(rgba(0.5921569, 1, 1), rgba(1, 1, 1), rgba(0, 0.9843137, 1)),
	"bgGradient":     pal(rgba(0.9647059, 0.972549, 0.99607843), rgba(1, 1, 1), rgba(0.64705884, 0.9019608, 0.8627451)),
	"charBgArms":     pal(rgba(0.9372549, 0.5568628, 0.27058825), rgba(1, 0.9372549, 0.78039217), rgba(0.627451, 0.627451, 0.627451)),
	"charLeft":       pal(rgba(0.65882355, 0.98823535, 0), rgba(1, 1, 1), rgba(1, 0.5529412, 0.07058824)),
	"drummerLeft":    pal(rgba(0.65882355, 0.98823535, 0), rgba(1, 0.9372549, 0.78039217), rgba(1, 0.5529412, 0.07058824)),
	"pompomRight":    pal(rgba(0.5921569, 1, 1), rgba(1, 1, 1), rgba(0, 0.9843138, 1)),
	"areaMiddle":     pal(rgba(0.8207547, 0.8207547, 0.8207547), rgba(0.9339623, 0.9339623, 0.9339623), rgba(0.5660378, 0.5660378, 0.5660378)),
	"faceLeft":       pal(leftBlush, leftSkin, leftHeadband),
}

func (m *Module) applyDefaultPalettes() {
	for name, p := range defaultPalettes {
		m.ctx.Scene.SetPaletteFor(name, p)
	}
}

func (m *Module) applyDrummerHeadColors() {
	left := leftSkin
	right := rightSkin
	if m.isAngry {
		left = angrySkin
		if !m.isWhiffing {
			right = angrySkin
		}
	}
	m.ctx.Scene.SetPaletteFor("faceLeft", pal(leftBlush, left, leftHeadband))
	m.ctx.Scene.SetPaletteFor("faceRight", pal(rightBlush, right, rightHeadband))
}

func pal(alpha, fill, outline [4]float64) kart.Palette {
	return kart.Palette{Alpha: alpha, Fill: fill, Outline: outline}
}

func rgba(r, g, b float64) [4]float64 {
	return [4]float64{r, g, b, 1}
}
