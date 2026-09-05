package engine

import "runtime/debug"

var (
	Version = "0.0.0"
	Commit  = "unknown"
	BuildAt = "unknown"
)

func init() {
	if Version == "0.0.0" || Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					Commit = setting.Value
				case "vcs.time":
					BuildAt = setting.Value
				case "vcs.modified":
					if setting.Value == "true" {
						Commit += "-dirty"
					}
				}
			}
			Version = info.Main.Version
			if Version == "" {
				Version = "devel"
			}
		}
	}
}
