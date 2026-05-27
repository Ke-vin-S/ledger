DROP VIEW IF EXISTS user_net_balances;
DROP VIEW IF EXISTS team_net_balances;
DROP VIEW IF EXISTS debt_balances;

DROP TRIGGER IF EXISTS audit_log_no_delete ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
DROP FUNCTION IF EXISTS audit_log_immutable();

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS notification_prefs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS expense_flags;
DROP TABLE IF EXISTS settlements;
DROP TABLE IF EXISTS expense_splits;
DROP TABLE IF EXISTS expense_versions;
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS invite_links;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS claim_tokens;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS audit_action;
DROP TYPE IF EXISTS flag_status;
DROP TYPE IF EXISTS debt_status;
DROP TYPE IF EXISTS settlement_method;
DROP TYPE IF EXISTS settlement_status;
DROP TYPE IF EXISTS split_method;
DROP TYPE IF EXISTS expense_scope;
DROP TYPE IF EXISTS membership_status;
DROP TYPE IF EXISTS team_role;
DROP TYPE IF EXISTS identity_type;
