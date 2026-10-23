package uninstall

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSimDevices(t *testing.T) {
	home := t.TempDir()
	data := func(u string) string { return filepath.Join(home, "Devices", u, "data") }
	mkfile(t, filepath.Join(data("OLD"), "f"), strings.Repeat("x", 4096))
	mkfile(t, filepath.Join(data("NEW"), "f"), "x")
	old := time.Now().Add(-120 * 24 * time.Hour).UTC().Format(time.RFC3339)
	recent := time.Now().Add(-5 * 24 * time.Hour).UTC().Format(time.RFC3339)
	js := `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-26-5":[
		{"name":"iPhone 17 Pro","udid":"OLD","state":"Shutdown","isAvailable":true,"lastBootedAt":"` + old + `","dataPath":"` + data("OLD") + `"},
		{"name":"iPhone 17e","udid":"NEW","state":"Shutdown","isAvailable":true,"lastBootedAt":"` + recent + `","dataPath":"` + data("NEW") + `"},
		{"name":"iPad","udid":"NEVER","state":"Shutdown","isAvailable":true,"dataPath":"` + data("NEVER") + `"},
		{"name":"Booted","udid":"ON","state":"Booted","isAvailable":true,"dataPath":"` + data("ON") + `"}]}}`
	e := &Env{Home: home, Look: func(string) bool { return true }, Run: func(argv ...string) (string, error) { return js, nil }}
	ps := byID(simDevices(e))
	if len(ps) != 4 {
		t.Fatalf("%d devices", len(ps))
	}
	o := ps["simdevice:OLD"]
	if o.Name != "iPhone 17 Pro (iOS 26.5)" || o.Note != "" || o.Size < 4096 || o.Command[2] != "delete" || o.LastUsed.IsZero() {
		t.Fatalf("old: %+v", o)
	}
	if n := ps["simdevice:NEW"]; n.Note != "booted recently" {
		t.Fatalf("new: %+v", n)
	}
	if n := ps["simdevice:NEVER"]; n.Note != "never booted" {
		t.Fatalf("never: %+v", n)
	}
	if b := ps["simdevice:ON"]; b.Command != nil || b.Note != "running — quit it first" {
		t.Fatalf("booted: %+v", b)
	}
}
