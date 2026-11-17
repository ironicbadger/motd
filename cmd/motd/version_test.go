package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionString(t *testing.T) {
	revision := debug.BuildSetting{Key: "vcs.revision", Value: "a36bae889d942caca8d8e4b0d288aaae07931f46"}
	clean := &debug.BuildInfo{Settings: []debug.BuildSetting{revision}}
	dirty := &debug.BuildInfo{Settings: []debug.BuildSetting{revision, {Key: "vcs.modified", Value: "true"}}}
	installed := &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}}
	for _, tc := range []struct {
		name string
		tag  string
		info *debug.BuildInfo
		want string
	}{
		{"release tag", "v0.3.0", dirty, "v0.3.0"},
		{"module install", "", installed, "v0.3.0"},
		{"clean checkout", "", clean, "devel+a36bae889d94"},
		{"dirty checkout", "", dirty, "devel+a36bae889d94.dirty"},
		{"source archive", "", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "devel"},
		{"missing build info", "", nil, "devel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionString(tc.tag, tc.info); got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}
