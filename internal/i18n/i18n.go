// Package i18n holds swelo's Turkish and English messages.
package i18n

import (
	"os"
	"os/exec"
	"strings"
)

var lang = detect()

// Set forces the language ("tr" or "en"); anything else is ignored.
func Set(l string) {
	if l == "tr" || l == "en" {
		lang = l
	}
}

// Has reports whether key has a message.
func Has(key string) bool { _, ok := msgs[key]; return ok }

// Lang returns the active language.
func Lang() string { return lang }

// T returns the message for key in the active language, falling back to English.
func T(key string) string {
	m, ok := msgs[key]
	if !ok {
		return key
	}
	if lang == "tr" && m[1] != "" {
		return m[1]
	}
	return m[0]
}

func detect() string {
	for _, v := range []string{os.Getenv("SWELO_LANG"), os.Getenv("LC_ALL"), os.Getenv("LANG")} {
		if strings.HasPrefix(v, "tr") {
			return "tr"
		}
		if strings.HasPrefix(v, "en") {
			return "en"
		}
	}
	// macOS preferred language, e.g. "(\n    \"tr-TR\",\n ..."
	if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		fields := strings.Fields(strings.Trim(string(out), "()\n "))
		if len(fields) > 0 && strings.HasPrefix(strings.Trim(fields[0], "\","), "tr") {
			return "tr"
		}
	}
	return "en"
}

