package freezeframe

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type carInstance struct {
	ev        carSpawnEvt
	inst      *kart.Instance
	idleClip  string
	moveClip  string
	spawnBase [2]float64
	smokeRel  [2]float64
}

type walkerInstance struct {
	ev   walkerEvt
	inst *kart.Instance
}

func (m *Module) initScene(beat float64) {
	m.showOverlay = true
	m.showCameraMan = true
	m.followCamera = true
	m.showCrowd = false
	m.showBillboard = false
	m.autoBop = true
	m.autoBlink = true
	m.crosshairOn = true
	m.showingPhotos = false
	m.signMoving = false
	m.moveRun = moveRuntime{}
	m.rotRun = rotateRuntime{}
	m.scaleRun = scaleRuntime{}

	for _, root := range []string{"FarCar", "NearCar", "Walker", "Main/Background/FarCarSpawn/FarCar", "Main/NearCarSpawn/NearCar", "Main/WalkerSpawn/Walker"} {
		m.ctx.Scene.SetActive(root, false)
	}
	m.ctx.Scene.SetActive(m.overlay, true)
	m.ctx.Scene.SetActive(m.crosshair, true)
	m.ctx.Scene.SetActive(m.cameraMan, true)
	m.ctx.Scene.SetActive(m.dimRect, false)
	m.ctx.Scene.SetActive(m.billboards, false)
	m.ctx.Scene.PlayDefaultState(m.cameraMan, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.shutter, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.results, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.introSign, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.crowd, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.SetPosOver(m.cameraMan, m.cameraBasePos[0], m.cameraBasePos[1])
	m.ctx.Scene.SetSpinOver(m.cameraMan, 0)
	m.ctx.Scene.SetScaleOver(m.cameraMan, m.cameraBaseScale[0], m.cameraBaseScale[1])
	for _, p := range m.photoRoots {
		m.hidePhoto(p, beat)
	}
}

func (m *Module) applyPersistent(beat float64) {
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.autoBop, m.autoBlink = ev.autoBop, ev.autoBlink
		if beat >= ev.beat && beat <= ev.beat+ev.length {
			if ev.bop {
				m.bop(beat)
			}
			if ev.blink {
				m.crosshairBlink()
			}
		}
	}
	for _, ev := range m.overlays {
		if ev.beat <= beat {
			m.applyOverlay(ev)
		}
	}
	for _, ev := range m.crowds {
		if ev.beat <= beat {
			m.applyCrowd(ev)
		}
	}
	for _, ev := range m.introSigns {
		if ev.beat <= beat {
			m.startIntroSign(ev)
		}
	}
	for _, ev := range m.introLights {
		if ev.beat <= beat {
			if ev.on {
				switch {
				case beat >= ev.beat+2*ev.length:
					m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light03", ev.beat+2*ev.length, 0.5)
				case beat >= ev.beat+ev.length:
					m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light02", ev.beat+ev.length, 0.5)
				default:
					m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "Light01", ev.beat, 0.5)
				}
			} else {
				m.ctx.Scene.PlayStateLayer("freezeFrame/intro/lights", m.introSign, "LightsOff", ev.beat, 0.5)
			}
		}
	}
	for _, ev := range m.moves {
		if ev.beat <= beat {
			m.applyCameraMove(ev)
		}
	}
	m.updateIntroSign(beat)
	m.updateCameraMan(beat)
	m.updateVisibility()
}

func (m *Module) applyOverlay(ev overlayEvt) {
	m.showOverlay = ev.showOverlay
	m.showCameraMan = ev.showTJ
	m.followCamera = ev.followCamera
	m.updateVisibility()
}

