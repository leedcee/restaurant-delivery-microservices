DROP TRIGGER IF EXISTS partner_audit_log_append_only ON partner_audit_log;
DROP FUNCTION IF EXISTS reject_partner_audit_mutation();
DROP TABLE IF EXISTS partner_audit_log;
DROP TABLE IF EXISTS partner_credentials;
