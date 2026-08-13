package preflight

import "strings"

// knownBinaries maps declared dependency names to PATH binaries.
// Unknown names are never executed.
var knownBinaries = map[string]string{
	"git":           "git",
	"go":            "go",
	"gh":            "gh",
	"github":        "gh",
	"github cli":    "gh",
	"node":          "node",
	"npm":           "npm",
	"docker":        "docker",
	"xcodebuild":    "xcodebuild",
	"cursor-agent":  "cursor-agent",
	"cursor agent":  "cursor-agent",
	"agent":         "agent",
}

func binaryForDep(name string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "", false
	}
	bin, ok := knownBinaries[key]
	return bin, ok
}

func envVarForCredential(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}
	ok := true
	for _, r := range n {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		ok = false
		break
	}
	if ok && strings.Contains(n, "_") {
		return n
	}
	switch strings.ToLower(n) {
	case "github token", "github":
		return "GITHUB_TOKEN"
	case "cursor api key", "cursor":
		return "CURSOR_API_KEY"
	default:
		return ""
	}
}
