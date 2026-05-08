//go:build !darwin

package sessions

// readKeychainCredentials returns nil on non-macOS platforms.
//
// Python's _read_keychain_credentials does the same: gates on
// `platform.system() != "Darwin"` and returns None.
func readKeychainCredentials() []byte {
	return nil
}
