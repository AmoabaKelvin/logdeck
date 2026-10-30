package alerts

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

func TestDeliverSlackBlocks(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	alert := testAlert()
	alert.Sample = "<!channel> upstream timeout"
	ch := config.AlertChannel{ID: "c1", Type: "slack", Enabled: true, URL: srv.URL}
	if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	var payload slackPayload
	if err := json.Unmarshal(cap.body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if want := "LogDeck alert: High error rate: 5 matches (level &gt;= ERROR) within 60s (local/web)"; payload.Text != want {
		t.Fatalf("fallback text = %q, want the escaped summary %q", payload.Text, want)
	}
	if len(payload.Blocks) != 2 || !strings.Contains(payload.Blocks[0].Text.Text, "High error rate") {
		t.Fatalf("blocks = %+v, want a heading section and a fields section", payload.Blocks)
	}
	// Log text is escaped so a sample cannot ping the channel.
	if got := payload.Blocks[1].Fields[2].Text; got != "*Sample*\n&lt;!channel&gt; upstream timeout" {
		t.Fatalf("sample field = %q, want it escaped", got)
	}
}

func TestDeliverDiscordEmbed(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "discord", Enabled: true, URL: srv.URL}
	for _, tc := range []struct {
		recovered bool
		color     int
	}{{false, discordRed}, {true, discordGreen}} {
		alert := testAlert()
		alert.Recovered = tc.recovered
		if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
			t.Fatalf("result = %+v, want ok", res)
		}
		var payload discordPayload
		if err := json.Unmarshal(cap.body, &payload); err != nil {
			t.Fatalf("payload is not valid JSON: %v", err)
		}
		if len(payload.Embeds) != 1 {
			t.Fatalf("embeds = %+v, want one", payload.Embeds)
		}
		e := payload.Embeds[0]
		if e.Title != alertTitle+": High error rate" || e.Description != alert.Reason || e.Color != tc.color {
			t.Fatalf("recovered=%v: embed = %+v", tc.recovered, e)
		}
		if len(e.Fields) == 0 || e.Fields[0].Name != "Host" || e.Fields[0].Value != "local" {
			t.Fatalf("fields = %+v, want host first", e.Fields)
		}
	}
}

// A Slack or Discord webhook URL is the credential, so a network error must
// not store it in history.
func TestDeliveryErrorRedactsWebhookURL(t *testing.T) {
	n := &notifier{client: &http.Client{Transport: failingRoundTripper{}}, retryDelay: 0}
	ch := config.AlertChannel{ID: "c1", Type: "slack", Enabled: true, URL: "https://hooks.slack.com/services/T0/B0/secretpart"}

	res := n.deliver(context.Background(), ch, testAlert(), nil)
	if res.Status != "failed" || strings.Contains(res.Error, "secretpart") {
		t.Fatalf("result = %+v, want failed without the webhook URL", res)
	}
}

func TestDeliverPagerDutyTriggerAndResolve(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "pagerduty", Enabled: true, URL: srv.URL, Token: "rk-123"}
	alert := testAlert()
	if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	var trigger pagerDutyEvent
	if err := json.Unmarshal(cap.body, &trigger); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if trigger.RoutingKey != "rk-123" || trigger.EventAction != "trigger" || trigger.DedupKey != "logdeck:r1:local:web" {
		t.Fatalf("trigger = %+v", trigger)
	}
	if p := trigger.Payload; p == nil || p.Summary != alertText(alert) || p.Source != "local" ||
		p.Severity != "critical" || p.CustomDetails.ID != alert.ID {
		t.Fatalf("trigger payload = %+v", p)
	}

	// The recovery is a new alert for the same rule and target.
	alert.ID, alert.Recovered = "efgh5678", true
	if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	var resolve map[string]any
	if err := json.Unmarshal(cap.body, &resolve); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if resolve["event_action"] != "resolve" || resolve["dedup_key"] != trigger.DedupKey || resolve["payload"] != nil {
		t.Fatalf("resolve = %v, want a resolve for the trigger's dedup key", resolve)
	}
}

func TestDeliverOpsgenieCreateAndClose(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "opsgenie", Enabled: true, URL: srv.URL + "/", Token: "key-1"}
	alert := testAlert()
	if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if cap.path != "/v2/alerts" || cap.auth != "GenieKey key-1" {
		t.Fatalf("path = %q, auth = %q", cap.path, cap.auth)
	}
	var created opsgenieAlert
	if err := json.Unmarshal(cap.body, &created); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if created.Alias != "logdeck:r1:local:web" || created.Message != alertText(alert) || !strings.Contains(created.Description, "Container: web") {
		t.Fatalf("created = %+v", created)
	}

	alert.Recovered = true
	if res := newNotifier().deliver(context.Background(), ch, alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if cap.path != "/v2/alerts/"+created.Alias+"/close" || cap.rawQuery != "identifierType=alias" {
		t.Fatalf("close request = %s?%s", cap.path, cap.rawQuery)
	}
}

