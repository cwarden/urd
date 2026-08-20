package remind

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Locations of the system timezone configuration consulted, in order, when Go
// can only tell us the local zone is "Local". Overridden by tests.
var (
	localZoneLink = "/etc/localtime"
	localZoneFile = "/etc/timezone"
)

// LocalZoneName returns the IANA name of loc, e.g. "America/Chicago", suitable
// for a reminder's TZ clause. Go names the zone it loads from /etc/localtime
// "Local" rather than after the zone itself, so fall back to the system's own
// timezone configuration to recover a name remind will accept. Returns an empty
// string when no name can be determined.
func LocalZoneName(loc *time.Location) string {
	if loc != nil {
		if name := loc.String(); name != "" && name != "Local" {
			return name
		}
	}

	if name := strings.TrimPrefix(os.Getenv("TZ"), ":"); isZoneName(name) {
		return name
	}

	// /etc/localtime is usually a symlink such as
	// /usr/share/zoneinfo/America/Chicago
	if target, err := os.Readlink(localZoneLink); err == nil {
		if name := zoneNameFromPath(target); isZoneName(name) {
			return name
		}
	}

	if contents, err := os.ReadFile(localZoneFile); err == nil {
		if name := strings.TrimSpace(string(contents)); isZoneName(name) {
			return name
		}
	}

	return ""
}

// zoneNameFromPath extracts the zone name from a path into the zoneinfo
// database, e.g. "/usr/share/zoneinfo/America/Chicago" -> "America/Chicago".
func zoneNameFromPath(path string) string {
	path = filepath.ToSlash(path)
	if i := strings.Index(path, "zoneinfo/"); i >= 0 {
		return path[i+len("zoneinfo/"):]
	}
	return ""
}

// isZoneName reports whether name looks like a usable zone name. Names remind
// can't resolve are worse than no TZ clause at all, so reject the placeholders
// Go and the system use for "whatever the machine is set to".
func isZoneName(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	return !strings.ContainsAny(name, " \t\n")
}
