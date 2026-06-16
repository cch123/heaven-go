package engine

// evalNum 按 VFXManager 语义折叠一种效果的某个 start/end 参数对。
func evalNum(list []fxEvt, beat float64, key string, def float64) float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		v = Ease(ease, num(e.data, key+"Start", def), num(e.data, key+"End", def), prog)
	}
	return v
}

// evalColor 折叠颜色参数对（分量缓动，VfxColorEase 同语义）。
func evalColor(list []fxEvt, beat float64, key string, def [4]float64) [4]float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		c0 := colorOf(e.data, key+"Start", def)
		c1 := colorOf(e.data, key+"End", def)
		for i := 0; i < 4; i++ {
			v[i] = Ease(ease, c0[i], c1[i], prog)
		}
	}
	return v
}

func evalColorPair(list []fxEvt, beat float64, fromKey, toKey string, def [4]float64) [4]float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		from := colorOf(e.data, fromKey, def)
		to := colorOf(e.data, toKey, def)
		for i := 0; i < 4; i++ {
			v[i] = Ease(ease, from[i], to[i], prog)
		}
	}
	return v
}

// evalFlag 取"当前生效事件"的布尔参数（最后一个 beat<=now 的事件）。
func evalFlag(list []fxEvt, beat float64, key string, def bool) bool {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		v = flag(e.data, key, def)
	}
	return v
}

func evalPlainNum(list []fxEvt, beat float64, key string, def float64) float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		v = num(e.data, key, def)
	}
	return v
}

func num(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}

func flag(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

func colorOf(m map[string]any, key string, def [4]float64) [4]float64 {
	cm, ok := m[key].(map[string]any)
	if !ok {
		return def
	}
	get := func(k string, d float64) float64 {
		if f, ok := cm[k].(float64); ok {
			return f
		}
		return d
	}
	return [4]float64{get("r", def[0]), get("g", def[1]), get("b", def[2]), get("a", def[3])}
}
