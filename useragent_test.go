package autotask

import (
	"runtime/debug"
	"testing"
)

func TestUserAgentFor(t *testing.T) {
	t.Parallel()
	const self = "github.com/tphakala/go-autotask"
	dep := func(version string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main: debug.Module{Path: "example.com/app", Version: "v9.9.9"},
			Deps: []*debug.Module{
				{Path: "golang.org/x/time", Version: "v0.1.0"},
				{Path: self, Version: version, Replace: replace},
			},
		}
	}
	main := func(version string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: self, Version: version}}
	}
	for _, tt := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"nil info", nil, "go-autotask"},
		{"dependency version, not the consumer's", dep("v1.5.1", nil), "go-autotask/1.5.1"},
		{"local replace has no version", dep("v1.5.1", &debug.Module{Path: "../go-autotask", Version: "(devel)"}), "go-autotask"},
		{"replace version decides", dep("v1.5.1", &debug.Module{Path: "example.com/fork", Version: "v1.7.0"}), "go-autotask/1.7.0"},
		{"workspace checkout", dep("(devel)", nil), "go-autotask"},
		{"other main module, no dependency entry", &debug.BuildInfo{Main: debug.Module{Path: "example.com/app", Version: "v1.0.0"}}, "go-autotask"},
		{"main module in a test binary", main("(devel)"), "go-autotask"},
		{"main module pseudo-version", main("v1.5.2-0.20261118121938-dc337645bbfd+dirty"), "go-autotask/1.5.2-0.20261118121938-dc337645bbfd+dirty"},
		{"main module with no version", main(""), "go-autotask"},
		{"version with characters outside a module version", dep("v1.5.1 (devel)", nil), "go-autotask"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := userAgentFor(tt.info); got != tt.want {
				t.Errorf("userAgentFor = %q; want %q", got, tt.want)
			}
		})
	}
}