// msgs: key -> {English, Turkish}
var msgs = map[string][2]string{
	"tagline": {"disk care for developer Macs", "developer Mac'leri için disk bakımı"},

	"what.title": {"What swelo does", "swelo ne yapar?"},
	"what.body": {
		"Finds files that pile up during development and can be regenerated:\n" +
			"node_modules, build folders, package manager caches, Docker leftovers,\n" +
			"old simulators and SDKs. Shows how much space they take; deletes only\n" +
			"what you approve.",
		"Geliştirme sırasında biriken ve yeniden oluşturulabilen dosyaları bulur:\n" +
			"node_modules, build klasörleri, paket yöneticisi cache'leri, Docker artıkları,\n" +
			"eski simülatörler ve SDK'lar. Ne kadar yer kapladıklarını gösterir; sadece\n" +
			"sizin onayladıklarınızı siler.",
	},
	"never.title": {"What swelo never does", "swelo ne yapmaz?"},
	"never.1":     {"Touch your source code, git-tracked files or documents", "Kaynak kodunuza, git'te takip edilen dosyalara, belgelerinize dokunmaz"},
	"never.2":     {"Delete Docker volumes, databases or browser profiles", "Docker volume'larını, veritabanlarını, tarayıcı profillerini silmez"},
	"never.3":     {"Touch projects you changed in the last 7 days", "Son 7 günde değiştirdiğiniz projelere dokunmaz"},
	"never.4":     {"Run sudo without asking, or send any data over the network", "Sizden habersiz sudo çalıştırmaz, internete veri göndermez"},
	"never.5":     {"Delete anything without --apply and your confirmation", "--apply ve onayınız olmadan hiçbir şey silmez"},

	"perm.title":      {"Permissions", "Yetki durumu"},
	"perm.user":       {"Home folder access", "Ev klasörü erişimi"},
	"perm.user.ok":    {"normal user rights", "normal kullanıcı yetkisi"},
	"perm.fda":        {"Full Disk Access", "Full Disk Access"},
	"perm.fda.ok":     {"granted to this terminal: protected app folders can be measured", "bu terminalde açık: korumalı uygulama klasörleri ölçülebilir"},
	"perm.fda.no":     {"not granted: Mail/Safari/Messages folders are skipped (optional)", "kapalı: Mail/Safari/Messages klasörleri atlanır (opsiyonel)"},
	"perm.sudo":       {"Administrator (sudo)", "Yönetici (sudo)"},
	"perm.sudo.admin": {"not needed by default; asked step by step, full command shown", "varsayılan olarak gerekmez; adım adım, komutun tamamı gösterilerek sorulur"},
	"perm.sudo.no":    {"this user is not an admin: steps that need sudo will be skipped", "bu kullanıcı yönetici değil: sudo isteyen adımlar atlanır"},
	"perm.more":       {"Details: swelo permissions", "Ayrıntılar: swelo permissions"},

	"cmds.title":      {"Commands", "Komutlar"},
	"cmd.analyze":     {"Show where disk space went (deletes nothing)", "Disk nereye gitmiş, göster (hiçbir şey silmez)"},
	"cmd.clean":       {"Clean regenerable files (dry run unless --apply)", "Yeniden oluşturulabilir dosyaları temizle (--apply olmadan sadece liste)"},
	"cmd.optimize":    {"Spotlight / Time Machine / Docker / git maintenance", "Spotlight / Time Machine / Docker / git bakımı"},
	"cmd.uninstall":   {"Remove apps and developer toolchains", "Uygulama ve geliştirme paketlerini kaldır"},
	"cmd.status":      {"Disk status and history", "Disk durumu ve geçmiş"},
	"cmd.permissions": {"Explain every permission swelo may use", "swelo'nun kullanabileceği tüm yetkileri açıkla"},
	"soon":            {"(coming soon)", "(yakında)"},

	"permdoc.intro": {
		"swelo runs as your normal user. It never stores your password and never keeps a sudo\n" +
			"session open. Every step that needs more than normal rights is listed below.",
		"swelo normal kullanıcınızla çalışır. Şifrenizi saklamaz, sudo oturumunu açık bırakmaz.\n" +
			"Normal yetkinin ötesine geçen her adım aşağıda listelenir.",
	},
	"permdoc.user": {
		"Reads folder sizes and marker files (package.json, pubspec.yaml) under your home,\n" +
			"runs `git ls-files` to skip tracked files, and deletes approved items there.",
		"Ev klasörünüz altında klasör boyutlarını ve işaret dosyalarını (package.json, pubspec.yaml)\n" +
			"okur, git'te takipli dosyaları atlamak için `git ls-files` çalıştırır, onaylananları siler.",
	},
	"permdoc.fda": {
		"Only used by `swelo analyze --full` to measure ~/Library/Containers, Mail, Safari, Messages.\n" +
			"Without it those folders are reported as \"not accessible\"; everything else works.\n" +
			"To grant: System Settings → Privacy & Security → Full Disk Access → enable your terminal.",
		"Sadece `swelo analyze --full`, ~/Library/Containers, Mail, Safari, Messages boyutlarını ölçmek\n" +
			"için kullanır. Yoksa bu klasörler \"erişilemedi\" olarak gösterilir; geri kalan her şey çalışır.\n" +
			"Açmak için: Sistem Ayarları → Gizlilik ve Güvenlik → Tam Disk Erişimi → terminalinizi açın.",
	},
	"permdoc.sudo": {
		"Needed only for: /Library/Caches, system simulator volumes, JDKs in /Library/Java,\n" +
			"DNS cache flush, system-wide Time Machine exclusions. Before each one swelo shows the\n" +
			"exact command, why it needs root and whether it can be undone, then asks [y/N].\n" +
			"--yes never approves sudo steps; --no-sudo skips all of them.",
		"Sadece şunlar için gerekir: /Library/Caches, sistem simülatör volume'ları, /Library/Java\n" +
			"altındaki JDK'lar, DNS cache temizliği, sistem geneli Time Machine dışlamaları. Her birinden\n" +
			"önce swelo çalışacak komutun tamamını, neden root gerektiğini ve geri alınıp alınamayacağını\n" +
			"gösterir, sonra [e/H] diye sorar. --yes sudo adımlarını onaylamaz; --no-sudo hepsini atlar.",
	},
	"permdoc.net": {
		"No telemetry. The network is used only by `swelo update` to check for a new release.",
		"Telemetri yok. Ağ sadece `swelo update` ile yeni sürüm kontrolünde kullanılır.",
	},
	"permdoc.files": {"Files swelo writes", "swelo'nun yazdığı dosyalar"},

	"plan.before": {"before it starts", "çalıştırmadan önce"},
	"plan.scan":   {"Will scan", "Taranacak"},
	"plan.read":   {"Will read", "Okunacak"},
	"plan.delete": {"Will delete", "Silinecek"},
	"plan.perm":   {"Permissions", "Yetki"},
	"plan.time":   {"Takes about", "Tahmini süre"},
	"confirm.yn":  {"[y/N]", "[e/H]"},
	"confirm.Yn":  {"[Y/n]", "[E/h]"},
	"continue":    {"Continue?", "Devam?"},

	"intro.first":  {"First run: here is what swelo does before it scans anything.", "İlk çalıştırma: swelo bir şey taramadan önce ne yaptığını anlatıyor."},
	"intro.config": {"Settings file", "Ayar dosyası"},
	"intro.enter":  {"Press Enter to continue, Ctrl+C to quit.", "Devam etmek için Enter, çıkmak için Ctrl+C."},

	"sudo.title":        {"This step needs administrator rights", "Bu adım yönetici yetkisi istiyor"},
	"sudo.action":       {"Step", "Adım"},
	"sudo.reason":       {"Why", "Neden"},
	"sudo.command":      {"Command", "Komut"},
	"sudo.undo":         {"Undo", "Geri alma"},
	"sudo.noundo":       {"cannot be undone", "geri alınamaz"},
	"sudo.ask":          {"Run it? (your password is never seen or stored by swelo)", "Çalıştırılsın mı? (şifreniz swelo tarafından görülmez ve saklanmaz)"},
	"sudo.skipped":      {"Skipped.", "Atlandı."},
	"sudo.skipped.flag": {"Skipped (--no-sudo).", "Atlandı (--no-sudo)."},

	"plan.delete.dry":   {"nothing (dry run) — add --apply to delete", "hiçbir şey (dry-run) — silmek için --apply"},
	"plan.delete.apply": {"the items listed below, after you confirm", "aşağıda listelenenler, siz onayladıktan sonra"},
	"plan.read.clean":   {"folder sizes, marker files (package.json, pubspec.yaml…), git ls-files, running processes", "klasör boyutları, işaret dosyaları (package.json, pubspec.yaml…), git ls-files, çalışan işlemler"},
	"plan.perm.none":    {"normal user; no sudo", "normal kullanıcı; sudo yok"},
	"plan.time.clean":   {"10–60 s depending on project count", "proje sayısına göre 10–60 sn"},
	"scanning":          {"Scanning…", "Taranıyor…"},
	"tag.risky":         {"(needs --risky)", "(--risky gerekir)"},
	"tag.report":        {"(report only, never deleted)", "(sadece rapor, silinmez)"},
	"more.items":        {"… %d more (--all lists everything)", "… %d öğe daha (--all hepsini listeler)"},
	"skipped":           {"skipped", "atlandı"},
	"rebuild":           {"rebuild:", "yeniden oluşturma:"},
	"total.safe":        {"Can be freed:", "Açılabilir:"},
	"total.risky":       {"with --risky: +", "--risky ile: +"},
	"disk.free":         {"free now:", "şu an boş:"},
	"hint.apply":        {"Nothing was deleted. Run `swelo clean --apply` to delete the 🟢 items.", "Hiçbir şey silinmedi. 🟢 öğeleri silmek için: swelo clean --apply"},
	"confirm.delete":    {"Delete these items (%s)?", "Bu öğeler silinsin mi (%s)?"},
	"cancelled":         {"Cancelled, nothing deleted.", "İptal edildi, hiçbir şey silinmedi."},
	"done.freed":        {"Freed:", "Açılan yer:"},
	"done.failed":       {"%d items could not be deleted (see above).", "%d öğe silinemedi (yukarıya bakın)."},
	"done.log":          {"Every deleted path is logged in", "Silinen her yol şuraya yazıldı:"},

	"plan.read.analyze": {"file and folder sizes only — never file contents", "sadece dosya ve klasör boyutları — dosya içerikleri asla okunmaz"},
	"plan.delete.none":  {"nothing — analyze never deletes", "hiçbir şey — analyze asla silmez"},
	"plan.perm.analyze": {"normal user; folders macOS protects (Mail, Messages, app containers, Trash) are skipped — add --full to include them", "normal kullanıcı; macOS'un koruduğu klasörler (Mail, Messages, uygulama container'ları, Çöp) atlanır — dahil etmek için --full"},
	"plan.perm.full":    {"Full Disk Access (granted): protected folders are measured too", "Full Disk Access (açık): korumalı klasörler de ölçülür"},
	"plan.time.analyze": {"20–90 s", "20–90 sn"},
	"analyze.nofda":     {"--full needs Full Disk Access for your terminal (see `swelo permissions`); protected folders will be skipped.", "--full için terminalinize Full Disk Access gerekir (bkz. swelo permissions); korumalı klasörler atlanacak."},
	"disk":              {"Disk:", "Disk:"},
	"col.category":      {"Home folder", "Ev klasörü"},
	"col.size":          {"size", "boyut"},
	"col.freeable":      {"clean can free", "clean ile açılır"},
	"col.total":         {"total", "toplam"},
	"cat.projects":      {"Projects", "Projeler"},
	"cat.xcode":         {"Xcode & simulators", "Xcode ve simülatörler"},
	"cat.android":       {"Android SDK", "Android SDK"},
	"cat.pkg-caches":    {"Package manager caches", "Paket yöneticisi cache'leri"},
	"cat.runtimes":      {"Language runtimes (nvm, pyenv…)", "Dil sürümleri (nvm, pyenv…)"},
	"cat.ai-models":     {"AI models & apps", "AI modelleri ve uygulamaları"},
	"cat.browsers":      {"Browsers", "Tarayıcılar"},
	"cat.editors":       {"Editors (VS Code, Cursor, JetBrains)", "Editörler (VS Code, Cursor, JetBrains)"},
	"cat.messaging":     {"Messaging (WhatsApp, Slack…)", "Mesajlaşma (WhatsApp, Slack…)"},
	"cat.docker":        {"Docker", "Docker"},
	"cat.app-caches":    {"Other app caches", "Diğer uygulama cache'leri"},
	"cat.library":       {"Other ~/Library", "Diğer ~/Library"},
	"cat.downloads":     {"Downloads", "İndirilenler"},
	"cat.trash":         {"Trash", "Çöp Kutusu"},
	"cat.media":         {"Documents & media", "Belgeler ve medya"},
	"cat.other":         {"Everything else in ~", "~ içindeki diğer her şey"},
	"analyze.docker":    {"Docker disk image uses %s on disk (can grow to %s)", "Docker disk dosyası diskte %s kaplıyor (en fazla %s'a büyüyebilir)"},
	"analyze.skipped":   {"%d protected folders were not read (add --full with Full Disk Access to include them).", "%d korumalı klasör okunmadı (dahil etmek için Full Disk Access ile --full)."},
	"analyze.bigfiles":  {"Files over 1 GB", "1 GB'tan büyük dosyalar"},
	"analyze.projects":  {"Projects with the most regenerable data", "En çok yeniden oluşturulabilir verisi olan projeler"},
	"analyze.daysago":   {"changed %d days ago", "%d gün önce değişti"},
	"analyze.today":     {"changed today", "bugün değişti"},
	"more.top":          {"… %d more (--top N shows more)", "… %d öğe daha (--top N ile daha fazlası)"},
	"analyze.next":      {"Nothing was deleted. Next: `swelo clean` shows exactly what would be removed.", "Hiçbir şey silinmedi. Sonraki adım: `swelo clean` neyin silineceğini tam olarak gösterir."},

	"tree.help":      {"↑↓ move · enter/→ open · ←/backspace up · d move to Trash · q quit", "↑↓ gez · enter/→ aç · ←/backspace üst klasör · d Çöp'e taşı · q çık"},
	"tree.denied":    {"(not read)", "(okunmadı)"},
	"tree.small":     {"%d small files", "%d küçük dosya"},
	"tree.confirm":   {"Move %s (%s) to the Trash? [y/N]", "%s (%s) Çöp'e taşınsın mı? [e/H]"},
	"tree.cancelled": {"Cancelled.", "İptal edildi."},
	"tree.trashed":   {"Moved %s to the Trash (%s). Empty the Trash to free the space.", "%s Çöp'e taşındı (%s). Yer açmak için Çöp'ü boşaltın."},
	"tree.refused":   {"Not allowed here: %s.", "Burada izin yok: %s."},
	"tree.summary":   {"Moved %s to the Trash in this session.", "Bu oturumda Çöp'e taşınan: %s."},

	"plan.delete.tree": {"only what you select with d and confirm — moved to the Trash, not erased", "sadece d ile seçip onayladıklarınız — kalıcı silinmez, Çöp'e taşınır"},
	"tree.needtty":     {"the tree view needs an interactive terminal", "ağaç görünümü etkileşimli bir terminal gerektirir"},

	"plan.scan.uninstall":   {"Homebrew, nvm, pyenv, rustup, simulator runtimes, Android SDK, JDKs, global npm, Docker images, VS Code/Cursor extensions, Ollama", "Homebrew, nvm, pyenv, rustup, simülatör runtime'ları, Android SDK, JDK'lar, global npm, Docker image'ları, VS Code/Cursor eklentileri, Ollama"},
	"plan.read.uninstall":   {"each tool's own list command, folder sizes, version files in your projects (.nvmrc, .python-version, .tool-versions)", "her aracın kendi listeleme komutu, klasör boyutları, projelerinizdeki sürüm dosyaları (.nvmrc, .python-version, .tool-versions)"},
	"plan.delete.uninstall": {"nothing yet — this only lists", "henüz hiçbir şey — sadece listeler"},
	"plan.perm.uninstall":   {"normal user; removing a JDK later needs sudo and is asked separately", "normal kullanıcı; JDK kaldırmak ileride sudo ister ve ayrıca sorulur"},
	"plan.time.uninstall":   {"5–30 s", "5–30 sn"},
	"uninstall.next":        {"Nothing was removed. Removing is done with each tool's own uninstaller (shown before it runs).", "Hiçbir şey kaldırılmadı. Kaldırma, her aracın kendi kaldırma komutuyla yapılır (çalışmadan önce gösterilir)."},
	"prov.simruntime":       {"Simulator runtimes", "Simülatör runtime'ları"},
	"prov.android":          {"Android SDK packages", "Android SDK paketleri"},
	"prov.brew":             {"Homebrew (installed on request)", "Homebrew (isteyerek kurulanlar)"},
	"prov.node":             {"Node versions (nvm)", "Node sürümleri (nvm)"},
	"prov.python":           {"Python versions (pyenv)", "Python sürümleri (pyenv)"},
	"prov.rust":             {"Rust toolchains", "Rust toolchain'leri"},
	"prov.jdk":              {"Java JDKs", "Java JDK'ları"},
	"prov.npm-global":       {"Global npm packages", "Global npm paketleri"},
	"prov.docker-image":     {"Docker images", "Docker image'ları"},
	"prov.vscode":           {"Editor extensions", "Editör eklentileri"},
	"prov.ollama":           {"Ollama models", "Ollama modelleri"},
	"pkg.lastused":          {"last used %s", "son kullanım %s"},
	"pkg.installed":         {"installed/updated %s", "kurulum/güncelleme %s"},
	"pkg.usedby":            {"pinned by %s (%d projects)", "%s tarafından isteniyor (%d proje)"},
	"today":                 {"today", "bugün"},
	"days.ago":              {"%d days ago", "%d gün önce"},

	"pick.count":         {"%d selected", "%d seçili"},
	"pick.help":          {"↑↓ move · space select · enter continue · q cancel", "↑↓ gez · boşluk seç · enter devam · q iptal"},
	"pick.title":         {"Select what to uninstall", "Kaldırılacakları seçin"},
	"uninstall.review":   {"These commands will run, one by one:", "Şu komutlar sırayla çalışacak:"},
	"uninstall.warn":     {"⚠ %s", "⚠ %s"},
	"uninstall.confirm":  {"Uninstall %d items (%s)?", "%d öğe kaldırılsın mı (%s)?"},
	"uninstall.sudonote": {"(sudo — asked separately with the full command)", "(sudo — tam komutla ayrıca sorulur)"},
	"uninstall.done":     {"Removed %d of %d (%s).", "%d / %d kaldırıldı (%s)."},
	"uninstall.none":     {"Nothing selected.", "Hiçbir şey seçilmedi."},
	"uninstall.unknown":  {"unknown id: %s (run `swelo uninstall` to see ids)", "bilinmeyen id: %s (id'leri görmek için: swelo uninstall)"},
	"uninstall.needtty":  {"--pick needs an interactive terminal; use --remove <id,...> in scripts", "--pick etkileşimli terminal ister; script'lerde --remove <id,...> kullanın"},
	"uninstall.next2":    {"To remove: swelo uninstall --pick (choose interactively) or --remove <id,...>", "Kaldırmak için: swelo uninstall --pick (interaktif seçim) veya --remove <id,...>"},

	"plan.delete.uninstall.sel": {"only what you select, after you review every command and confirm", "sadece seçtikleriniz; her komutu görüp onayladıktan sonra"},
	"sudo.reason.jdk":           {"JDKs in /Library/Java are owned by root", "/Library/Java altındaki JDK'lar root'a ait"},
	"sudo.undo.jdk":             {"reinstall the JDK (e.g. brew install --cask zulu@17)", "JDK'yı yeniden kurun (ör. brew install --cask zulu@17)"},
}
