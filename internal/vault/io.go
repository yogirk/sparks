package vault

import "bytes"

// bytesReader wraps a byte slice as an io.Reader without pulling in the
// standard library's strings package. Kept tiny so vault.go stays focused
// on the vault concept.
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
