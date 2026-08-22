//go:build !windows

package service

import "os"

func relayHasRoot() bool {
	return os.Geteuid() == 0
}
