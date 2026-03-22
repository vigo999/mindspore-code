package version

// Version is injected at build time via -ldflags.
// Dev builds keep the default value and skip update checks.
var Version = "dev"
