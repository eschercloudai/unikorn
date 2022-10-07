package constants

import (
	"fmt"
	"os"
	"path"
)

var (
	// Application is the application name.
	Application = path.Base(os.Args[0])

	// Version is the application version set via the Makefile.
	Version string

	// Revision is the git revision set via the Makefile.
	Revision string
)

// VersionString returns a canonical version string.  It's based on
// HTTP's User-Agent so can be used to set that too, if this ever has to
// call out ot other micro services.
func VersionString() string {
	return fmt.Sprintf("%s/%s (revision/%s)", Application, Version, Revision)
}
