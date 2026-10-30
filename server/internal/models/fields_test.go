package models

import "testing"

func TestParseFieldFilter(t *testing.T) {
	filter, err := ParseFieldFilter(`level=error  user.id!=42 msg="slow \"db\" query"` + "\tpath=")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		key, value string
		negate     bool
	}{
		{"level", "error", false},
		{"user.id", "42", true},
		{"msg", `slow "db" query`, false},
		{"path", "", false},
	}
	if len(filter) != len(want) {
		t.Fatalf("got %d terms, want %d", len(filter), len(want))
	}
	for i, w := range want {
		if term := filter[i]; term.Key != w.key || term.Value != w.value || term.Negate != w.negate {
			t.Errorf("term %d = %+v, want %+v", i, term, w)
		}
	}

	for _, bad := range []string{"timeout", "=x", "1a=b", `"a"=b`} {
		if _, err := ParseFieldFilter(bad); err == nil {
			t.Errorf("ParseFieldFilter(%q) succeeded", bad)
		}
	}
	if filter, err := ParseFieldFilter("  "); err != nil || filter != nil {
		t.Errorf("blank filter = %v, %v", filter, err)
	}
}

func TestFieldFilterMatch(t *testing.T) {
	jsonLine := `{"level":"ERROR","msg":"checkout failed","user_id":42,"ok":false,"err":null,` +
		`"http":{"path":"/checkout","status":500},"log.level":"warn","tags":["a"]}`
	logfmtLine := `time=2026-05-28T05:00:38Z level=error msg="checkout failed" user_id=42 http.path=/checkout`

	tests := []struct {
		filter  string
		message string
		want    bool
	}{
		{"level=error", jsonLine, true},
		{"level=error user_id=42", jsonLine, true},
		{"user_id=42.0", jsonLine, true},
		{"user_id=43", jsonLine, false},
		{"http.path=/checkout http.status=500", jsonLine, true},
		{"http.status=404", jsonLine, false},
		{"log.level=warn", jsonLine, true},
		{"ok=false err=null", jsonLine, true},
		{`msg="checkout failed"`, jsonLine, true},
		{"msg=checkout", jsonLine, false},
		{"http=x", jsonLine, false},
		{"tags=a", jsonLine, false},
		{"missing=x", jsonLine, false},
		{"level!=info", jsonLine, true},
		{"level!=error", jsonLine, false},
		{"missing!=x", jsonLine, true},

		{"level=ERROR user_id=42", logfmtLine, true},
		{`msg="checkout failed"`, logfmtLine, true},
		{"http.path=/checkout", logfmtLine, true},
		{"user_id=42.0", logfmtLine, false}, // logfmt values are text
		{"level!=error", logfmtLine, false},

		// Pairs in a plain sentence count; the sentence itself does not.
		{"err=nil", "request done err=nil took=3ms", true},
		{"took=3ms", "request done err=nil took=3ms", true},
		{"request=done", "request done err=nil took=3ms", false},
		{"level=error", "ERROR something broke", false},
		{"level!=error", "ERROR something broke", true},

		// Escapes skip the prefilter rather than defeating it.
		{"path=/checkout", `{"path":"\/checkout"}`, true},
		{`msg="say \"hi\""`, `msg="say \"hi\"" level=info`, true},
		{"msg=café", `{"msg":"café"}`, true},

		// Not a JSON object: read as logfmt.
		{"a=1", `{"a":1} trailing a=1`, true},
	}
	for _, tt := range tests {
		filter, err := ParseFieldFilter(tt.filter)
		if err != nil {
			t.Fatalf("%q: %v", tt.filter, err)
		}
		if got := filter.Match(tt.message); got != tt.want {
			t.Errorf("%q on %q = %v, want %v", tt.filter, tt.message, got, tt.want)
		}
	}

	if !FieldFilter(nil).Match("anything") {
		t.Error("an empty filter must match every line")
	}
}
