package rapmen

func sc(clip string, beat, off float64) soundCue {
	return soundCue{clip: clip, beat: beat, vol: 1, offset: off, pan: -0.25}
}

func scv(clip string, beat, vol, off float64) soundCue {
	return soundCue{clip: clip, beat: beat, vol: vol, offset: off, pan: -0.25}
}

func desukaSounds(female bool, voice int) []soundCue {
	if female {
		switch voice {
		case 2:
			return []soundCue{sc("rapWomen/yoW", 0, 0.03), scv("rapWomen/juW", 0.75, 0.75, 0.003), scv("rapWomen/jiW", 1.25, 0.75, 0.001), sc("rapWomen/desuAW", 1.5, 0), sc("rapWomen/kaAW", 2, 0.015)}
		case 3:
			return []soundCue{sc("rapWomen/naishoA1W", 0, 0.1), sc("rapWomen/naishoA2W", 0.25, 0.035), sc("rapWomen/desuBW", 0.5, 0.01), sc("rapWomen/kaBW", 0.75, 0.02)}
		default:
			return []soundCue{sc("rapWomen/yoW", 0, 0.03), sc("rapWomen/oyatsuA1W", 0.75, 0.05), sc("rapWomen/oyatsuA2W", 1, 0.01), sc("rapWomen/desuAW", 1.5, 0), sc("rapWomen/kaAW", 2, 0.015)}
		}
	}
	switch voice {
	case 1:
		return []soundCue{sc("yo", 0, 0.03), sc("wakari1", 0.75, 0.025), sc("wakari2", 1, 0.012), sc("wakari3", 1.25, 0.022), sc("masuka1", 1.5, 0.093), sc("masuka2", 2, 0.017)}
	case 2:
		return []soundCue{sc("yo", 0, 0.03), sc("sanji1", 0.75, 0.094), sc("sanji2", 1.25, 0.009), sc("desuA", 1.5, 0.02), sc("kaA", 2, 0.003)}
	case 4:
		return []soundCue{sc("honto1", -0.25, 0.029), sc("honto2", 0.25, 0.004), sc("desuB", 0.5, 0.01), sc("kaB", 1, 0.02)}
	default:
		return []soundCue{sc("yo", 0, 0.03), sc("oyatsu1", 0.75, 0.03), sc("oyatsu2", 1, 0.02), sc("desuA", 1.5, 0.02), sc("kaA", 2, 0.003)}
	}
}

func kamoneSounds(female bool, voice int) []soundCue {
	if female {
		switch voice {
		case 2:
			return []soundCue{sc("rapWomen/akete1W", 0, 0.007), sc("rapWomen/akete2W", 0.25, 0.016), sc("rapWomen/akete3W", 0.5, 0.014), sc("rapWomen/iiAW", 0.75, 0.014), sc("rapWomen/kamone1W", 1.25, 0.008), sc("rapWomen/kamone2W", 1.5, 0.017), sc("rapWomen/kamone3W", 1.75, 0.005)}
		case 3:
			return []soundCue{sc("rapWomen/tabete1W", 0, 0.007), sc("rapWomen/tabete2W", 0.25, 0.016), sc("rapWomen/tabete3W", 0.5, 0.014), sc("rapWomen/iiAW", 0.75, 0.014), sc("rapWomen/kamone1W", 1.25, 0.008), sc("rapWomen/kamone2W", 1.5, 0.017), sc("rapWomen/kamone3W", 1.75, 0.005)}
		default:
			return []soundCue{sc("rapWomen/kareA1W", 0, 0.034), sc("rapWomen/kareA2W", 0.25, 0.008), sc("rapWomen/noW", 0.5, 0.017), sc("rapWomen/oyatsuB1W", 0.75, 0), sc("rapWomen/oyatsuB2W", 1.25, 0.2), sc("rapWomen/dane1W", 1.5, 0.031), sc("rapWomen/dane2W", 1.75, 0.037)}
		}
	}
	switch voice {
	case 1:
		return []soundCue{sc("tanoshi1", 0, 0.008), sc("tanoshi2", 0.25, 0), sc("tanoshi3", 0.5, 0.067), sc("kamone1", 1.25, 0.01), sc("kamone2", 1.5, 0), sc("kamone3", 1.75, 0.035)}
	case 3:
		return []soundCue{sc("herushi1", 0, 0.047), sc("herushi2", 0.25, 0.008), sc("herushi3", 0.5, 0.064), sc("kamone1", 1.25, 0.01), sc("kamone2", 1.5, 0), sc("kamone3", 1.75, 0.035)}
	case 4:
		return []soundCue{sc("orenosei1", 0, 0), sc("orenosei2", 0.25, 0.006), sc("orenosei3", 0.5, 0.022), sc("orenosei4", 0.75, 0.175), sc("kamone1", 1.25, 0.01), sc("kamone2", 1.5, 0), sc("kamone3", 1.75, 0.035)}
	case 5:
		return []soundCue{sc("soremoso1", 0, 0.038), sc("soremoso2", 0.25, 0.01), sc("soremoso3", 0.5, 0.05), sc("soremoso4", 0.75, 0.143), sc("kamone1", 1.25, 0.01), sc("kamone2", 1.5, 0), sc("kamone3", 1.75, 0.035)}
	default:
		return []soundCue{sc("oishi1", 0, 0.019), sc("oishi3", 0.5, 0.135), sc("kamone1", 1.25, 0.01), sc("kamone2", 1.5, 0), sc("kamone3", 1.75, 0.035)}
	}
}