func (m *Module) applyCrowd(ev crowdEvt) {
	m.showCrowd = ev.show
	m.showBillboard = ev.billboard
	if ev.show {
		m.ctx.Scene.PlayState(m.crowd, "Show", ev.beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(m.crowd, "Hide", ev.beat, 0.5)
	}
	if ev.custom {
		sprites := m.ctx.Assets.Extra.Components["game"].SpriteArrays["CrowdSprites"]
		parts := []struct {
			path string
			idx  int
		}{
			{m.crowdFarLeft, ev.farLeft}, {m.crowdLeft, ev.left},
			{m.crowdRight, ev.right}, {m.crowdFarRight, ev.farRight},
		}
		for _, p := range parts {
			if len(sprites) > 0 {
				m.ctx.Scene.SetSpriteOver(p.path, sprites[p.idx%len(sprites)])
			}
		}
	}
	m.updateVisibility()
}

func (m *Module) startIntroSign(ev introSignEvt) {
	m.activeSign = ev
	m.signMoving = true
}

func (m *Module) updateIntroSign(beat float64) {
	if !m.signMoving {
		return
	}
	u := norm(beat, m.activeSign.beat, m.activeSign.length)
	eased := frozen01(engine.Ease(m.activeSign.ease, 0, 1, u))
	state := "Enter"
	if !m.activeSign.enter {
		state = "Exit"
	}
	m.ctx.Scene.PlayFrozen(m.introSign, state, eased)
	if u >= 1 {
		m.signMoving = false
	}
}

func (m *Module) applyCameraMove(ev cameraMoveEvt) {
	if ev.move {
		m.moveRun = moveRuntime{
			active: true, beat: ev.beat, len: ev.length, ease: ev.ease,
			x0: m.cameraBasePos[0] + ev.startX, y0: m.cameraBasePos[1] + ev.startY,
			x1: m.cameraBasePos[0] + ev.endX, y1: m.cameraBasePos[1] + ev.endY,
		}
	}
	if ev.rotate {
		m.rotRun = rotateRuntime{active: true, beat: ev.beat, len: ev.length, start: ev.startRot, end: ev.endRot, ease: ev.ease}
	}
	if ev.scale {
		m.scaleRun = scaleRuntime{active: true, beat: ev.beat, len: ev.length, ease: ev.ease, x0: ev.startSX, y0: ev.startSY, x1: ev.endSX, y1: ev.endSY}
	}
}

func (m *Module) updateCameraMan(beat float64) {
	if m.moveRun.active {
		u := norm(beat, m.moveRun.beat, m.moveRun.len)
		x := engine.Ease(m.moveRun.ease, m.moveRun.x0, m.moveRun.x1, u)
		y := engine.Ease(m.moveRun.ease, m.moveRun.y0, m.moveRun.y1, u)
		m.ctx.Scene.SetPosOver(m.cameraMan, x, y)
		if u >= 1 {
			m.moveRun.active = false
		}
	}
	if m.rotRun.active {
		u := norm(beat, m.rotRun.beat, m.rotRun.len)
		deg := engine.Ease(m.rotRun.ease, m.rotRun.start, m.rotRun.end, u)
		m.ctx.Scene.SetSpinOver(m.cameraMan, deg*math.Pi/180)
		if u >= 1 {
			m.rotRun.active = false
		}
	}
	if m.scaleRun.active {
		u := norm(beat, m.scaleRun.beat, m.scaleRun.len)
		sx := engine.Ease(m.scaleRun.ease, m.scaleRun.x0, m.scaleRun.x1, u)
		sy := engine.Ease(m.scaleRun.ease, m.scaleRun.y0, m.scaleRun.y1, u)
		m.ctx.Scene.SetScaleOver(m.cameraMan, sx, sy)
		if u >= 1 {
			m.scaleRun.active = false
		}
	}
}

func (m *Module) updateVisibility() {
	m.ctx.Scene.SetActive(m.overlay, m.showOverlay && !m.showingPhotos)
	m.ctx.Scene.SetActive(m.dimRect, m.showingPhotos)
	m.ctx.Scene.SetActive(m.cameraMan, m.showCameraMan)
	m.ctx.Scene.SetActive(m.billboards, m.showBillboard)
	m.ctx.Scene.SetActive(m.crosshair, m.crosshairOn)
}

func (m *Module) bop(beat float64) {
	if m.showingPhotos {
		return
	}
	m.ctx.Scene.PlayState(m.cameraMan, "Bop", beat, 0.5)
}

func (m *Module) crosshairBlink() {
	m.crosshairOn = !m.crosshairOn
	m.ctx.Scene.SetActive(m.crosshair, m.crosshairOn)
}

func (m *Module) cameraFlash(beat float64) {
	m.ctx.Scene.PlayState(m.shutter, "Shut", beat, 0.5)
	m.ctx.Scene.PlayState(m.cameraMan, "Flash", beat, 0.5)
	m.ctx.Sound("shutter")
}

func (m *Module) clearPhotos() {
	m.photos = nil
}

func (m *Module) pushPhoto(args photoArgs, seedBeat float64) {
	if args.clear {
		m.clearPhotos()
	}
	if args.typ == photoRandom {
		if pseudo(seedBeat, float64(len(m.photos)), 0) >= 0.875 {
			switch int(pseudo(seedBeat, float64(len(m.photos)), 1) * 3) {
			case 0:
				args.typ = photoNinja
			case 1:
				args.typ = photoGhost
			default:
				args.typ = photoRats
			}
		} else {
			args.typ = photoDefault
		}
	}
	m.photos = append(m.photos, args)
}

func pseudo(a, b, c float64) float64 {
	v := math.Sin(a*12.9898+b*78.233+c*37.719) * 43758.5453
	return v - math.Floor(v)
}

func (m *Module) showPhotos(beat, length float64, grade gradeType, audience, clearCache bool) {
	if len(m.photos) == 0 {
		return
	}
	m.ctx.Sound("pictureShow")
	score := 2
	for _, p := range m.photos {
		if p.state <= -2 {
			score = 0
			break
		}
		if score == 2 && p.state != 0 {
			score = 1
		}
	}
	m.playResult(score, grade, beat)
	for i := 0; i < len(m.photos) && i < len(m.photoRoots); i++ {
		m.showPhoto(m.photoRoots[i], m.photos[i], beat)
	}
	if clearCache {
		m.clearPhotos()
	}
	m.showingPhotos = true
	m.updateVisibility()
	switch score {
	case 2:
		m.ctx.Scene.PlayState(m.cameraMan, "Happy", beat, 0.5)
		m.ctx.Sound("result_Hi")
	case 1:
		m.ctx.Scene.PlayState(m.cameraMan, "Oops", beat, 0.5)
		m.ctx.Sound("result_Ok")
		if audience {
			m.ctx.PlayCommon("applause")
		}
	default:
		m.ctx.Scene.PlayState(m.cameraMan, "Cry", beat, 0.5)
		m.ctx.Sound("result_Ng")
	}
	m.ctx.At(beat+length, func() { m.hidePhotos(beat + length) })
}

func (m *Module) playResult(score int, grade gradeType, beat float64) {
	if grade == gradeNone {
		m.ctx.Scene.PlayState(m.results, "None", beat, 0.5)
		return
	}
	if grade == gradeSymbols {
		switch score {
		case 0:
			m.ctx.Scene.PlayState(m.results, "Batsu", beat, 0.5)
		case 1:
			m.ctx.Scene.PlayState(m.results, "Sankaku", beat, 0.5)
		default:
			m.ctx.Scene.PlayState(m.results, "Maru", beat, 0.5)
		}
		return
	}
	switch score {
	case 0:
		m.ctx.Scene.PlayState(m.results, "ThumbsDown", beat, 0.5)
	case 1:
		m.ctx.Scene.PlayState(m.results, "ThumbsSide", beat, 0.5)
	default:
		m.ctx.Scene.PlayState(m.results, "ThumbsUp", beat, 0.5)
	}
}

func (m *Module) showPhoto(path string, args photoArgs, beat float64) {
	m.hidePhoto(path, beat)
	if args.state <= -2 {
		return
	}
	carState := "SlowCar_Perfect"
	if args.car == carFast {
		carState = "FastCar_Perfect"
	}
	if args.state > 0 {
		if args.car == carFast {
			carState = "FastCar_Late"
		} else {
			carState = "SlowCar_Late"
		}
	} else if args.state < 0 {
		if args.car == carFast {
			carState = "FastCar_Early"
		} else {
			carState = "SlowCar_Early"
		}
	}
	m.ctx.Scene.PlayStateLayer(path+"/car", path, carState, beat, 0.5)
	m.ctx.Scene.PlayStateLayer(path+"/cameo", path, cameoState(args), beat, 0.5)
	m.ctx.Scene.PlayStateLayer(path+"/show", path, "Show", beat, 0.5)
}

func (m *Module) hidePhoto(path string, beat float64) {
	m.ctx.Scene.PlayStateLayer(path+"/car", path, "NoCar", beat, 0.5)
	m.ctx.Scene.PlayStateLayer(path+"/cameo", path, "Cameo_None", beat, 0.5)
	m.ctx.Scene.PlayStateLayer(path+"/show", path, "Hide", beat, 0.5)
}

func (m *Module) hidePhotos(beat float64) {
	m.updateVisibility()
	m.ctx.Scene.SetActive(m.dimRect, false)
	for _, p := range m.photoRoots {
		m.hidePhoto(p, beat)
	}
	m.ctx.Scene.PlayState(m.results, "None", beat, 0.5)
	m.showingPhotos = false
	m.updateVisibility()
	if math.Abs(beat-math.Round(beat)) < 1e-6 && m.autoBop {
		m.ctx.Scene.PlayState(m.cameraMan, "Bop", beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(m.cameraMan, "Idle", beat, 0.5)
	}
}

func cameoState(args photoArgs) string {
	if args.state != 0 {
		switch args.typ {
		case photoGirlfriendRight:
			if args.state > 0 {
				return "Cameo_Girlfriend_Right_Late"
			}
			return "Cameo_Girlfriend_Right_Early"
		case photoGirlfriendLeft:
			if args.state > 0 {
				return "Cameo_Girlfriend_Left_Late"
			}
			return "Cameo_Girlfriend_Left_Early"
		case photoDude1Right:
			if args.state > 0 {
				return "Cameo_Dude1_Right_Late"
			}
			return "Cameo_Dude1_Right_Early"
		case photoDude1Left:
			if args.state > 0 {
				return "Cameo_Dude1_Left_Late"
			}
			return "Cameo_Dude1_Left_Early"
		case photoDude2Right:
			if args.state > 0 {
				return "Cameo_Dude2_Right_Late"
			}
			return "Cameo_Dude2_Right_Early"
		case photoDude2Left:
			if args.state > 0 {
				return "Cameo_Dude2_Left_Late"
			}
			return "Cameo_Dude2_Left_Early"
		default:
			return "Cameo_None"
		}
	}
	switch args.typ {
	case photoNinja:
		return "Cameo_Ninja"
	case photoGhost:
		return "Cameo_Ghost"
	case photoRats:
		return "Cameo_Rats"
	case photoPeace:
		if args.car == carFast {
			return "Cameo_PeaceFast"
		}
		return "Cameo_PeaceSlow"
	case photoGirlfriendRight:
		return "Cameo_Girlfriend_Right_Perfect"
	case photoGirlfriendLeft:
		return "Cameo_Girlfriend_Left_Perfect"
	case photoDude1Right:
		return "Cameo_Dude1_Right_Perfect"
	case photoDude1Left:
		return "Cameo_Dude1_Left_Perfect"
	case photoDude2Right:
		return "Cameo_Dude2_Right_Perfect"
	case photoDude2Left:
		return "Cameo_Dude2_Left_Perfect"
	default:
		return "Cameo_None"
	}
}

func (m *Module) spawnCar(ev carSpawnEvt) {
	var tmpl *kart.Template
	idleClip := "Animations/FarCar/Idle"
	moveClip := "Animations/FarCar/" + ev.state
	base := m.farSpawnPos
	smoke := [2]float64{0.11, -0.11}
	if ev.near {
		tmpl = m.nearT
		idleClip = "Animations/NearCar/Idle"
		moveClip = "Animations/NearCar/" + ev.state
		base = m.nearSpawnPos
		smoke = [2]float64{-0.35, -0.18}
	} else {
		tmpl = m.farT
	}
	if tmpl == nil {
		return
	}
	inst := tmpl.NewInstance()
	inst.Offset[0] += base[0]
	inst.Offset[1] += base[1]
	scale := clipScale(m.ctx, moveClip, ev.length)
	inst.PlayLayer("freezeFrame/car/move", "", moveClip, ev.beat, scale)
	m.activeCars = append(m.activeCars, &carInstance{ev: ev, inst: inst, idleClip: idleClip, moveClip: moveClip, spawnBase: base, smokeRel: smoke})
}

func (m *Module) spawnPersistedCars(beat float64) {
	for _, ev := range m.spawns {
		if beat >= ev.beat && beat <= ev.beat+ev.length+3 {
			m.spawnCar(ev)
		}
	}
}

func (c *carInstance) queue(m *Module, beat float64) {
	if c.inst == nil {
		return
	}
	c.inst.PlayNormalized("", c.idleClip, math.Mod(math.Max(beat, 0)*4, 1))
	c.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	c.queueSmoke(m, beat)
}

func (c *carInstance) queueSmoke(m *Module, beat float64) {
	if beat < c.ev.beat || beat > c.ev.beat+c.ev.length+0.8 {
		return
	}
	start := int(math.Floor((beat - c.ev.beat) * 8))
	for i := start - 6; i <= start; i++ {
		if i < 0 {
			continue
		}
		t := c.ev.beat + float64(i)/8
		age := beat - t
		if age < 0 || age > 0.75 {
			continue
		}
		u := age / 0.75
		x := c.inst.Offset[0] + c.smokeRel[0] - u*0.35
		y := c.inst.Offset[1] + c.smokeRel[1] + u*0.22
		s := 0.22 + 0.35*u
		a := 1 - u
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: "CloudParticle_" + itoa(i%4),
			World:  kart.TRS(x, y, 0, s, s),
			Order:  54,
			Tint:   [4]float64{1, 1, 1, a},
		})
	}
}

