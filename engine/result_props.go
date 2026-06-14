package engine

import "strings"

func resultSecondMessage(s string) string {
	if len(s) > 1 {
		rs := []rune(s)
		if len(rs) > 1 && !isLowerASCII(rs[0]) && isLowerASCII(rs[1]) {
			rs[0] = []rune(strings.ToLower(string(rs[0])))[0]
			s = string(rs)
		}
	}
	return "Also... " + s
}

func isLowerASCII(r rune) bool { return r >= 'a' && r <= 'z' }

func (a *App) resultProp(key string) string {
	if a.bm != nil {
		if s := a.bm.Prop(key); s != "" {
			return s
		}
	}
	defaults := map[string]string{
		"resultcaption":   "Rhythm League Notes",
		"resultcommon_hi": "That was great! Really great!",
		"resultcommon_ok": "Eh. Passable.",
		"resultcommon_ng": "That...could have been better.",
		"resultcat0_hi":   "You show strong fundamentals.",
		"resultcat0_ng":   "Work on your fundamentals.",
		"resultcat1_hi":   "You kept the beat well.",
		"resultcat1_ng":   "You had trouble keeping the beat.",
		"resultcat2_hi":   "You had great aim.",
		"resultcat2_ng":   "Your aim was a little shaky.",
		"resultcat3_hi":   "You followed the example well.",
		"resultcat3_ng":   "Next time, follow the example better.",
		"epilogue_hi":     "Superb",
		"epilogue_ok":     "OK",
		"epilogue_ng":     "Try Again",
	}
	if s, ok := defaults[key]; ok {
		return s
	}
	switch {
	case strings.HasPrefix(key, "resultcat") && strings.HasSuffix(key, "_hi"):
		return defaults["resultcommon_hi"]
	case strings.HasPrefix(key, "resultcat") && strings.HasSuffix(key, "_ng"):
		return defaults["resultcommon_ng"]
	}
	return ""
}
