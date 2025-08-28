-- =============================================================================
-- PgArachne Schema Definition
-- =============================================================================

-- Create schema for PgArachne internal functionality
CREATE SCHEMA IF NOT EXISTS pgarachne;
COMMENT ON SCHEMA pgarachne IS 'Schema for PgArachne internal functionality (tokens, system functions).';

-- Create schema for public API functions
CREATE SCHEMA IF NOT EXISTS api;
COMMENT ON SCHEMA api IS 'Schema for user-defined JSON-RPC functions exposed via PgArachne.';
GRANT USAGE ON SCHEMA api TO public;

-- Extension: pgcrypto (Required for hashing and random generation)
CREATE EXTENSION IF NOT EXISTS pgcrypto;


-- =============================================================================
-- Table: pgarachne.api_tokens
-- =============================================================================
CREATE TABLE IF NOT EXISTS pgarachne.api_tokens (
    token_hash TEXT PRIMARY KEY,
    role TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMPTZ,

    CONSTRAINT updated_at_check CHECK (updated_at >= created_at),
    CONSTRAINT valid_to_check CHECK (valid_to IS NULL OR valid_to >= updated_at)
);

COMMENT ON TABLE pgarachne.api_tokens IS 'Stores long-lived API tokens for authentication.';


-- Trigger to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION pgarachne.update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_api_tokens_timestamp
BEFORE UPDATE ON pgarachne.api_tokens
FOR EACH ROW
EXECUTE FUNCTION pgarachne.update_timestamp();


-- =============================================================================
-- Function: pgarachne.add_api_token
-- Description: Generates a random token, hashes it, and saves it. Returns raw token.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.add_api_token(
    token_description TEXT,
    target_role TEXT DEFAULT CURRENT_USER,
    token_valid_to TIMESTAMPTZ DEFAULT NULL
)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    raw_token TEXT;
    hashed_token TEXT;
BEGIN
    IF token_valid_to IS NOT NULL AND token_valid_to <= NOW() THEN
        RAISE EXCEPTION 'valid_to must be in the future';
    END IF;

    -- Generate random token (32 bytes hex = 64 chars)
    raw_token := encode(gen_random_bytes(32), 'hex');
    -- Hash it using SHA-256
    hashed_token := encode(digest(raw_token, 'sha256'), 'hex');

    INSERT INTO pgarachne.api_tokens (role, token_hash, description, valid_to)
    VALUES (target_role, hashed_token, token_description, token_valid_to);

    RETURN raw_token;
END;
$$;

COMMENT ON FUNCTION pgarachne.add_api_token(TEXT, TEXT, TIMESTAMPTZ) IS 'Generates a new API token for the specified role.
--- PARAMS ---
{
    "token_description": {"type": "string", "description": "Human readable description"},
    "target_role": {"type": "string", "description": "Database role to impersonate (default: current_user)"},
    "token_valid_to": {"type": "string", "format": "date-time", "description": "Expiration time (optional)"}
}';


