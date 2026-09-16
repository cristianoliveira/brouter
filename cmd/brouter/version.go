package main

import "runtime/debug"

// version is replaced by release builds with the validated Git tag version.
// Local builds identify themselves with the full embedded VCS revision when
// Go build metadata provides one.
var version = "dev"

var readBuildInfo = debug.ReadBuildInfo

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := readBuildInfo()
	return developmentVersion(info, ok)
}

func developmentVersion(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "dev"
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
	if revision == "" {
		return "dev"
	}

	resolved := "dev-" + revision
	if modified {
		resolved += "-dirty"
	}
	return resolved
}
