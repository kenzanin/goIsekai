//go:build linux || darwin || freebsd || netbsd || openbsd

package pluginmanager

import "github.com/ebitengine/purego"

// nativeLib is a dlopen'd shared object (a C/C++ .so plugin). purego opens it
// through the platform loader directly, so the host keeps building
// CGO_ENABLED=0 — which stdlib `plugin` could never do.
type nativeLib struct{ handle uintptr }

func openNativeLib(path string) (*nativeLib, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	return &nativeLib{handle: h}, nil
}

// bind resolves name and wires it to fptr, which must be a pointer to a
// function variable. Resolving first keeps an absent optional symbol (GetGenres)
// an ordinary error instead of the panic RegisterLibFunc raises on a miss.
func (l *nativeLib) bind(name string, fptr any) error {
	if _, err := purego.Dlsym(l.handle, name); err != nil {
		return err
	}
	purego.RegisterLibFunc(fptr, l.handle, name)
	return nil
}

func (l *nativeLib) close() error { return purego.Dlclose(l.handle) }
