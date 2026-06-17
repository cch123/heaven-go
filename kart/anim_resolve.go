package kart

import (
	"path"
	"strings"

	"hsdemo/kmdata"
)

func resolveAnim(as *Assets, name string) (*kmdata.Anim, string, bool) {
	if as == nil || as.Anims == nil || name == "" {
		return nil, "", false
	}
	if a, ok := as.Anims[name]; ok && animHasCurves(a) {
		return a, name, true
	}
	if alt := fallbackClipName(name); alt != name {
		if a, ok := as.Anims[alt]; ok && animHasCurves(a) {
			return a, alt, true
		}
	}
	if a, ok := as.Anims[name]; ok {
		return a, name, true
	}
	return nil, "", false
}

func resolveAnimOnly(as *Assets, name string) (*kmdata.Anim, bool) {
	a, _, ok := resolveAnim(as, name)
	return a, ok
}

func fallbackClipName(name string) string {
	// Some Unity controllers point at imported "Animations/Foo" stubs while the
	// extracted .anim curves live under "Foo". Limit the fallback to that export
	// directory so namespaced clips such as "Girl/Bop" never collapse together.
	if strings.HasPrefix(name, "Animations/") {
		return path.Base(name)
	}
	return name
}

func animHasCurves(a *kmdata.Anim) bool {
	if a == nil {
		return false
	}
	return len(a.Pos) > 0 || len(a.Euler) > 0 || len(a.Scale) > 0 ||
		len(a.Sprites) > 0 || len(a.Floats) > 0
}
