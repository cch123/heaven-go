package fanclub

func (m *Module) toSpot(unspot bool) {
	if unspot {
		m.materialSubtree(m.arisa, [4]float64{1, 1, 1, 1}, [4]float64{})
		m.tintSubtree(m.blue, [4]float64{1, 1, 1, 1})
		m.tintSubtree(m.orange, [4]float64{1, 1, 1, 1})
		for _, f := range m.fans {
			f.tint([4]float64{1, 1, 1, 1})
		}
		return
	}
	spot := [4]float64{117.0 / 255.0, 177.0 / 255.0, 209.0 / 255.0, 1}
	// Arisa's NtrIdolAri.ToSpot writes coreMat._AddColor, unlike the backup
	// dancers and crowd which use material _Color. Keeping this separate avoids
	// flattening the idol into the same blue tint as the spectators.
	m.materialSubtree(m.arisa, [4]float64{1, 1, 1, 1}, [4]float64{0, 100.0 / 255.0, 200.0 / 255.0, 0})
	m.tintSubtree(m.blue, spot)
	m.tintSubtree(m.orange, spot)
	for _, f := range m.fans {
		f.tint(spot)
	}
}

func (m *Module) materialSubtree(root string, matColor, add [4]float64) {
	prefix := root + "/"
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Sprite == "" || (n.Path != root && !hasPrefix(n.Path, prefix)) {
			continue
		}
		m.ctx.Scene.SetMaterialOver(n.Path, matColor, add)
	}
}

func (m *Module) tintSubtree(root string, c [4]float64) {
	prefix := root + "/"
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Sprite == "" || (n.Path != root && !hasPrefix(n.Path, prefix)) {
			continue
		}
		m.ctx.Scene.SetColorOver(n.Path, c)
	}
}

func (f *fan) tint(c [4]float64) {
	if f == nil || f.inst == nil {
		return
	}
	for _, n := range f.inst.T.Nodes {
		f.inst.SetColor(n.RelPath, c)
	}
}