func saikoSounds(female bool, voice int) []soundCue {
	if female {
		switch voice {
		case 2:
			return []soundCue{sc("rapWomen/amakute1W", 0, 0.01), sc("rapWomen/amakute2W", 0.25, 0.005), sc("rapWomen/amakute3W", 0.5, 0.03), sc("rapWomen/amakute4W", 0.75, 0.014), sc("rapWomen/saikoA1W", 1, 0.084), sc("rapWomen/saikoA2W", 1.5, 0.027)}
		case 3:
			return []soundCue{sc("rapWomen/kibun1W", 0, 0.014), sc("rapWomen/kibun2W", 0.25, 0), sc("rapWomen/waBW", 0.75, 0.024), sc("rapWomen/saikoA1W", 1, 0.084), sc("rapWomen/saikoA2W", 1.5, 0.027)}
		case 4:
			return []soundCue{sc("rapWomen/betsuW", 0, 0.032), sc("rapWomen/baraW", 0.5, 0.017), sc("rapWomen/saikoA1W", 1, 0.084), sc("rapWomen/saikoA2W", 1.5, 0.027)}
		case 5:
			return []soundCue{sc("rapWomen/kareB1W", 0, 0.024), sc("rapWomen/kareB2W", 0.25, 0.016), sc("rapWomen/niBW", 0.5, 0.029), sc("rapWomen/waCW", 0.75, 0.041), sc("rapWomen/naishoB1W", 1, 0.058), sc("rapWomen/naishoB2W", 1.5, 0.057)}
		case 6:
			return []soundCue{sc("rapWomen/dare1W", 0, 0.006), sc("rapWomen/dare2W", 0.25, 0.013), sc("rapWomen/niAW", 0.5, 0.026), sc("rapWomen/moW", 0.75, 0.036), sc("rapWomen/naishoB1W", 1, 0.058), sc("rapWomen/naishoB2W", 1.5, 0.057)}
		default:
			return []soundCue{sc("rapWomen/oyatsuC1W", 0, 0.008), sc("rapWomen/oyatsuC2W", 0.25, 0.009), sc("rapWomen/oyatsuC3W", 0.5, 0.121), sc("rapWomen/waAW", 0.75, 0.007), sc("rapWomen/saikoA1W", 1, 0.084), sc("rapWomen/saikoA2W", 1.5, 0.027)}
		}
	}
	switch voice {
	case 2:
		return []soundCue{sc("kibunwa1", 0, 0.005), sc("kibunwa2", 0.25, 0.016), sc("kibunwa3", 0.75, 0.024), sc("saiko1", 1, 0.125), sc("saiko2", 1.5, 0.005)}
	case 3:
		return []soundCue{sc("orette1", 0, 0.006), sc("orette2", 0.25, 0.013), sc("orette3", 0.75, 0.003), sc("saiko1", 1, 0.125), sc("saiko2", 1.5, 0.005)}
	case 4:
		return []soundCue{sc("kimitte1", 0, 0.001), sc("kimitte2", 0.25, 0.038), sc("kimitte3", 0.75, 0.003), sc("saiko1", 1, 0.125), sc("saiko2", 1.5, 0.005)}
	case 5:
		return []soundCue{sc("oyatsuga1", 0, 0.004), sc("oyatsuga2", 0.25, 0.033), sc("oyatsuga3", 0.5, 0.087), sc("oyatsuga4", 0.75, 0.01), sc("naiyo1", 1, 0.07), sc("naiyo2", 1.5, 0.003)}
	case 6:
		return []soundCue{sc("oreshira1", 0, 0.006), sc("oreshira2", 0.25, 0.083), sc("oreshira3", 0.5, 0.083), sc("oreshira4", 0.75, 0.009), sc("naiyo1", 1, 0.07), sc("naiyo2", 1.5, 0.003)}
	default:
		return []soundCue{sc("oyatsuwa1", 0, 0.004), sc("oyatsuwa2", 0.25, 0.033), sc("oyatsuwa3", 0.5, 0.087), sc("oyatsuwa4", 0.75, 0.01), sc("saiko1", 1, 0.125), sc("saiko2", 1.5, 0.005)}
	}
}
