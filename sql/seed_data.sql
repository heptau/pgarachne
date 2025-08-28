-- =============================================================================
-- Seed Data / Example Functions
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Function: api.server_info
-- Description: Example function returning server details.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION api.server_info(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN json_build_object(
        'server_version', version(),
        'current_user', current_user,
        'current_database', current_database(),
        'current_time', now()
    );
END;
$$;

COMMENT ON FUNCTION api.server_info(jsonb) IS 'Returns PostgreSQL server information.
--- PARAMS ---
{}';
