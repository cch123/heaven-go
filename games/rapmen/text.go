package rapmen

func (m *Module) rapText(ev rapEvt) string {
	if ev.caption == 999 {
		return ev.text
	}
	lang := ev.caption
	if lang == 0 {
		lang = m.subtitleLanguage
	}
	voice := ev.voice
	if ev.gender == 1 {
		voice = ev.womanVoice
	}
	if ev.gender == 1 {
		return lookupText(femaleText[ev.cue], lang, voice)
	}
	return lookupText(maleText[ev.cue], lang, voice)
}

func lookupText(table map[int]map[int]string, lang, voice int) string {
	if byVoice := table[lang]; byVoice != nil {
		if s := byVoice[voice]; s != "" {
			return s
		}
	}
	if byVoice := table[1]; byVoice != nil {
		if s := byVoice[voice]; s != "" {
			return s
		}
	}
	return ""
}

var maleText = map[string]map[int]map[int]string{
	"desuka": {
		1:  {1: "Yo, wakari|masuka?", 2: "Yo, sanji |desuka?", 3: "Yo, oyatsu |desuka?", 4: "Honto |desuka?"},
		2:  {1: "わかります|か?", 2: "PM 3:00 です|か?", 3: "おやつ です|か?", 4: "ほんとです|か?"},
		10: {1: "It's understood, |isn't it?", 2: "It's already 3PM, |isn't it?", 3: "It's time for a snack, |isn't it?", 4: "It's all good, |isn't it?"},
		11: {1: "You get it now, |huh?", 2: "It's 3PM, |huh?", 3: "It's snack time, |huh?", 4: "Maybe so, |huh?"},
	},
	"kamone": {
		1:  {1: "Tanoshi, |kamone.", 2: "Oishi, |kamone.", 3: "Herushi, |kamone.", 4: "Orenosei, |kamone.", 5: "Soremoso, |kamone."},
		2:  {1: "たのしい かも|ネ", 2: "おいしい かも|ネ", 3: "ヘルシー かも|ネ", 4: "オレの せいかも|ネ", 5: "それもそう かも|ネ"},
		10: {1: "Havin' fun, it |could be", 2: "Something sweet, it |could be", 3: "Something salty, it |could be", 4: "Some sugar candies, it |could be", 5: "A bag of chips, it |could be"},
		11: {1: "Havin' fun, |ya feel me?", 2: "Tasty snacks, |ya feel me?", 3: "Healthy snacks, |ya feel me?", 4: "Might've been us, |ya feel me?", 5: "That's how it be, |ya feel me?"},
	},
	"saiko": {
		1:  {1: "Oyatsuwa |saiko!!", 2: "Kibunwa |saiko!!", 3: "Orette |saiko!!", 4: "Kimitte |saiko!!", 5: "Oyatsuga |naiyo!!", 6: "Ore, shiranai|yo!!"},
		2:  {1: "おやつは |サイコー!!", 2: "きぶんは |サイコー!!", 3: "オレって |サイコー!!", 4: "キミって |サイコー!!", 5: "おやつが |ナイヨー!!", 6: "オレ, しらナイ|ヨー!!"},
		10: {1: "That's why snacks are |the BEST!!", 2: "They make you feel |the BEST!!", 3: "I am |the BEST!!", 4: "You are |the BEST!!", 5: "Hey, who swiped |the rest?", 6: "Lack of snacks makes me |depressed!!"},
		11: {1: "Snacking is so |AWESOME!", 2: "I feel so |AWESOME!", 3: "We are so |AWESOME!", 4: "You are so |AWESOME!", 5: "Wait a sec, we |LOST SOME!", 6: "Guess it's how we |LOST SOME!"},
	},
}

var femaleText = map[string]map[int]map[int]string{
	"desuka": {
		1:  {1: "Yo, oyatsu |desuka?", 2: "Yo, juji |desuka?", 3: "Naisho |desuka?"},
		2:  {1: "おやつ　です|か?", 2: "AM 10:00　です|か?", 3: "ナイショです|か?"},
		10: {1: "It's time for a snack, |isn't it?", 2: "It's 10 AM, |isn't it?", 3: "It's our little secret, |isn't it?"},
		11: {1: "It's snack time, |huh?", 2: "It's 10 AM, |right?", 3: "Who's to know, |right?"},
	},
	"kamone": {
		1:  {1: "Kare no oyatsu, |dane.", 2: "Akete ii, |kamone.", 3: "Tabete ii, |kamone."},
		2:  {1: "カレのおやつ だ|ね", 2: "あけていー　かも|ネ", 3: "たべていー　かも|ネ"},
		10: {1: "The Rap Men are out, it |could be", 2: "Their door is open... |could it be?", 3: "They've got snacks, I |could see"},
		11: {1: "These are all their snacks, |ya see?", 2: "Open 'em up, |ya feel me?", 3: "Eat 'em all up, |ya feel me?"},
	},
	"saiko": {
		1:  {1: "Oyatsuwa |saiko!!", 2: "Amakute |saiko!!", 3: "Kibunwa |saiko!!", 4: "Betsubara |saiko!!", 5: "Kareniwa |naisho!!", 6: "Darenimo |naisho!!"},
		2:  {1: "おやつは |サイコー!!", 2: "あまくて |サイコー!!", 3: "きぶんは |サイコー!!", 4: "べつばら |サイコー!!", 5: "カレには |ナイヨー!!", 6: "ダレにも |ナイヨー!!"},
		10: {1: "Snacks are |the BEST!!", 2: "Sweet or salty is |the BEST!!", 3: "They make you feel |the BEST!!", 4: "Raps and snacks are |the BEST!!", 5: "Secret snacking is |the BEST!!", 6: "Now let's wrap this rap to |digest!!"},
		11: {1: "Snacking is so |AWESOME!", 2: "Sweets are so |AWESOME!", 3: "I feel so |AWESOME!", 4: "Cakes are so |AWESOME!", 5: "Clueless that we |STOLE 'EM!", 6: "Tell 'em we are |LOATHSOME!"},
	},
}
