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

-- -----------------------------------------------------------------------------
-- Function: api.export_documents
-- Description: Example /file function. Set-returning function with the file
-- contract (path, content, mime_type, store_only); one row is served directly,
-- several rows are packed into a ZIP.
-- Params: {"count": 2, "bad_path": "../evil.txt"}  (bad_path is test-only)
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION api.export_documents(params jsonb DEFAULT '{}'::jsonb)
RETURNS TABLE (path text, content bytea, mime_type text, store_only boolean)
LANGUAGE sql
AS $$
    SELECT
        COALESCE(params->>'bad_path', 'doc-' || n || '.txt'),
        convert_to('Document ' || n, 'UTF8'),
        'text/plain'::text,
        false
    FROM generate_series(1, COALESCE((params->>'count')::int, 2)) AS n;
$$;

COMMENT ON FUNCTION api.export_documents(jsonb) IS 'Example file export (POST /file).
--- PARAMS ---
{}';
