package i18n

import "strings"

// reasonsTR translates skip reasons produced by safety and clean.
var reasonsTR = map[string]string{
	"Time Machine is not set up":                   "Time Machine kurulu değil",
	"project folder is in the Trash":               "proje klasörü Çöp'te",
	"project folder no longer exists":              "proje klasörü artık yok",
	"idle build daemon":                            "boşta duran build daemon'ı",
	"Playwright browser older than a day":          "bir günden eski Playwright tarayıcısı",
	"MCP server whose session ended":               "oturumu bitmiş MCP sunucusu",
	"brew not installed":                           "brew kurulu değil",
	"moves the app and its leftovers to the Trash": "uygulamayı ve artıklarını Çöp'e taşır",
	"moves the app to the Trash":                   "uygulamayı Çöp'e taşır",
	"owned by root: moved to the Trash with sudo":  "root'a ait: sudo ile Çöp'e taşınır",
	"the folder you are browsing":                  "gezdiğiniz klasörün kendisi",
	"outside your home folder":                     "ev klasörünüzün dışında",
	"a system part of ~/Library":                   "~/Library'nin sistem parçası",
	"protected by macOS":                           "macOS tarafından korunuyor",
	"app is gone":                                  "uygulaması silinmiş",
	"runtime no longer installed":                  "runtime'ı artık kurulu değil",
	"never booted":                                 "hiç açılmamış",
	"booted recently":                              "yakın zamanda açılmış",
	"running — quit it first":                      "çalışıyor — önce kapatın",
	"not an absolute path":                         "mutlak yol değil",
	"does not exist":                               "yok",
	"cannot resolve parent":                        "üst klasör çözülemedi",
	"home or root directory":                       "ev veya kök klasör",
	"outside allowed roots":                        "izin verilen klasörlerin dışında",
	"protected path":                               "korumalı yol",
	"is an allowed root itself":                    "taranan kök klasörün kendisi",
	"environment file":                             "ortam dosyası (.env)",
	"tracked by git":                               "git'te takip ediliyor",
	"excluded in config":                           "ayarlarda hariç tutulmuş",
	"a process is running in this project":         "bu projede çalışan bir süreç var",
	"newest version, kept":                         "en yeni sürüm, korunuyor",
	"nvm default":                                  "nvm varsayılanı",
	"rustup default":                               "rustup varsayılanı",
	"app size not included":                        "uygulama boyutu dahil değil",
	"used by a container (stopped or running)":     "bir container kullanıyor (durmuş veya çalışan)",
	"root-owned; if installed with Homebrew, uninstall the cask instead": "root'a ait; Homebrew ile kurulduysa cask'ı kaldırın",
	"not deletable (bundled with Xcode)":                                 "silinemez (Xcode ile geliyor)",
}

// reasonPrefixesTR translates reasons that carry a value, e.g. "in use (SingletonLock)".
var reasonPrefixesTR = [][2]string{
	{"project changed in the last ", "son "},
	{"app is running ", "uygulama açık "},
	{"in use ", "kullanımda "},
	{"inside ", "içinde: "},
	{"installed by Homebrew cask ", "Homebrew cask'ı ile kuruldu: "},
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
