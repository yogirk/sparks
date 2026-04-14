package core

import "os"

// fileExists is intentionally tiny; we don't want to grow a generic util
// package. If two more helpers want to live here, that's the signal to
// reconsider.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
