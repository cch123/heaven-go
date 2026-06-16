package nogame

import (
	"testing"

	"hsdemo/engine"
	"hsdemo/riq"
)

func TestNoGameIsExplicitInertModule(t *testing.T) {
	m := New()
	if got := m.ID(); got != "noGame" {
		t.Fatalf("ID = %q, want noGame", got)
	}
	if err := m.Load(&engine.Ctx{}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if mm, ok := m.(*Module); !ok || mm.face == nil {
		t.Fatalf("Load should prepare the official No Game label face, got %#v", m)
	}
	m.OnEvent(&riq.Entity{Datamodel: "noGame/ignored", Beat: 12})
	m.Ready()
	m.OnSwitch(12)
	m.Whiff(12)
	m.Update(0, 12)
}
