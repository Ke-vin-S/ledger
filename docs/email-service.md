# Email Service (AWS SES)

Transactional email for SplitLedger. The API runs on AWS Lambda Function URL in
`ap-southeast-1`; email is sent through **AWS SES v2**.

## What's implemented

New package `api/internal/email/`:

- `Sender` — transport interface. Two impls: `SESSender` (real) and `LogSender`
  (logs instead of sending; used locally / when `EMAIL_FROM` is empty).
- `Mailer` — renders subject + HTML/text bodies, owns the `From` address and the
  frontend base URL (so domain code never builds links).

Wired into four flows (best-effort — a send failure is logged and never fails the
originating request):

| Trigger                        | Where                                             | Email                                 |
| ------------------------------ | ------------------------------------------------- | ------------------------------------- | --------------- |
| Team member invite             | `POST /v1/teams/{id}/members/invite`              | "You've been invited to <team>"       |
| Password reset                 | `POST /v1/auth/password/reset-request`            | reset link (`/reset-password?token=`) |
| Join request approved/rejected | `POST .../members/requests/{rid}/approve          | reject`                               | decision notice |
| Invite-link sharing            | `POST /v1/teams/{id}/invite-links` with `"email"` | shareable `/invite/<token>` link      |

Scope: **registered users only** — inviting an unknown email still returns
`USER_NOT_FOUND` (see "Later / not yet done").

## Configuration

| Source                                      | Meaning                                                                                                                     |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `/splitleger/email_from` (SSM SecureString) | Verified SES sender, e.g. `SplitLedger <no-reply@ledger.kevinsanjula.me>`. Empty or absent logs mail instead of sending it. |
| `AWS_REGION`                                | Lambda-provided region; SES identity must be verified in the same region.                                                   |

The CDK data stack creates the encrypted parameter. Populate it with:

```bash
aws ssm put-parameter --name /splitleger/email_from --type SecureString --overwrite \
  --value "SplitLedger <no-reply@ledger.kevinsanjula.me>"
```

## AWS console setup (one-time, region ap-southeast-1)

1. **Verify a sender identity** — SES → Verified identities → Create identity.
   Prefer a **domain** (`ledger.kevinsanjula.me`): add the 3 DKIM CNAME records SES
   gives you to DNS, wait for "Verified". (Or verify a single email address for a
   quick test.)
2. **Leave the sandbox** — SES → Account dashboard → Request production access.
   In the sandbox you can only send to _verified_ recipients, so real invites won't
   arrive until this is approved (usually < 24h).
3. **Set the sender parameter** in SSM Parameter Store as shown above; the Lambda execution role decrypts it at cold start.
4. **IAM** — `SplitlegerApp` grants the Lambda role `ses:SendEmail` and `ses:SendRawEmail`. Do not add permissions manually or recreate them through a setup script.

Test: `POST /v1/auth/password/reset-request` or a team invite, then check the inbox
and CloudWatch logs for SES errors.

## Later / not yet done

- **Notification preferences** — `user.NotificationPrefs.EmailEnabled` / `DisabledTypes` exist but transactional emails are sent unconditionally for now. Decide which of these should honour the per-user opt-out.
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
