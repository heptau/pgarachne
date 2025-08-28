-- =============================================================================
-- User Management
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Role: pgarachne (Service User)
-- Used by the PgArachne gateway to manage tokens and proxy requests.
-- -----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'pgarachne') THEN
        CREATE ROLE pgarachne WITH LOGIN PASSWORD 'change_me_in_production';
    END IF;
END
$$;
COMMENT ON ROLE pgarachne IS 'Service user for the PgArachne gateway.';

-- Permissions for service user
GRANT USAGE ON SCHEMA pgarachne TO pgarachne;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE pgarachne.api_tokens TO pgarachne;

-- -----------------------------------------------------------------------------
-- Role: demo (Example User)
-- -----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'demo') THEN
        CREATE ROLE demo WITH LOGIN PASSWORD 'Demo1234';
    END IF;
END
$$;
COMMENT ON ROLE demo IS 'Demo user for testing.';

-- Allow pgarachne to switch to demo role (impersonation)
GRANT demo TO pgarachne;

-- Grant permissions to demo user on the API schema
GRANT USAGE ON SCHEMA api TO demo;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA api TO demo;

-- -----------------------------------------------------------------------------
-- Seed Data: Create an initial token for the demo user
-- -----------------------------------------------------------------------------
SELECT pgarachne.add_api_token(
    token_description => 'demo',
    target_role => 'demo'
);
