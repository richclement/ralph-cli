//go:build windows

package agent

func isPTYEOF(err error) bool {
	return false
}
