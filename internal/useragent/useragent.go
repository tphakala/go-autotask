// Package useragent builds the default User-Agent from the build info. The
// client and the middleware package share it, so every request sends the same
// default.
package useragent

import (
	"runtime/debug"
	"sync"
)

const (
	// modulePath is this module's path as the build info records it.
	modulePath = "github.com/tphakala/go-autotask"
	// userAgentProduct is the User-Agent product token.
	userAgentProduct = "go-autotask"
)

// defaultValue reads the build info once per process.
var defaultValue = sync.OnceValue(func() string {
	info, _ := debug.ReadBuildInfo()
	return For(info)
})

// Default returns the User-Agent go-autotask sends unless the caller sets
// another, such as with autotask.WithUserAgent.
func Default() string { return defaultValue() }

// For returns "go-autotask/<version>" when info gives this module a
// version, and "go-autotask" when it does not. A local replace directive, a
// workspace checkout and the module's own tests leave the version as "(devel)",
// and then no version is claimed.
func For(info *debug.BuildInfo) string {
	if v := moduleVersion(info); v != "" {
		return userAgentProduct + "/" + v
	}
	return userAgentProduct
}

// moduleVersion returns this module's version without the leading "v", or ""
// when info holds none. The module is the main module when it is this one, else
// the dependency with this module's path. A replacement decides when there is
// one, because the replaced entry's version does not describe the code that
// runs.
func moduleVersion(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	mod := &info.Main
	if mod.Path != modulePath {
		mod = nil
		for _, dep := range info.Deps {
			if dep.Path == modulePath {
				mod = dep
				break
			}
		}
	}
	if mod == nil {
		return ""
	}
	if mod.Replace != nil {
		mod = mod.Replace
	}
	if !isModuleVersion(mod.Version) {
		return ""
	}
	return mod.Version[1:]
}

// isModuleVersion reports whether v is "v", a digit, then only ASCII letters,
// digits, '.', '-' and '+'. That excludes "" and "(devel)" and keeps the result
// a valid User-Agent product token.
func isModuleVersion(v string) bool {
	if len(v) < 2 || v[0] != 'v' || v[1] < '0' || v[1] > '9' {
		return false
	}
	for i := 2; i < len(v); i++ {
		c := v[i]
		ok := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '-' || c == '+'
		if !ok {
			return false
		}
	}
	return true
}
