//go:build !linux

package procname

func Set(string) error {
	return nil
}
