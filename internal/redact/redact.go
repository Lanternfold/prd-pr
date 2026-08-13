package redact

import "regexp"

var (
	keyed  = regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password|passwd|cursor_api_key)\s*[:=]\s*)\S+`)
	auth   = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)\S+(?:\s+\S+)?`)
	bearer = regexp.MustCompile(`(?i)(bearer\s+)\S+`)
	ghp    = regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`)
	gpat   = regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)
	sk     = regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)
)

// String removes likely secret values from logs and transcripts.
func String(s string) string {
	s = keyed.ReplaceAllString(s, "${1}***")
	s = auth.ReplaceAllString(s, "${1}***")
	s = bearer.ReplaceAllString(s, "${1}***")
	s = ghp.ReplaceAllString(s, "ghp_***")
	s = gpat.ReplaceAllString(s, "github_pat_***")
	s = sk.ReplaceAllString(s, "sk-***")
	return s
}
