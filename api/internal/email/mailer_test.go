package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Ke-vin-S/ledger/api/internal/email"
)

// fakeSender captures the last message instead of delivering it.
type fakeSender struct {
	msg   email.Message
	calls int
	err   error
}

func (s *fakeSender) Send(_ context.Context, msg email.Message) error {
	s.calls++
	s.msg = msg
	return s.err
}

func newMailer(s email.Sender) *email.Mailer {
	return email.NewMailer(s, "SplitLedger <no-reply@example.com>", "https://app.example.com/")
}

func TestInvitationEmail_SetsFromToSubjectAndAcceptURL(t *testing.T) {
	s := &fakeSender{}
	if err := newMailer(s).InvitationEmail(context.Background(), "bob@x.com", "Trip", "Alice", "tok123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.calls != 1 {
		t.Fatalf("want 1 send, got %d", s.calls)
	}
	m := s.msg
	if m.From != "SplitLedger <no-reply@example.com>" {
		t.Errorf("from = %q", m.From)
	}
	if m.To != "bob@x.com" {
		t.Errorf("to = %q", m.To)
	}
	if !strings.Contains(m.Subject, "Alice") || !strings.Contains(m.Subject, "Trip") {
		t.Errorf("subject = %q, want it to mention inviter + team", m.Subject)
	}
	const wantURL = "https://app.example.com/invitations/tok123"
	if !strings.Contains(m.TextBody, wantURL) {
		t.Errorf("text body missing accept URL %q\nbody: %s", wantURL, m.TextBody)
	}
}

func TestPasswordReset_BuildsTokenURL(t *testing.T) {
	s := &fakeSender{}
	if err := newMailer(s).PasswordReset(context.Background(), "u@x.com", "User", "tok en/+"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// frontendURL trailing slash is trimmed; token is query-escaped. Assert on the
	// text body — the HTML body additionally entity-escapes "+" to "&#43;".
	const want = "https://app.example.com/reset-password?token=tok+en%2F%2B"
	if !strings.Contains(s.msg.TextBody, want) {
		t.Errorf("text body missing reset URL %q\nbody: %s", want, s.msg.TextBody)
	}
}

func TestInviteLink_BuildsInvitePathURL(t *testing.T) {
	s := &fakeSender{}
	if err := newMailer(s).InviteLink(context.Background(), "u@x.com", "Trip", "abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = "https://app.example.com/invite/abc123"
	if !strings.Contains(s.msg.TextBody, want) {
		t.Errorf("text body missing invite URL %q\nbody: %s", want, s.msg.TextBody)
	}
}

func TestSend_EmptyRecipient_Errors(t *testing.T) {
	s := &fakeSender{}
	err := newMailer(s).JoinApproved(context.Background(), "", "User", "Trip")
	if err == nil {
		t.Fatal("expected an error for empty recipient")
	}
	if s.calls != 0 {
		t.Error("sender should not be called with an empty recipient")
	}
}
