# Email Service (AWS SES)

Transactional email for SplitLedger. Backend is on AWS Lambda + API Gateway in
`ap-south-1`; email is sent through **AWS SES v2**.

## What's implemented

New package `api/internal/email/`:
- `Sender` — transport interface. Two impls: `SESSender` (real) and `LogSender`
  (logs instead of sending; used locally / when `EMAIL_FROM` is empty).
- `Mailer` — renders subject + HTML/text bodies, owns the `From` address and the
  frontend base URL (so domain code never builds links).

Wired into four flows (best-effort — a send failure is logged and never fails the
originating request):

| Trigger | Where | Email |
| --- | --- | --- |
| Team member invite | `POST /v1/teams/{id}/members/invite` | "You've been invited to <team>" |
| Password reset | `POST /v1/auth/password/reset-request` | reset link (`/reset-password?token=`) |
| Join request approved/rejected | `POST .../members/requests/{rid}/approve|reject` | decision notice |
| Invite-link sharing | `POST /v1/teams/{id}/invite-links` with `"email"` | shareable `/invite/<token>` link |

Scope: **registered users only** — inviting an unknown email still returns
`USER_NOT_FOUND` (see "Later / not yet done").

## Configuration

| Env var | Meaning |
| --- | --- |
| `EMAIL_FROM` | Verified SES sender, e.g. `SplitLedger <no-reply@ledger.kevinsanjula.me>`. **Empty → emails are logged, not sent** (no AWS creds needed locally). |
| `AWS_REGION` | SES region. Must match where the identity is verified (`ap-south-1`). |

`.env.example` line to add (the file is git-blocked from tooling here — add manually):

```
# Verified SES sender. Leave empty in local dev to log emails instead of sending.
EMAIL_FROM=
```

## AWS console setup (one-time, region ap-south-1)

1. **Verify a sender identity** — SES → Verified identities → Create identity.
   Prefer a **domain** (`ledger.kevinsanjula.me`): add the 3 DKIM CNAME records SES
   gives you to DNS, wait for "Verified". (Or verify a single email address for a
   quick test.)
2. **Leave the sandbox** — SES → Account dashboard → Request production access.
   In the sandbox you can only send to *verified* recipients, so real invites won't
   arrive until this is approved (usually < 24h).
3. **Set `EMAIL_FROM`** on the Lambda (Configuration → Environment variables).
4. **IAM** — `api/scripts/setup-lambda.sh` attaches an `ses:SendEmail` /
   `ses:SendRawEmail` policy to the exec role. If the function already exists and you
   don't re-run the script, add this inline policy to `splitleger-api-exec-role`:
   ```json
   { "Version": "2012-10-17", "Statement": [{ "Effect": "Allow",
     "Action": ["ses:SendEmail","ses:SendRawEmail"], "Resource": "*" }] }
   ```

Test: `POST /v1/auth/password/reset-request` or a team invite, then check the inbox
and CloudWatch logs for SES errors.

## Later / not yet done

- **Frontend `/reset-password` page** (Next.js, `web/app/(auth)/`). The reset email
  already links to `FRONTEND_URL/reset-password?token=...`; the page that consumes the
  token and calls `POST /v1/auth/password/reset` doesn't exist yet.
- **Invite-link email UI** — add an optional "email this link to…" field to the team
  invite modal that passes `email` to `POST /v1/teams/{id}/invite-links`.
- **Invite-to-signup for non-registered emails** — currently inviting an email with no
  account returns `USER_NOT_FOUND`. A future iteration would create a pending invite +
  email a signup link that joins the team after registration (new table/token flow).
- **Respect notification prefs** — `user.NotificationPrefs.EmailEnabled` /
  `DisabledTypes` exist but transactional emails are sent unconditionally for now.
  Decide which of these should honour the per-user opt-out.
- **Bounce / complaint handling** — wire an SES → SNS topic to track bounces and
  suppress bad addresses before requesting higher SES sending limits.
