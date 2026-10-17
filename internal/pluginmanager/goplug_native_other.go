//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package pluginmanager

import "errors"

// nativeLib is unavailable where the platform loader purego needs is absent.
// The type still exists so every other file compiles; opening a .so plugin
// fails with a clear message instead of a build break.
type nativeLib struct{ handle uintptr }

func openNativeLib(string) (*nativeLib, error) {
	return nil, errors.New("native .so plugins are not supported on this platform")
}

func (l *nativeLib) bind(string, any) error {
	return errors.New("native .so plugins are not supported on this platform")
}

func (l *nativeLib) close() error { return nil }
