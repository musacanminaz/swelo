package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every i18n.T call with a literal key must have a message.
func TestAllKeysDefined(t *testing.T) {
	re := regexp.MustCompile(`i18n\.T\("([^"]+)"\)`)
	filepath.WalkDir("../..", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "keys_test.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !Has(m[1]) {
				t.Errorf("%s: missing message %q", p, m[1])
			}
		}
		return nil
	})
}
