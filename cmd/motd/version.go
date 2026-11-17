package main

import "runtime/debug"

// The release workflow sets releaseVersion from the tag with the Go linker.
var releaseVersion string

func versionString(tag string, info *debug.BuildInfo) string {
	if tag != "" {
		return tag
	}
	if info == nil {
		return "devel"
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision != "" {
		version := "devel+" + revision[:min(len(revision), 12)]
		if modified {
			version += ".dirty"
		}
		return version
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}
