package froghop

import "hsdemo/engine"

type frogKind int

const (
	frogBackup frogKind = iota
	frogLeader
	frogSinger
)

type frog struct {
	ctx         *engine.Ctx
	path        string
	kind        frogKind
	scale       [2]float64
	side        int
	bumped      bool
	spriteParts []string
	bodyParts   []string
	headParts   []string
	belt        string
	missFace    string
	beltColor   [4]float64
}

func newFrog(ctx *engine.Ctx, path string, kind frogKind) *frog {
	f := &frog{ctx: ctx, path: path, kind: kind, scale: nodeScale(ctx.Assets, path), side: -1, beltColor: white}
	for _, c := range ctx.Assets.Extra.Components {
		if c.Path != path {
			continue
		}
		f.spriteParts = append(f.spriteParts, c.RefArrays["SpriteParts"]...)
		f.bodyParts = append(f.bodyParts, c.RefArrays["BodyMat"]...)
		f.headParts = append(f.headParts, c.RefArrays["HeadMat"]...)
		f.belt = c.Refs["Belt"]
		f.missFace = c.Refs["MissFace"]
	}
	if f.belt == "" {
		f.belt = path + "/Belt"
	}
	return f
}

func (f *frog) reset(beat float64) {
	f.side = -1
	f.bumped = false
	f.ctx.Scene.SetScaleOver(f.path, f.scale[0], f.scale[1])
	f.ctx.Scene.PlayDefaultState(f.path, beat, f.ctx.SecPerBeat(beat))
	if f.missFace != "" {
		f.ctx.Scene.SetActive(f.missFace, false)
	}
	for _, p := range f.spriteParts {
		f.ctx.Scene.SetColorOver(p, white)
	}
	if f.belt != "" {
		f.ctx.Scene.SetColorOver(f.belt, f.beltColor)
	}
}

func (f *frog) bop(beat float64) {
	f.bumped = false
	f.ctx.Scene.PlayState(f.path, "Bop", beat, 0.5)
}

func (f *frog) hop(beat float64, side int, long bool) {
	f.swapSide(side)
	state := "Hop"
	if long {
		state = "LongHop"
	}
	f.bumped = false
	f.ctx.Scene.PlayState(f.path, state, beat, 0.5)
}

func (f *frog) charge(beat float64, side int) {
	f.swapSide(side)
	f.bumped = false
	f.ctx.Scene.PlayState(f.path, "Charge", beat, 0.5)
}

func (f *frog) spin(beat float64, hs bool) {
	state := "Spin"
	if hs && f.kind == frogSinger {
		state = "SpinHS"
	}
	f.bumped = false
	f.ctx.Scene.PlayState(f.path, state, beat, 0.5)
}

func (f *frog) talk(beat float64, state string, _ float64) {
	f.ctx.Scene.PlayStateLayer(f.path+"/talk", f.path, "Talk"+state, beat, 0.5)
}

func (f *frog) wink(beat float64, state string, end float64) {
	f.talk(beat, state, end)
	f.ctx.At(end, func() { f.talk(end, "Wide", end) })
}

func (f *frog) glare(beat float64) {
	if f.kind == frogBackup {
		f.ctx.Scene.PlayStateLayer(f.path+"/glare", f.path, "Glare", beat, 0.5)
	}
}

func (f *frog) sweat(beat float64) {
	if f.kind == frogBackup {
		f.ctx.Scene.PlayStateLayer(f.path+"/sweat", f.path, "Sweat", beat, 0.5)
	}
}

func (f *frog) bump(beat float64) {
	if f.bumped || f.kind != frogBackup {
		return
	}
	f.bumped = true
	f.ctx.Scene.SetScaleOver(f.path, f.scale[0], f.scale[1])
	f.ctx.Scene.PlayState(f.path, "Bump", beat, 0.5)
	f.ctx.Scene.PlayStateLayer(f.path+"/ouch", f.path, "Ouch", beat, 0.5)
	f.ctx.Sound("SE_NTR_FROG_EN_MISS")
	f.ctx.Sound("SE_NTR_FROG_EN_MISS_BOING")
	if f.missFace != "" {
		f.ctx.Scene.SetActive(f.missFace, true)
		f.ctx.At(beat+0.75, func() { f.ctx.Scene.SetActive(f.missFace, false) })
	}
}

func (f *frog) darken(reverse bool) {
	col := dim
	if reverse {
		col = white
	}
	for _, p := range f.spriteParts {
		f.ctx.Scene.SetColorOver(p, col)
	}
	if f.belt != "" {
		bc := f.beltColor
		if !reverse {
			bc = [4]float64{bc[0] * 0.5, bc[1] * 0.5, bc[2] * 0.5, bc[3]}
		}
		f.ctx.Scene.SetColorOver(f.belt, bc)
	}
}

func (f *frog) recolor(skin, tummy, pants, belt, sclera, lip [4]float64, lipstick, hasBelt bool) {
	// Heaven Studio drives Frog Hop colors through mapped material shader
	// channels. Ebitengine renders baked sprites, so we approximate the same
	// intent by coloring the extracted body/head/belt renderer groups.
	for _, p := range f.bodyParts {
		f.ctx.Scene.SetColorOver(p, mix3(skin, tummy, pants))
	}
	head := mix2(skin, sclera)
	if lipstick {
		head = mix3(skin, sclera, lip)
	}
	for _, p := range f.headParts {
		f.ctx.Scene.SetColorOver(p, head)
	}
	f.beltColor = belt
	if f.belt != "" {
		f.ctx.Scene.SetActive(f.belt, hasBelt)
		f.ctx.Scene.SetColorOver(f.belt, belt)
	}
}

func (f *frog) swapSide(side int) {
	if side != 0 {
		f.side = side
	} else {
		f.side *= -1
	}
	if f.side == 0 {
		f.side = 1
	}
	f.ctx.Scene.SetScaleOver(f.path, f.scale[0]*float64(f.side), f.scale[1])
}

func mix2(a, b [4]float64) [4]float64 {
	return [4]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2, 1}
}

func mix3(a, b, c [4]float64) [4]float64 {
	return [4]float64{(a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3, 1}
}