-- =============================================================================
-- Function: pgarachne.verify_api_token
-- Description: Verifies a raw token and returns the associated role if valid.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.verify_api_token(input_raw_token TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
   found_role TEXT;
   input_hash TEXT;
BEGIN
   input_hash := encode(digest(input_raw_token, 'sha256'), 'hex');

   SELECT role INTO found_role
   FROM pgarachne.api_tokens
   WHERE token_hash = input_hash
    AND (valid_to IS NULL OR valid_to > NOW());

   RETURN found_role;
END;
$$;


-- =============================================================================
-- Function: pgarachne.allowed_schemas
--Description: Returns list of schemas exposed via API.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.allowed_schemas()
RETURNS TEXT[]
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT ARRAY['api'];
$$;


-- =============================================================================
-- Function: pgarachne.capabilities
-- Description: Introspects database to list available JSON-RPC functions.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.capabilities(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    result json;
BEGIN
    WITH api_functions AS (
        SELECT
            n.nspname AS schema_name,
            p.proname AS function_name,
            obj_description(p.oid, 'pg_proc') AS full_comment,
            pg_get_function_arguments(p.oid) as args
        FROM pg_proc AS p
        JOIN pg_namespace AS n ON p.pronamespace = n.oid
        WHERE (n.nspname = ANY(pgarachne.allowed_schemas())
               OR (n.nspname = 'pgarachne' AND p.proname = 'capabilities'))
          AND p.pronargs = 1
          AND p.proargtypes[0] IN ((SELECT oid FROM pg_type WHERE typname IN ('jsonb', 'json')))
    )
    SELECT json_agg(json_build_object(
        'method',
            CASE WHEN af.schema_name || '.' || af.function_name = 'pgarachne.capabilities'
                THEN 'capabilities'
                ELSE af.schema_name || '.' || af.function_name
            END,
        'description', COALESCE(split_part(af.full_comment, E'\n', 1), 'No description'),
        'parameters', json_build_object(
            'type', 'object',
            'properties', COALESCE(
                (substring(af.full_comment from '--- PARAMS ---\s*(\{.*\})'))::jsonb,
                jsonb_build_object('params', jsonb_build_object('type', 'object', 'description', 'Arguments'))
            ),
            'required', jsonb_build_array()
        ),
        'http_method', 'POST',
        'endpoint', '/api/' || current_catalog || '/' || af.schema_name || '.' || af.function_name
    )) INTO result
    FROM api_functions af;

    RETURN result;
END;
$$;

COMMENT ON FUNCTION pgarachne.capabilities(jsonb) IS 'Returns available JSON-RPC methods.';


-- =============================================================================
-- Function: pgarachne.generate_openapi_spec
-- Description: Generates OpenAPI specification.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.generate_openapi_spec(
   server_url_base TEXT,
   db_name TEXT DEFAULT CURRENT_CATALOG
)
RETURNS JSONB
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
AS $$
DECLARE
    paths_object JSONB;
BEGIN
    WITH api_functions AS (
        SELECT
            p.proname AS function_name,
            obj_description(p.oid, 'pg_proc') AS full_comment
        FROM pg_proc AS p
        JOIN pg_namespace AS n ON p.pronamespace = n.oid
        WHERE n.nspname = ANY(pgarachne.allowed_schemas())
            AND p.pronargs = 1
            AND p.proargtypes[0] = (SELECT oid FROM pg_type WHERE typname = 'jsonb')
            AND p.proname <> 'generate_openapi_spec'
    ),
    processed_functions AS (
        SELECT
            function_name,
            split_part(full_comment, E'\n', 1) as summary,
            full_comment as description,
            COALESCE(
                (substring(full_comment from '--- PARAMS ---\s*(\{.*\})'))::jsonb,
                '{}'::jsonb
            ) AS parameter_schema
        FROM api_functions
    )
    SELECT
        jsonb_object_agg(
            '/' || pf.function_name,
            jsonb_build_object(
                'post', jsonb_build_object(
                    'summary', pf.summary,
                    'description', pf.description,
                    'tags', ARRAY['API Functions'],
                    'requestBody', jsonb_build_object(
                        'required', true,
                        'content', jsonb_build_object(
                            'application/json', jsonb_build_object(
                                'schema', jsonb_build_object(
                                    'type', 'object',
                                    'properties', jsonb_build_object(
                                        'jsonrpc', jsonb_build_object('type', 'string', 'example', '2.0'),
                                        'method', jsonb_build_object('type', 'string', 'example', pf.function_name),
                                        'id', jsonb_build_object('type', 'integer', 'example', 1),
                                        'params', pf.parameter_schema
                                    )
                                )
                            )
                        )
                    ),
                    'responses', jsonb_build_object(
                        '200', jsonb_build_object('description', 'Successful JSON-RPC response')
                    ),
                    'security', jsonb_build_array(
                        jsonb_build_object('BearerAuth', '{}'::jsonb)
                    )
                )
            )
        )
    INTO paths_object
    FROM processed_functions AS pf;

    RETURN jsonb_build_object(
        'openapi', '3.0.1',
        'info', jsonb_build_object(
            'title', 'PgArachne API for ''' || CURRENT_CATALOG || ''' database',
            'version', '1.0.0',
            'description', 'Auto-generated OpenAPI spec.'
        ),
        'servers', jsonb_build_array(
            jsonb_build_object(
                'url', server_url_base || '/api/' || CURRENT_CATALOG,
                'description', 'API Server'
            )
        ),
        'paths', COALESCE(paths_object, '{}'::jsonb),
        'components', jsonb_build_object(
            'securitySchemes', jsonb_build_object(
                'BearerAuth', jsonb_build_object(
                    'type', 'http',
                    'scheme', 'bearer',
                    'description', 'Accepts a short-lived JWT or a long-lived API Token.'
                )
            )
        )
    );
END;
$$;
