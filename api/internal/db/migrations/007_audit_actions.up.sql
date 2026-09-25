ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'member.role_changed';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'loan.created';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'user.password_reset';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'team.invite_link_created';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'team.invite_link_revoked';
