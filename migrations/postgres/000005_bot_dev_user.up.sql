-- Create the bot-dev user for development mode bot token
-- This user is referenced by the dev-only authentication in the interceptor
INSERT INTO users (id, email, name, role, user_type, disabled)
VALUES ('bot-dev', '', 'Development Bot', 'bot', 'oidc', false)
ON CONFLICT (id) DO NOTHING;
