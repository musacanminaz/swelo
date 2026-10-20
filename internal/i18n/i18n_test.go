package i18n

import "testing"

func TestReasonTR(t *testing.T) {
	Set("tr")
	defer Set("en")
	for in, want := range map[string]string{
		"tracked by git":                     "git'te takip ediliyor",
		"project changed in the last 7 days": "son 7 günde değişmiş proje",
		"in use (SingletonLock)":             "kullanımda (SingletonLock)",
		"something new":                      "something new",
	} {
		if got := Reason(in); got != want {
			t.Errorf("Reason(%q) = %q, want %q", in, got, want)
		}
	}
}
