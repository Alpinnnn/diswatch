package version

// Version information populated at build time via -ldflags
// Example: go build -ldflags "-X diswatch/internal/version.version=1.2.3"

var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

func String() string {
	if Version == "dev" {
		return "Diswatch (development build)"
	}
	return "Diswatch v" + Version
}

func FullString() string {
	info := "Diswatch v" + Version
	if Commit != "" {
		info += " (" + Commit + ")"
	}
	if BuildTime != "" {
		info += " built at " + BuildTime
	}
	return info
}
