package buildflow

import (
	"fmt"
	"strings"
)

type Target struct{ OS, Arch string }

func ParseTarget(value string) (Target, error) {
	allowed := map[string]bool{
		"windows/amd64": true, "windows/arm64": true,
		"linux/amd64": true, "linux/arm64": true,
		"darwin/amd64": true, "darwin/arm64": true, "darwin/universal": true,
	}
	if !allowed[value] {
		return Target{}, fmt.Errorf("unsupported desktop target %q", value)
	}
	os, arch, ok := strings.Cut(value, "/")
	if !ok {
		return Target{}, fmt.Errorf("invalid desktop target %q", value)
	}
	return Target{OS: os, Arch: arch}, nil
}

func (t Target) String() string    { return t.OS + "/" + t.Arch }
func (t Target) Directory() string { return t.OS + "-" + t.Arch }
