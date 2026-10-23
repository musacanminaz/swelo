package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
)

func TestHelpFollowsLanguage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := earlyLang([]string{"clean", "--lang=en"}); got != "en" {
		t.Fatalf("earlyLang = %q", got)
	}
	if got := earlyLang([]string{"--lang", "tr", "clean"}); got != "tr" {
		t.Fatalf("earlyLang = %q", got)
	}
	i18n.Set("tr")
	defer i18n.Set("en")
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"clean", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Kullanım:", "Seçenekler:", "Genel seçenekler:", "listelenenleri sil", "bu komut için yardım", "çıktı dili"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "help for") || strings.Contains(out.String(), "Usage:") {
		t.Fatalf("English left in:\n%s", out.String())
	}
}
