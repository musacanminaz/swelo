package i18n

import "strings"

// reasonsTR translates skip reasons produced by safety and clean.
var reasonsTR = map[string]string{
	"not an absolute path":                 "mutlak yol değil",
	"does not exist":                       "yok",
	"cannot resolve parent":                "üst klasör çözülemedi",
	"home or root directory":               "ev veya kök klasör",
	"outside allowed roots":                "izin verilen klasörlerin dışında",
	"protected path":                       "korumalı yol",
	"is an allowed root itself":            "taranan kök klasörün kendisi",
	"environment file":                     "ortam dosyası (.env)",
	"tracked by git":                       "git'te takip ediliyor",
	"excluded in config":                   "ayarlarda hariç tutulmuş",
	"a process is running in this project": "bu projede çalışan bir süreç var",
	"newest version, kept":                 "en yeni sürüm, korunuyor",
}

// reasonPrefixesTR translates reasons that carry a value, e.g. "in use (SingletonLock)".
var reasonPrefixesTR = [][2]string{
	{"project changed in the last ", "son "},
	{"app is running ", "uygulama açık "},
	{"in use ", "kullanımda "},
	{"inside ", "içinde: "},
	{"user data file ", "kullanıcı verisi "},
}

// Reason returns a skip reason in the active language.
func Reason(s string) string {
	if lang != "tr" {
		return s
	}
	if t, ok := reasonsTR[s]; ok {
		return t
	}
	for _, p := range reasonPrefixesTR {
		if rest, ok := strings.CutPrefix(s, p[0]); ok {
			if p[0] == "project changed in the last " {
				return "son " + strings.TrimSuffix(rest, " days") + " günde değişmiş proje"
			}
			return p[1] + rest
		}
	}
	return s
}
