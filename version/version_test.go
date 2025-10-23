package version

import (
	"testing"

	"github.com/Masterminds/semver"
)

func TestVersion(t *testing.T) {
	ver, err := semver.NewVersion("v0.0.1-47-g891d6a0")
	if err != nil {
		t.Error(err)
	}

	t.Log(ver.String())
	t.Log(ver.String())
	t.Log(ver.String())
	t.Log(ver.String())
}
