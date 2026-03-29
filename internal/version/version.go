package version

// These variables are set at build time via -ldflags.
//
//	go build -ldflags "-X github.com/codested/chagg/internal/version.Version=0.11.0"
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Full returns a human-readable version string including commit and date
// when available.
func Full() string {
	s := Version
	if Commit != "unknown" {
		short := Commit
		if len(short) > 7 {
			short = short[:7]
		}
		s += " (" + short
		if Date != "unknown" {
			s += ", " + Date
		}
		s += ")"
	}
	return s
}
