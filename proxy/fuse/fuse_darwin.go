package fuse

import (
	"os"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/logging"
)

const (
	macfusePath = "/Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse"
	osxfusePath = "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse"
)

// Supported checks if macfuse or osxfuse are installed on the host by looking
// for both in their known installation location.
func Supported() bool {
	// check for macfuse first (newer version of osxfuse)
	if _, err := os.Stat(macfusePath); err != nil {
		// if that fails, check for osxfuse next
		if _, err := os.Stat(osxfusePath); err != nil {
			logging.Errorf(`FUSE support on macOS depends on osxfuse or macfuse.
Neither were found on your machine. For installation instructions,
see https://osxfuse.github.io.`)
			return false
		}
	}
	return true
}
