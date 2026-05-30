-- Anonymous users cannot accept invitations interactively, so any 'invited'
-- membership they hold should be 'active'.
UPDATE team_members
SET status    = 'active',
    joined_at = COALESCE(joined_at, NOW())
WHERE status = 'invited'
  AND user_id IN (
    SELECT id FROM users WHERE identity_type = 'anonymous'
  );

-- Registered users who claimed an anonymous account inherited the wrong
-- 'invited' status from the old Claim() transaction. Claiming is acceptance.
UPDATE team_members
SET status    = 'active',
    joined_at = COALESCE(joined_at, NOW())
WHERE status = 'invited'
  AND EXISTS (
    SELECT 1 FROM users u
    WHERE u.claimed_by = team_members.user_id
      AND u.identity_type = 'anonymous'
  );
