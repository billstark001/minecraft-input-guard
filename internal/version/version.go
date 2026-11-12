package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionFile string

func Number() string {
	return strings.TrimSpace(versionFile)
}
