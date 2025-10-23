package version

import (
	"fmt"

	"github.com/Masterminds/semver"
)

// Write when compiling
var (
	v *semver.Version

	// Version should be updated by hand at each release
	Version string

	//will be overwritten automatically by the build system
	GitCommit string
	GoVersion string
	BuildTime string
)

type Info struct {
	Version   string
	GitCommit string
	GoVersion string
	BuildTime string
}

// FullVersion formats the version to be printed
func FullVersion() string {
	return fmt.Sprintf("Version: %6s \nGit commit: %6s \nGo version: %6s \nBuild time: %6s",
		Version, GitCommit, GoVersion, BuildTime)
}

func SimpleVersion() string {
	if v == nil {
		if Version == "" {
			return ""
		}
		ver, err := semver.NewVersion(Version)
		if err != nil {
			return ""
		}
		v = ver
	}

	return fmt.Sprintf("v%d.%d.%d", v.Major(), v.Minor(), v.Patch())
}

func GetVersionInfo() *Info {
	return &Info{
		Version:   Version,
		GitCommit: GitCommit,
		GoVersion: GoVersion,
		BuildTime: BuildTime,
	}
}
