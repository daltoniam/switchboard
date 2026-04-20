package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return Version
}

func Full() string {
	return Version + " (commit: " + Commit + ", built: " + Date + ")"
}
