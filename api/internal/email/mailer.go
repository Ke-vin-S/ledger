package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// Mailer renders and sends the application's transactional emails. It owns the
// sender, the From address, and the frontend base URL so that domain services
// only pass data — never links or formatting.
//
// One Mailer value satisfies both team.Mailer and user.Mailer (structural typing).
type Mailer struct {
	sender      Sender
	from        string
	frontendURL string
	log         *zap.Logger
}

// NewMailer constructs a Mailer. `from` is the verified SES sender address
// (e.g. `SplitLedger <no-reply@example.com>`); `frontendURL` is the web app base.
// `log` surfaces send failures — callers swallow the returned error (emails are
// best-effort), so this is the only place a failed send becomes visible.
func NewMailer(sender Sender, from, frontendURL string, log *zap.Logger) *Mailer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Mailer{
		sender:      sender,
		from:        from,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		log:         log,
	}
}

// InvitationEmail invites someone (registered or not) to a team via a token link
// that takes them to the accept page. Built from the raw invitation token.
func (m *Mailer) InvitationEmail(ctx context.Context, to, teamName, inviterName, rawToken string) error {
	acceptURL := m.frontendURL + "/invitations/" + url.PathEscape(rawToken)
	return m.send(ctx, to,
		fmt.Sprintf("%s invited you to %q on SplitLedger", inviterName, teamName),
		emailContent{
			Heading: "You've been invited",
			Lines: []string{
				fmt.Sprintf("%s invited you to join the team %q on SplitLedger.", inviterName, teamName),
				"Click below to accept. You'll be asked to sign in or create an account first. This invitation expires in 7 days.",
			},
			ButtonLabel: "Accept invitation",
			ButtonURL:   acceptURL,
		})
}

// PasswordReset sends a one-time password reset link built from the raw token.
func (m *Mailer) PasswordReset(ctx context.Context, to, userName, rawToken string) error {
	resetURL := m.frontendURL + "/reset-password?token=" + url.QueryEscape(rawToken)
	return m.send(ctx, to,
		"Reset your SplitLedger password",
		emailContent{
			Heading: "Reset your password",
			Lines: []string{
				fmt.Sprintf("Hi %s,", userName),
				"We received a request to reset your SplitLedger password. This link expires in 1 hour.",
				"If you didn't request this, you can safely ignore this email.",
			},
			ButtonLabel: "Reset password",
			ButtonURL:   resetURL,
		})
}

// JoinApproved notifies a user that their request to join a team was approved.
func (m *Mailer) JoinApproved(ctx context.Context, to, userName, teamName string) error {
	return m.send(ctx, to,
		fmt.Sprintf("You're now a member of %q", teamName),
		emailContent{
			Heading: "Request approved",
			Lines: []string{
				fmt.Sprintf("Hi %s,", userName),
				fmt.Sprintf("Your request to join %q on SplitLedger was approved. You're now a member.", teamName),
			},
			ButtonLabel: "Open team",
			ButtonURL:   m.frontendURL + "/teams",
		})
}

// JoinRejected notifies a user that their request to join a team was rejected.
func (m *Mailer) JoinRejected(ctx context.Context, to, userName, teamName string) error {
	return m.send(ctx, to,
		fmt.Sprintf("Update on your request to join %q", teamName),
		emailContent{
			Heading: "Request not approved",
			Lines: []string{
				fmt.Sprintf("Hi %s,", userName),
				fmt.Sprintf("Your request to join %q on SplitLedger was not approved at this time.", teamName),
			},
		})
}

// InviteLink emails a shareable team invite link, built from the raw token.
func (m *Mailer) InviteLink(ctx context.Context, to, teamName, rawToken string) error {
	inviteURL := m.frontendURL + "/invite/" + url.PathEscape(rawToken)
	return m.send(ctx, to,
		fmt.Sprintf("You've been invited to join %q on SplitLedger", teamName),
		emailContent{
			Heading: "Join the team",
			Lines: []string{
				fmt.Sprintf("You've been invited to join %q on SplitLedger.", teamName),
				"Use the link below to join. You'll be asked to sign in or create an account first.",
			},
			ButtonLabel: "Join team",
			ButtonURL:   inviteURL,
		})
}

// send renders the HTML + text bodies and dispatches through the sender. Any
// failure is logged here at warn level — callers treat email as best-effort and
// discard the returned error, so this log is the only signal a send failed.
func (m *Mailer) send(ctx context.Context, to, subject string, c emailContent) error {
	err := m.dispatch(ctx, to, subject, c)
	if err != nil {
		m.log.Warn("email send failed",
			zap.String("to", to),
			zap.String("from", m.from),
			zap.String("subject", subject),
			zap.Error(err),
		)
	}
	return err
}

func (m *Mailer) dispatch(ctx context.Context, to, subject string, c emailContent) error {
	if to == "" {
		return fmt.Errorf("email: empty recipient")
	}
	htmlBody, err := c.renderHTML()
	if err != nil {
		return fmt.Errorf("render html: %w", err)
	}
	return m.sender.Send(ctx, Message{
		From:     m.from,
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: c.renderText(),
	})
}

// emailContent is the data for the shared email layout.
type emailContent struct {
	Heading     string
	Lines       []string
	ButtonLabel string
	ButtonURL   string
}

var layoutTmpl = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:Arial,Helvetica,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:24px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;padding:32px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;color:#18181b;">{{.Heading}}</h1>
          {{range .Lines}}<p style="margin:0 0 12px;font-size:14px;line-height:20px;color:#3f3f46;">{{.}}</p>{{end}}
          {{if .ButtonURL}}<p style="margin:24px 0 0;">
            <a href="{{.ButtonURL}}" style="display:inline-block;background:#4f46e5;color:#ffffff;text-decoration:none;padding:10px 20px;border-radius:6px;font-size:14px;">{{.ButtonLabel}}</a>
          </p>
          <p style="margin:16px 0 0;font-size:12px;color:#a1a1aa;">Or paste this link into your browser:<br>{{.ButtonURL}}</p>{{end}}
        </td></tr>
      </table>
      <p style="margin:16px 0 0;font-size:12px;color:#a1a1aa;">SplitLedger</p>
    </td></tr>
  </table>
</body>
</html>`))

func (c emailContent) renderHTML() (string, error) {
	var buf bytes.Buffer
	if err := layoutTmpl.Execute(&buf, c); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c emailContent) renderText() string {
	var b strings.Builder
	b.WriteString(c.Heading)
	b.WriteString("\n\n")
	for _, line := range c.Lines {
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	if c.ButtonURL != "" {
		b.WriteString(c.ButtonLabel)
		b.WriteString(": ")
		b.WriteString(c.ButtonURL)
		b.WriteString("\n\n")
	}
	b.WriteString("— SplitLedger")
	return b.String()
}