func clipScale(ctx *engine.Ctx, clip string, length float64) float64 {
	if length <= 0 {
		return ctx.SecPerBeat(ctx.Beat())
	}
	if a := ctx.Assets.Anims[clip]; a != nil && a.Duration > 0 {
		return a.Duration / length
	}
	return 1 / length
}

func (m *Module) spawnWalker(ev walkerEvt, nowBeat float64) {
	if m.walkerT == nil {
		return
	}
	inst := m.walkerT.NewInstance()
	inst.Offset[0] += m.walkerSpawnPos[0]
	inst.Offset[1] += m.walkerSpawnPos[1]
	inst.SetGroupOrder(ev.layer)
	typ := "Dude1"
	if ev.kind == personDude2 {
		typ = "Dude2"
	} else if ev.kind == personGirlfriend {
		typ = "Girlfriend"
	}
	dir := ev.dir
	if dir == directionRandom {
		if pseudo(ev.beat, ev.length, 99) >= 0.5 {
			dir = directionLeft
		} else {
			dir = directionRight
		}
	}
	move := "EnterRight"
	if dir == directionLeft {
		move = "EnterLeft"
	}
	inst.PlayStateLayer("freezeFrame/walker/type", "", typ, ev.beat, 0.5)
	inst.PlayLayer("freezeFrame/walker/move", "", "Animations/Walker/"+move, ev.beat, clipScale(m.ctx, "Animations/Walker/"+move, ev.length))
	nextBeat := math.Ceil(ev.beat)
	for b := nextBeat; b < ev.beat+ev.length; b++ {
		b := b
		if b >= nowBeat {
			m.ctx.At(b, func() { inst.PlayStateLayer("freezeFrame/walker/bop"+itoa(int(b*100)), "", "Bop", b, 0.5) })
		}
	}
	m.activeWalker = append(m.activeWalker, &walkerInstance{ev: ev, inst: inst})
}

func (m *Module) spawnPersistedWalkers(beat float64) {
	for _, ev := range m.walkers {
		if beat >= ev.beat && beat <= ev.beat+ev.length {
			m.spawnWalker(ev, beat)
		}
	}
}

func (w *walkerInstance) queue(m *Module, beat float64) {
	if w.inst == nil {
		return
	}
	w.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func (m *Module) compactRuntime(beat float64) {
	cars := m.activeCars[:0]
	for _, c := range m.activeCars {
		if beat <= c.ev.beat+c.ev.length+3 {
			cars = append(cars, c)
		}
	}
	m.activeCars = cars
	walkers := m.activeWalker[:0]
	for _, w := range m.activeWalker {
		if beat <= w.ev.beat+w.ev.length {
			walkers = append(walkers, w)
		}
	}
	m.activeWalker = walkers
}
