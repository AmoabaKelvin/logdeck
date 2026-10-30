package redact

import "regexp"

// Builtin describes one built-in pattern for the settings UI.
type Builtin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Default marks the patterns offered as enabled before the settings are
	// first saved.
	Default bool `json:"default"`
}

type builtin struct {
	Builtin
	re *regexp.Regexp
	// template is the replacement with {} standing for the replacement text.
	// Capture groups keep the key ("password=") so only the value is hidden.
	template string
	// hints, fold and lead are as on rule. Each pattern bounds anything it
	// allows before its hint, so lead holds.
	hints []string
	fold  bool
	lead  int
}

// builtins run in this order: the specific ones before the broad ones.
var builtins = []builtin{
	{
		Builtin: Builtin{ID: "authorization", Name: "Authorization headers", Default: true,
			Description: "The value of an Authorization or Proxy-Authorization header, keeping the scheme."},
		re:       regexp.MustCompile(`(?i)(\b(?:proxy-)?authorization["']?\s*[:=]\s*["']?(?:(?:bearer|basic|token|digest|negotiate)\s+)?)[^\s"',;]+`),
		template: "${1}{}",
		hints:    []string{"authorization"},
		fold:     true,
		lead:     len("proxy-"),
	},
	{
		Builtin: Builtin{ID: "bearer", Name: "Bearer tokens", Default: true,
			Description: "Any token after the word Bearer."},
		re:       regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9\-._~+/]+=*`),
		template: "${1}{}",
		hints:    []string{"bearer"},
		fold:     true,
	},
	{
		Builtin: Builtin{ID: "credentials", Name: "Passwords and secrets", Default: true,
			Description: "Values of keys such as password, secret, token, api_key and access_key, in key=value, key: value and JSON form."},
		// The match starts at the keyword: the rest of a key like db_password
		// is kept either way, and a leading \w* would make the regex retry at
		// every position of every line.
		re:       regexp.MustCompile(`(?i)((?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key)["']?\s*[:=]\s*["']?)[^\s"',;&]+`),
		template: "${1}{}",
		// Not "key": cache and map keys are everywhere, and each false alarm
		// costs a regex run.
		hints: []string{"pass", "pwd", "secret", "token", "apikey", "api_key", "api-key",
			"accesskey", "access_key", "access-key", "privatekey", "private_key", "private-key"},
		fold: true,
	},
	{
		Builtin: Builtin{ID: "url-credentials", Name: "Passwords in URLs", Default: true,
			Description: "The password in a URL such as postgres://user:password@host."},
		re:       regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]{0,31}://[^\s:/@]*:)[^\s/@]+@`),
		template: "${1}{}@",
		hints:    []string{"://"},
		lead:     32,
	},
	{
		Builtin: Builtin{ID: "api-keys", Name: "Common API keys", Default: true,
			Description: "AWS access key IDs and GitHub, GitLab, Slack, Stripe, OpenAI, Anthropic and Google API keys."},
		re: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b` +
			`|\bgh[pousr]_[A-Za-z0-9]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{22,}\b` +
			`|\bglpat-[A-Za-z0-9_-]{20,}` +
			`|\bxox[abposr]-[A-Za-z0-9-]{10,}` +
			`|\b[rs]k_(?:live|test)_[A-Za-z0-9]{10,}\b` +
			`|\bsk-[A-Za-z0-9_-]{20,}` +
			`|\bAIza[0-9A-Za-z_-]{35}`),
		template: "{}",
		hints:    []string{"AKIA", "ASIA", "ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "glpat-", "xox", "k_live_", "k_test_", "sk-", "AIza"},
		lead:     1, // the s of sk_live_
	},
	{
		Builtin: Builtin{ID: "jwt", Name: "JSON Web Tokens", Default: true,
			Description: "Anything shaped like a JWT (eyJ….….…)."},
		re:       regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
		template: "{}",
		hints:    []string{"eyJ"},
	},
	{
		Builtin: Builtin{ID: "email", Name: "Email addresses",
			Description: "Email addresses. Off by default: they are personal data, but often what you search logs for."},
		re:       regexp.MustCompile(`[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`),
		template: "{}",
		hints:    []string{"@"},
		lead:     64,
	},
}

// Catalog lists the built-in patterns in the order they run.
func Catalog() []Builtin {
	catalog := make([]Builtin, len(builtins))
	for i, b := range builtins {
		catalog[i] = b.Builtin
	}
	return catalog
}

// DefaultBuiltins returns the IDs of the patterns offered as enabled before
// the settings are first saved.
func DefaultBuiltins() []string {
	var ids []string
	for _, b := range builtins {
		if b.Default {
			ids = append(ids, b.ID)
		}
	}
	return ids
}
