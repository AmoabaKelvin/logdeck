package redact

import (
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

func mustCompile(t *testing.T, cfg config.RedactionConfig) *Set {
	t.Helper()
	set, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return set
}

func TestBuiltinsRedactValuesAndKeepKeys(t *testing.T) {
	ids := make([]string, 0, len(builtins))
	for _, b := range builtins {
		ids = append(ids, b.ID)
	}
	set := mustCompile(t, config.RedactionConfig{Enabled: true, Builtins: ids})

	cases := []struct{ in, want string }{
		{"GET / Authorization: Bearer abc.def-123", "GET / Authorization: Bearer [REDACTED]"},
		{`{"authorization":"Basic dXNlcjpwYXNz"}`, `{"authorization":"Basic [REDACTED]"}`},
		{"calling api with bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2lnbmF0dXJl", "calling api with bearer [REDACTED]"},
		{"login user=bob password=hunter2 ok", "login user=bob password=[REDACTED] ok"},
		{`{"db_password": "hunter2", "level":"info"}`, `{"db_password": "[REDACTED]", "level":"info"}`},
		{"GET /cb?access_token=abc123&state=x", "GET /cb?access_token=[REDACTED]&state=x"},
		{"GITHUB_TOKEN: ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "GITHUB_TOKEN: [REDACTED]"},
		{"connecting to postgres://app:s3cret@db:5432/app", "connecting to postgres://app:[REDACTED]@db:5432/app"},
		{"key id AKIAABCDEFGHIJKLMNOP used", "key id [REDACTED] used"},
		{"stripe sk_live_abcdefghijklmnop failed", "stripe [REDACTED] failed"},
		{"session eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2lnbmF0dXJl", "session [REDACTED]"},
		{"mail sent to jane.doe@example.com", "mail sent to [REDACTED]"},
		// Lines the patterns must leave alone.
		{"used 512 tokens: ok", "used 512 tokens: ok"},
		{"max_tokens=4096 token_type=bearer", "max_tokens=4096 token_type=bearer"},
		{"Secret Manager: started", "Secret Manager: started"},
		{"GET /health 200 3ms", "GET /health 200 3ms"},
	}
	for _, tc := range cases {
		if got := set.Preview(tc.in); got != tc.want {
			t.Errorf("Preview(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestRedactionIsIdempotent(t *testing.T) {
	set := mustCompile(t, config.RedactionConfig{Enabled: true, Builtins: DefaultBuiltins()})
	once := set.Preview("Authorization: Bearer abc password=x postgres://u:p@h")
	if twice := set.Preview(once); twice != once {
		t.Fatalf("second pass changed %q to %q", once, twice)
	}
}

func TestEngineTimestampIsKept(t *testing.T) {
	set := mustCompile(t, config.RedactionConfig{
		Enabled: true,
		Rules:   []config.RedactionRule{{Name: "digits", Enabled: true, Pattern: `\d+`}},
	})
	line := "2026-10-30T12:00:00.123456789Z order 4242 shipped"
	want := "2026-10-30T12:00:00.123456789Z order [REDACTED] shipped"
	if got := set.For(Target{}).Line(line); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCustomRuleReplacementAndOrder(t *testing.T) {
	set := mustCompile(t, config.RedactionConfig{
		Enabled:     true,
		Replacement: "***",
		Rules: []config.RedactionRule{
			{Name: "ssn", Enabled: true, Pattern: `(ssn=)\d{3}-\d{2}-\d{4}`, Replacement: "${1}<ssn>"},
			{Name: "card", Enabled: true, Pattern: `\b\d{4}(?: \d{4}){3}\b`},
			{Name: "off", Enabled: false, Pattern: `shipped`},
		},
	})
	got := set.Preview("ssn=123-45-6789 card 4111 1111 1111 1111 shipped")
	if want := "ssn=<ssn> card *** shipped"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDollarInReplacementIsLiteral(t *testing.T) {
	set := mustCompile(t, config.RedactionConfig{Enabled: true, Replacement: "$1", Builtins: []string{"credentials"}})
	if got := set.Preview("password=x"); got != "password=$1" {
		t.Fatalf("got %q", got)
	}
}

func TestTargeting(t *testing.T) {
	set := mustCompile(t, config.RedactionConfig{
		Enabled: true,
		Rules: []config.RedactionRule{
			{Name: "api", Enabled: true, Pattern: `secret`, Hosts: []string{"prod"}, Projects: []string{"shop"}},
		},
	})
	cases := []struct {
		target Target
		want   bool
	}{
		{Target{Host: "prod", Name: "web", Project: "shop"}, true},
		{Target{Host: "prod", Name: "web"}, false},
		{Target{Host: "dev", Name: "web", Project: "shop"}, false},
	}
	for _, tc := range cases {
		if got := set.For(tc.target) != nil; got != tc.want {
			t.Errorf("For(%+v) applies = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestCompileOffOrEmptyReturnsNil(t *testing.T) {
	for _, cfg := range []config.RedactionConfig{
		{Enabled: false, Builtins: DefaultBuiltins()},
		{Enabled: true},
		{Enabled: true, Rules: []config.RedactionRule{{Name: "x", Pattern: "x"}}},
	} {
		set, err := Compile(cfg)
		if err != nil || set != nil {
			t.Errorf("Compile(%+v) = %v, %v; want nil, nil", cfg, set, err)
		}
	}
}

func TestCompileRejectsBadRules(t *testing.T) {
	cases := []struct {
		cfg  config.RedactionConfig
		want string
	}{
		{config.RedactionConfig{Builtins: []string{"nope"}}, `unknown built-in pattern "nope"`},
		{config.RedactionConfig{Rules: []config.RedactionRule{{Name: "broken", Pattern: "(a"}}}, `rule "broken": invalid pattern`},
		{config.RedactionConfig{Rules: []config.RedactionRule{{Pattern: ""}}}, "rule 1: pattern is required"},
		{config.RedactionConfig{Rules: []config.RedactionRule{{Name: "e", Pattern: "a*"}}}, "can match empty text"},
		{config.RedactionConfig{Rules: []config.RedactionRule{{Name: "e", Pattern: `[^ ]*`}}}, "can match empty text"},
		{config.RedactionConfig{Rules: []config.RedactionRule{{Name: "e", Pattern: `\b`}}}, "can match empty text"},
	}
	for _, tc := range cases {
		// Disabled rules and a disabled config are validated too.
		_, err := Compile(tc.cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Compile(%+v) error = %v, want it to contain %q", tc.cfg, err, tc.want)
		}
	}
}

func TestStreamFollowsActivation(t *testing.T) {
	t.Cleanup(func() { Activate(nil) })
	stream := NewStream(Target{Host: "local", Name: "web"})
	line := "password=hunter2"

	if got := stream.Line(line); got != line {
		t.Fatalf("inactive: got %q", got)
	}
	Activate(mustCompile(t, config.RedactionConfig{Enabled: true, Builtins: []string{"credentials"}}))
	if got := stream.Line(line); got != "password=[REDACTED]" {
		t.Fatalf("active: got %q", got)
	}
	Activate(nil)
	if got := stream.Line(line); got != line {
		t.Fatalf("deactivated: got %q", got)
	}
	var none *Stream
	if got := none.Line(line); got != line {
		t.Fatalf("nil stream: got %q", got)
	}
}

func TestAppendLowerKeepsOffsets(t *testing.T) {
	in := "Authorization: BEARER x-Ünicode"
	got := string(appendLower([]byte("keep:"), in))
	if want := "keep:authorization: bearer x-Ünicode"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Starting each regex at its earliest hint must not change what it redacts.
func TestHintWindowMatchesFullScan(t *testing.T) {
	lines := []string{
		"user=bob password=a token=b api_key=c private-key=d",
		"token=x then much later Authorization: Bearer abc and password=y",
		"reach postgres+ssl://app:pw@db and mysql://root:toor@db2",
		"keys sk_live_abcdefghijklmnop and AKIAABCDEFGHIJKLMNOP and ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"mail jane.doe@example.com, bob@example.org",
		"Proxy-Authorization: Basic dXNlcjpw then authorization=zzz",
		`{"a":"x","access_key":"k1","nested":{"secret":"s"}}`,
	}
	for _, b := range builtins {
		set := mustCompile(t, config.RedactionConfig{Enabled: true, Builtins: []string{b.ID}})
		replacement := set.rules[0].replacement
		for _, line := range lines {
			want := b.re.ReplaceAllString(line, replacement)
			if got := set.Preview(line); got != want {
				t.Errorf("%s on %q:\n got %q\nwant %q", b.ID, line, got, want)
			}
		}
	}
}

func TestFoldHintsStartWithLowerLetter(t *testing.T) {
	for _, b := range builtins {
		if !b.fold {
			continue
		}
		for _, hint := range b.hints {
			if hint == "" || hint[0] < 'a' || hint[0] > 'z' || strings.ToLower(hint) != hint {
				t.Errorf("%s: fold hint %q must be lower-case and start with a letter", b.ID, hint)
			}
		}
	}
}
