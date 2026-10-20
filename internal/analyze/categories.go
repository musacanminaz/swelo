package analyze

import (
	"path/filepath"
	"strings"
)

// Categories returns swelo's default categories with ~ expanded. Projects are
// the configured project roots.
func Categories(home string, projectRoots []string) []Category {
	h := func(ps ...string) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
		return out
	}
	lib := "~/Library/"
	return []Category{
		{"projects", projectRoots},
		{"xcode", h(lib + "Developer")},
		{"android", h(lib+"Android", "~/.android")},
		{"pkg-caches", h("~/.npm", lib+"pnpm", "~/.gradle", "~/.pub-cache", "~/go/pkg", "~/.cargo", "~/.m2",
			"~/.bun", "~/.cocoapods", "~/.cache", lib+"Caches/Yarn", lib+"Caches/pip", lib+"Caches/CocoaPods",
			lib+"Caches/Homebrew", lib+"Caches/ms-playwright", lib+"Caches/ms-playwright-mcp", lib+"Caches/go-build")},
		{"runtimes", h("~/.nvm", "~/.pyenv", "~/.rbenv", "~/.rustup", "~/.sdkman", "~/fvm", "~/.volta",
			"~/.local/share/uv", "~/.deno", "~/flutter", "~/development/flutter")},
		{"ai-models", h("~/.ollama", "~/.cache/huggingface", "~/.lmstudio", lib+"Application Support/Claude")},
		{"browsers", h(lib+"Application Support/Google", lib+"Application Support/BraveSoftware",
			lib+"Application Support/Microsoft Edge", lib+"Application Support/Firefox", lib+"Application Support/Arc",
			lib+"Caches/Google", lib+"Caches/BraveSoftware", lib+"Caches/Microsoft Edge", lib+"Caches/Firefox",
			lib+"Caches/company.thebrowser.Browser")},
		{"editors", h(lib+"Application Support/Code", lib+"Application Support/Cursor",
			lib+"Application Support/JetBrains", lib+"Caches/JetBrains", "~/.vscode", "~/.cursor")},
		{"messaging", h(lib+"Group Containers/group.net.whatsapp.WhatsApp.shared",
			lib+"Application Support/Slack", lib+"Application Support/discord", lib+"Application Support/Microsoft Teams",
			lib+"Containers/com.microsoft.teams2", lib+"Containers/net.whatsapp.WhatsApp")},
		{"docker", h(lib+"Containers/com.docker.docker", lib+"Group Containers/group.com.docker", "~/.docker")},
		{"app-caches", h(lib + "Caches")},
		{"library", h(lib[:len(lib)-1])},
		{"downloads", h("~/Downloads")},
		{"trash", h("~/.Trash")},
		{"media", h("~/Documents", "~/Pictures", "~/Movies", "~/Music")},
	}
}
