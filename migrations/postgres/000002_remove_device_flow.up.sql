-- Migration to remove device flow authentication
-- Drop indexes first
DROP INDEX IF EXISTS idx_device_flow_user_code;
DROP INDEX IF EXISTS idx_device_flow_expires_at;
DROP INDEX IF EXISTS idx_device_flow_user_id;

-- Drop table
DROP TABLE IF EXISTS device_flow_sessions;
