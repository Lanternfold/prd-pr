package cli

import (
	"runtime/debug"
	"strings"
)

func currentVersion() string {
	info, _ := debug.ReadBuildInfo()
	return resolveVersion(Version, info)
}

func resolveVersion(ldflags string, info *debug.BuildInfo) string {
	if v := strings.TrimSpace(ldflags); v != "" && v != "dev" {
		return v
	}
	if info == nil {
		return "dev"
	}
	v := strings.TrimSpace(info.Main.Version)
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}
