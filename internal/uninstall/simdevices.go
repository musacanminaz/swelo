package uninstall

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/musacanminaz/swelo/internal/clean"
)

// simIdle: a simulator device not booted for this long is offered first.
const simIdle = 90 * 24 * time.Hour

// simDevices lists the simulator devices (iPhone 15, iPad…) with the size
// of their data and when they were last booted. Deleting one frees its
// data; Xcode recreates devices in a click.
func simDevices(e *Env) []Package {
	if !e.Look("xcrun") {
		return nil
	}
	js, err := e.Run("xcrun", "simctl", "list", "devices", "-j")
	if err != nil {
		return nil
	}
	var list struct {
		Devices map[string][]struct {
			Name, UDID, State, DataPath string
			IsAvailable                 bool
			LastBootedAt                *time.Time
		}
	}
	if json.Unmarshal([]byte(js), &list) != nil {
		return nil
	}
	var out []Package
	for runtime, devs := range list.Devices {
		// com.apple.CoreSimulator.SimRuntime.iOS-26-5 -> iOS 26.5
		os := runtime[strings.LastIndex(runtime, ".")+1:]
		if plat, ver, ok := strings.Cut(os, "-"); ok {
			os = plat + " " + strings.ReplaceAll(ver, "-", ".")
		}
		for _, d := range devs {
			p := Package{Provider: "simdevice", ID: "simdevice:" + d.UDID, Name: d.Name + " (" + os + ")",
				Size: clean.DirSize(d.DataPath), Command: []string{"xcrun", "simctl", "delete", d.UDID}}
			switch {
			case d.State != "Shutdown":
				p.Note = "running — quit it first"
				p.Command = nil
			case !d.IsAvailable:
				p.Note = "runtime no longer installed"
			case d.LastBootedAt == nil:
				p.Note = "never booted"
			default:
				p.LastUsed = *d.LastBootedAt
				if time.Since(*d.LastBootedAt) < simIdle {
					p.Note = "booted recently" // shown as a warning before deleting
				}
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}
