//go:build !windows

package client

func markHidden(path string) error {
	return nil
}
