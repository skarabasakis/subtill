//go:build !windows

package console

// PrepareHeadless is a no-op on non-Windows platforms.
func PrepareHeadless() error {
	return nil
}
