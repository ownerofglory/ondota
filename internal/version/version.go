// Package version exposes the build version injected at link time.
package version

// Version is set via -ldflags "-X github.com/ownerofglory/ondota/internal/version.Version=...".
var Version = "dev"
