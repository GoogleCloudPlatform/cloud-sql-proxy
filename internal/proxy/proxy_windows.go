package proxy

import (
	"filepath"
	"strings"
)

// unixAddress returns the Unix socket for a given instance in the provided
// directory, by replacing all colons in the instance's name with periods.
func unixAddress(dir, inst string) string {
	inst2 := strings.ReplaceAll(inst.Name, ":", ".")
	return filepath.Join(dir, inst2)
}
