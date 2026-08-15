package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name    string
		ldflags string
		info    *debug.BuildInfo
		want    string
	}{
		{"local devel", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"nil info", "dev", nil, "dev"},
		{"empty module version", "dev", &debug.BuildInfo{}, "dev"},
		{"tagged go install", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "0.1.0"},
		{"unprefixed version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "0.1.0"}}, "0.1.0"},
		{"ldflags wins over devel", "0.1.0", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "0.1.0"},
		{"ldflags wins over other module version", "0.1.0", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, "0.1.0"},
		{"whitespace ldflags", "  ", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "0.1.0"},
		{"pseudo-version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1-0.20260815120000-abcdef123456"}}, "0.1.1-0.20260815120000-abcdef123456"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.ldflags, tc.info); got != tc.want {
				t.Fatalf("resolveVersion(%q) = %q, want %q", tc.ldflags, got, tc.want)
			}
		})
	}
}

func TestCurrentVersionLocalSourceIsDev(t *testing.T) {
	if Version != "dev" {
		t.Skip("ldflags overrode Version; this test is for source builds")
	}
	if got := currentVersion(); got != "dev" {
		t.Fatalf("currentVersion() = %q, want %q for a test/source build", got, "dev")
	}
}
