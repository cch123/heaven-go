package basketballgirls

import (
	"testing"

	"hsdemo/kart"
)

func TestLocalCameraZoom(t *testing.T) {
	m := New().(*Module)
	m.zooms = []zoomEvt{{beat: 4, length: 4, ease: 0, in: true}}
	if got := m.localCameraAt(2); got != m.camFar {
		t.Fatalf("camera before zoom = %#v, want far %#v", got, m.camFar)
	}
	got := m.localCameraAt(6)
	if got[1] != (m.camFar[1]+m.camNear[1])*0.5 || got[2] != (m.camFar[2]+m.camNear[2])*0.5 {
		t.Fatalf("camera mid zoom = %#v, want midpoint", got)
	}
	if got := m.localCameraAt(9); got != m.camNear {
		t.Fatalf("camera after zoom = %#v, want near %#v", got, m.camNear)
	}
}

func TestBasketballGirlsExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/basketballGirls", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"baseBall", "girlLeftAnim", "girlRightAnim", "goalAnim", "BGPlane"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if got := as.Extra.RefArrays["CameraPosition"]; len(got) != 2 {
		t.Fatalf("CameraPosition refs = %#v, want 2", got)
	}
	for _, ctrl := range []string{"Ball", "GirlLeft", "GirlRight", "Goal"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/ballPrepare", "Animations/ballCatch", "Animations/ballShootJust",
		"Animations/ballShootBarely", "Animations/ballJust", "Animations/ballBarely",
		"Animations/ballHit", "Animations/girlPrepare", "Animations/girlDribble",
		"Animations/girlPass", "Animations/girlCatch", "Animations/girlShoot",
		"Animations/girlHit", "Animations/girlBlank", "Animations/goalJust",
		"Animations/goalBarely", "Animations/goalIdle",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{
		"voice", "catch", "throw", "dunk", "ok1", "ok2", "6", "1", "A",
		"dribble1", "dribble2", "dribbleEcho1", "dribbleEcho2", "dribbleEcho3",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}