// fakeSMTP is a minimal plaintext SMTP server. reject, when set, is its reply
// to MAIL FROM.
type fakeSMTP struct {
	addr     string
	reject   string
	sessions atomic.Int32
	auth     chan string
	from     chan string
	rcpts    chan []string
	data     chan string
}

func newFakeSMTP(t *testing.T, reject string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeSMTP{
		addr:   ln.Addr().String(),
		reject: reject,
		auth:   make(chan string, 4),
		from:   make(chan string, 4),
		rcpts:  make(chan []string, 4),
		data:   make(chan string, 4),
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			f.sessions.Add(1)
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	reply := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }
	reply("220 fake ESMTP")
	var rcpts []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch strings.ToUpper(strings.SplitN(line, " ", 2)[0]) {
		case "EHLO":
			reply("250-fake")
			reply("250 AUTH PLAIN")
		case "AUTH":
			f.auth <- line
			reply("235 ok")
		case "MAIL":
			if f.reject != "" {
				reply(f.reject)
				continue
			}
			f.from <- line
			reply("250 ok")
		case "RCPT":
			rcpts = append(rcpts, line)
			reply("250 ok")
		case "DATA":
			reply("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.rcpts <- rcpts
			f.data <- b.String()
			reply("250 queued")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("502 unknown")
		}
	}
}

func emailChannel(addr string) config.AlertChannel {
	return config.AlertChannel{
		ID:       "c1",
		Type:     "email",
		Enabled:  true,
		URL:      "smtp://" + addr,
		Username: "alerts",
		Token:    "s3cret",
		From:     "LogDeck <alerts@example.com>",
		Target:   "ops@example.com, Dev Team <dev@example.com>",
	}
}

func TestDeliverEmail(t *testing.T) {
	srv := newFakeSMTP(t, "")
	alert := testAlert()

	if res := newNotifier().deliver(context.Background(), emailChannel(srv.addr), alert, nil); res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	// net/smtp sends the password unencrypted only to localhost, which is
	// what lets this plaintext server see it.
	wantAuth := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00alerts\x00s3cret"))
	if got := <-srv.auth; got != wantAuth {
		t.Fatalf("auth = %q, want %q", got, wantAuth)
	}
	if got := <-srv.from; got != "MAIL FROM:<alerts@example.com>" {
		t.Fatalf("mail from = %q", got)
	}
	if rcpts := <-srv.rcpts; len(rcpts) != 2 || rcpts[0] != "RCPT TO:<ops@example.com>" || rcpts[1] != "RCPT TO:<dev@example.com>" {
		t.Fatalf("rcpts = %q", rcpts)
	}
	header, body, ok := strings.Cut(<-srv.data, "\r\n\r\n")
	if !ok {
		t.Fatalf("message has no header/body split: %q", header)
	}
	if !strings.Contains(header, "Subject: "+alertText(alert)+"\r\n") ||
		!strings.Contains(header, "To: <ops@example.com>, \"Dev Team\" <dev@example.com>\r\n") {
		t.Fatalf("headers = %q", header)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	if err != nil {
		t.Fatalf("body is not quoted-printable: %v", err)
	}
	if !strings.Contains(string(decoded), "Reason: "+alert.Reason+"\r\n") || !strings.Contains(string(decoded), "Container: web") {
		t.Fatalf("body = %q", decoded)
	}
}

func TestDeliverEmailPermanentRejectIsNotRetried(t *testing.T) {
	srv := newFakeSMTP(t, "550 sender rejected")
	n := newNotifier()
	n.retryDelay = time.Minute // would hang the test if a 5xx were retried

	res := n.deliver(context.Background(), emailChannel(srv.addr), testAlert(), nil)
	if res.Status != "failed" || !strings.Contains(res.Error, "sender rejected") {
		t.Fatalf("result = %+v, want failed with the server's reply", res)
	}
	if got := srv.sessions.Load(); got != 1 {
		t.Fatalf("sessions = %d, want 1 (5xx is permanent)", got)
	}
}

func TestDeliverEmailRetriesTransientReject(t *testing.T) {
	srv := newFakeSMTP(t, "451 try again later")
	n := newNotifier()
	n.retryDelay = 10 * time.Millisecond

	if res := n.deliver(context.Background(), emailChannel(srv.addr), testAlert(), nil); res.Status != "failed" {
		t.Fatalf("result = %+v, want failed", res)
	}
	if got := srv.sessions.Load(); got != 2 {
		t.Fatalf("sessions = %d, want 2 (4xx is retried once)", got)
	}
}
