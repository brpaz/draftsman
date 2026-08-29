package app

import "fmt"

// VersionInfo holds build-time metadata injected via ldflags.
type VersionInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// String returns a formatted version string.
func (v VersionInfo) String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", v.Version, v.Commit, v.BuildDate)
}
