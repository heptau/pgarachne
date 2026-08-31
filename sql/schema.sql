-- =============================================================================
-- PgArachne Schema Definition
-- =============================================================================

-- Create schema for PgArachne internal functionality
CREATE SCHEMA IF NOT EXISTS pgarachne;
COMMENT ON SCHEMA pgarachne IS 'Schema for PgArachne internal functionality (tokens, system functions).';
GRANT USAGE ON SCHEMA pgarachne TO public;

-- Create schema for public API functions
CREATE SCHEMA IF NOT EXISTS api;
COMMENT ON SCHEMA api IS 'Schema for user-defined JSON-RPC functions exposed via PgArachne.';
GRANT USAGE ON SCHEMA api TO public;

-- Extension: pgcrypto (Required for hashing and random generation)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =============================================================================
-- Role: pgarachne_admin
-- Description: Admin role for managing API tokens (minting).
-- =============================================================================
DO $$
BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgarachne_admin')
	THEN
		BEGIN
			CREATE ROLE pgarachne_admin;
		EXCEPTION
			WHEN insufficient_privilege
			THEN
				RAISE NOTICE 'Skipping CREATE ROLE pgarachne_admin (insufficient privileges).';
		END;
	END IF;

	IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgarachne_admin')
		AND EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgarachne')
	THEN
		BEGIN
			GRANT pgarachne_admin TO pgarachne;
		EXCEPTION
			WHEN insufficient_privilege
			THEN
				RAISE NOTICE 'Skipping GRANT pgarachne_admin TO pgarachne (insufficient privileges).';
		END;
	END IF;
END;
$$;


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

DROP TRIGGER IF EXISTS update_api_tokens_timestamp ON pgarachne.api_tokens;

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
	IF token_valid_to IS NOT NULL AND token_valid_to <= NOW()
	THEN
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
-- Restrict token minting to a dedicated admin role only to avoid public minting.
-- Use a separate admin group so the proxy user (DB_USER) does not need to be granted extra privileges.
REVOKE EXECUTE ON FUNCTION pgarachne.add_api_token(TEXT, TEXT, TIMESTAMPTZ) FROM public;
GRANT EXECUTE ON FUNCTION pgarachne.add_api_token(TEXT, TEXT, TIMESTAMPTZ) TO pgarachne_admin;


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
REVOKE EXECUTE ON FUNCTION pgarachne.verify_api_token(TEXT) FROM public;
GRANT EXECUTE ON FUNCTION pgarachne.verify_api_token(TEXT) TO pgarachne;


-- =============================================================================
-- Function: pgarachne.allowed_schemas
-- Description: Returns list of schemas exposed via API.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.allowed_schemas()
RETURNS TEXT[]
LANGUAGE sql
IMMUTABLE
AS $$
	SELECT ARRAY['api'];
$$;


-- =============================================================================
-- Function: pgarachne.api_prefix
-- Description: Returns the URL path prefix used by the running PgArachne
--              instance. PgArachne sets the GUC `app.api_prefix` on every
--              connection, so generated endpoint URLs in capabilities() and
--              generate_openapi_spec() match the actual server route.
--              The GUC may be unset for ad-hoc psql sessions — in that case
--              we fall back to the documented default of 'db' (the value the
--              Go server uses when API_PREFIX is empty).
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.api_prefix()
RETURNS TEXT
LANGUAGE sql
STABLE
AS $$
	SELECT COALESCE(NULLIF(current_setting('app.api_prefix', true), ''), 'db');
$$;

COMMENT ON FUNCTION pgarachne.api_prefix() IS 'Returns the API path prefix (first URL segment) used by the running PgArachne instance.';
GRANT EXECUTE ON FUNCTION pgarachne.api_prefix() TO public;


-- =============================================================================
-- Function: pgarachne.capabilities
-- Description: Introspects database to list available JSON-RPC functions.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.capabilities(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
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
			AND has_function_privilege(current_user, p.oid, 'EXECUTE')
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
        'endpoint', '/' || pgarachne.api_prefix() || '/' || current_catalog || '/jsonrpc'
	)) INTO result
	FROM api_functions af;

	RETURN result;
END;
$$;

COMMENT ON FUNCTION pgarachne.capabilities(jsonb) IS 'Returns available JSON-RPC methods.';
GRANT EXECUTE ON FUNCTION pgarachne.capabilities(jsonb) TO public;


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
SET search_path = pg_catalog
STABLE
AS $$
DECLARE
    methods_array   JSONB;
    method_descriptions JSONB;
    method_paths    JSONB;
    path_template   TEXT;
    methods_list    TEXT;
BEGIN
    -- Deliberately SECURITY INVOKER (the default — no SECURITY DEFINER
    -- clause above): capabilities() filters by has_function_privilege
    -- (current_user, ...), and current_user only reflects the caller's
    -- authenticated role if this function runs as invoker. A SECURITY
    -- DEFINER function forces current_user to its owner for its entire
    -- execution, including nested calls to capabilities() below — that
    -- would silently defeat the role-based filtering the Go handler
    -- relies on (it does SET LOCAL ROLE before calling this function).
    --
    -- Pull the list of callable methods from the same source that the
    -- capabilities() method uses, so the spec stays in sync with what
    -- is actually executable, and reflects only what the calling role
    -- may call. The first element of the array is the synthetic
    -- "capabilities" entry that lists the database itself.
    methods_array := COALESCE(pgarachne.capabilities()::jsonb, '[]'::jsonb);

    -- Build the per-method extension block. Each entry carries the
    -- JSON-RPC method name, its first-line description, and the
    -- parameters schema parsed from the function's --- PARAMS ---
    -- comment marker (or a generic placeholder when absent).
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'name',        method_entry->>'method',
        'description', COALESCE(method_entry->>'description', 'No description'),
        'parameters',  COALESCE(method_entry->'parameters', '{}'::jsonb)
    )), '[]'::jsonb)
    INTO method_descriptions
    FROM jsonb_array_elements(methods_array) AS method_entry;

    -- Aggregate the method names into a comma-separated string for the
    -- human-readable description. Order follows the source order, so
    -- the most "official" methods (capabilities, etc.) appear first.
    SELECT string_agg(method_entry->>'method', ', ' ORDER BY ord)
    INTO methods_list
    FROM jsonb_array_elements(methods_array)
    WITH ORDINALITY AS t(method_entry, ord);

    path_template := '/' || pgarachne.api_prefix() || '/' || CURRENT_CATALOG || '/jsonrpc';

    -- One native `paths` entry per exposed method, at a virtual
    -- /rpc/{method} address. These paths do not exist as real HTTP
    -- routes — every actual call still goes through the single
    -- /jsonrpc endpoint above — they exist purely so tooling that
    -- expects one operation per path (Swagger UI, Postman, codegen)
    -- can list and describe each method individually. Each operation's
    -- description spells out the real JSON-RPC request needed to
    -- invoke it. Method names may contain dots (api.hello_world),
    -- which need no escaping as either a jsonb object key or an
    -- OpenAPI path segment.
    SELECT COALESCE(jsonb_object_agg(
        '/' || pgarachne.api_prefix() || '/' || CURRENT_CATALOG || '/rpc/' || (method_entry->>'method'),
        jsonb_build_object(
            'post', jsonb_build_object(
                'operationId', replace(method_entry->>'method', '.', '_'),
                'summary',     'Call ' || (method_entry->>'method'),
                'description', COALESCE(method_entry->>'description', 'No description') ||
                               E'\n\nThis path is a documentation-only placeholder — there is no real HTTP route at this address. ' ||
                               'To actually call this method, send: POST ' || path_template ||
                               ' with body {"jsonrpc":"2.0","method":"' || (method_entry->>'method') ||
                               '","params":<params matching the schema below>,"id":1}',
                'tags',        jsonb_build_array(
                    CASE WHEN (method_entry->>'method') = 'capabilities'
                        THEN 'pgarachne'
                        ELSE split_part(method_entry->>'method', '.', 1)
                    END
                ),
                'requestBody', jsonb_build_object(
                    'required', true,
                    'content', jsonb_build_object(
                        'application/json', jsonb_build_object(
                            'schema', COALESCE(method_entry->'parameters', jsonb_build_object('type', 'object'))
                        )
                    )
                ),
                'responses', jsonb_build_object(
                    '200', jsonb_build_object(
                        'description', 'Successful call (via /jsonrpc) — result is the function''s JSON return value',
                        'content', jsonb_build_object(
                            'application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcResponse'))
                        )
                    ),
                    '400', jsonb_build_object('description', 'Malformed JSON-RPC request or invalid parameters',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                    '401', jsonb_build_object('description', 'Missing or invalid Authorization header',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                    '403', jsonb_build_object('description', 'Authenticated but not permitted to call this method',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                    '404', jsonb_build_object('description', 'Unknown method',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                    '409', jsonb_build_object('description', 'Idempotency key has been used before',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                    '500', jsonb_build_object('description', 'Server-side error while executing the SQL function',
                        'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError'))))
                ),
                'security', jsonb_build_array(jsonb_build_object('BearerAuth', '{}'::jsonb))
            )
        )
    ), '{}'::jsonb)
    INTO method_paths
    FROM jsonb_array_elements(methods_array) AS method_entry;

    RETURN jsonb_build_object(
        'openapi', '3.1.0',
        'info', jsonb_build_object(
            'title',       'PgArachne API for ''' || CURRENT_CATALOG || ''' database',
            'summary',     'JSON-RPC gateway for PostgreSQL functions',
            'version',     '1.0.0',
            'description', 'Auto-generated OpenAPI 3.1 spec. Set the JSON-RPC `method` field to one of: ' ||
                            COALESCE(methods_list, '(no methods exposed)')
        ),
        'servers', jsonb_build_array(
            jsonb_build_object(
                -- The bare origin, not origin+/jsonrpc: every `paths` key
                -- below (both /jsonrpc and the per-method /rpc/* entries) is
                -- already a full absolute path, and per OpenAPI 3.1's URL
                -- resolution rule (server.url + path key) the server entry
                -- must therefore be just the origin, or URLs built by
                -- tooling from this spec would double up the path.
                'url',         server_url_base,
                'description', 'PgArachne API base URL'
            )
        ),
        'paths', jsonb_build_object(
            path_template,
            jsonb_build_object(
                'post', jsonb_build_object(
                    'summary',     'Invoke any exposed database function via JSON-RPC',
                    'description', 'All exposed methods: ' || COALESCE(methods_list, '(none)'),
                    'tags',        ARRAY['JSON-RPC'],
                    'requestBody', jsonb_build_object(
                        'required', true,
                        'content', jsonb_build_object(
                            'application/json', jsonb_build_object(
                                'schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcRequest')
                            )
                        )
                    ),
                    'responses', jsonb_build_object(
                        '200', jsonb_build_object(
                            'description', 'Successful JSON-RPC response (result field populated)',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcResponse')))
                        ),
                        '400', jsonb_build_object('description', 'Malformed JSON-RPC request',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                        '401', jsonb_build_object('description', 'Missing or invalid Authorization header',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                        '409', jsonb_build_object('description', 'Idempotency key has been used before',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                        '429', jsonb_build_object('description', 'Login rate limit exceeded',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError')))),
                        '500', jsonb_build_object('description', 'Server-side error while executing the SQL function',
                            'content', jsonb_build_object('application/json', jsonb_build_object('schema', jsonb_build_object('$ref', '#/components/schemas/JsonRpcError'))))
                    ),
                    'security', jsonb_build_array(
                        jsonb_build_object('BearerAuth', '{}'::jsonb)
                    ),
                    -- PgArachne-specific extension. OpenAPI 3.1 explicitly
                    -- permits x-* extensions at any level, and we use this
                    -- slot to expose the per-method metadata that the single
                    -- JSON-RPC POST operation cannot convey in the standard
                    -- schema. Tools that understand the extension can render
                    -- a method picker; others ignore it safely.
                    'x-pgarachne-methods', method_descriptions
                )
            )
        ) || method_paths,
        'components', jsonb_build_object(
            'schemas', jsonb_build_object(
                'JsonRpcRequest', jsonb_build_object(
                    'type', 'object',
                    'properties', jsonb_build_object(
                        'jsonrpc', jsonb_build_object(
                            'type',    'string',
                            'const',   '2.0',
                            'examples', jsonb_build_array('2.0')
                        ),
                        'method', jsonb_build_object(
                            'type',    'string',
                            'examples', jsonb_build_array('api.hello_world')
                        ),
                        'id', jsonb_build_object(
                            'type',     jsonb_build_array('integer', 'string', 'null'),
                            'examples', jsonb_build_array(1, 'abc', NULL)
                        ),
                        'params', jsonb_build_object(
                            'type',        'object',
                            'description', 'Function arguments, passed through to the SQL function as jsonb.'
                        ),
                        'idempotencyKey', jsonb_build_object(
                            'type',        'string',
                            'description', 'Optional non-standard extension. When present, a previously-seen key returns HTTP 409 Conflict.'
                        )
                    ),
                    'required', jsonb_build_array('jsonrpc', 'method')
                ),
                'JsonRpcResponse', jsonb_build_object(
                    'type', 'object',
                    'properties', jsonb_build_object(
                        'jsonrpc', jsonb_build_object('type', 'string', 'const', '2.0'),
                        'result',  jsonb_build_object('description', 'Function result. Shape depends on the called method.'),
                        'id',      jsonb_build_object('type', jsonb_build_array('integer', 'string', 'null'))
                    )
                ),
                'JsonRpcError', jsonb_build_object(
                    'type', 'object',
                    'properties', jsonb_build_object(
                        'jsonrpc', jsonb_build_object('type', 'string', 'const', '2.0'),
                        'error',   jsonb_build_object(
                            'type', 'object',
                            'properties', jsonb_build_object(
                                'code',    jsonb_build_object('type', 'integer'),
                                'message', jsonb_build_object('type', 'string')
                            ),
                            'required', jsonb_build_array('message')
                        ),
                        'id', jsonb_build_object('type', jsonb_build_array('integer', 'string', 'null'))
                    )
                )
            ),
            'securitySchemes', jsonb_build_object(
                'BearerAuth', jsonb_build_object(
                    'type',        'http',
                    'scheme',      'bearer',
                    'description', 'Accepts a short-lived JWT or a long-lived API token.'
                )
            )
        )
    );
END;
$$;

COMMENT ON FUNCTION pgarachne.generate_openapi_spec(text, text)
    IS 'Build an OpenAPI 3.1 spec for the current database, filtered to the methods the calling role may execute. server_url_base is the public base URL (e.g. https://api.example.com). The second argument is accepted for API symmetry and currently unused; the function always reflects the database it is invoked from. paths includes the real /jsonrpc endpoint plus one virtual /rpc/{method} path per exposed method (documentation only — real calls always go through /jsonrpc).';
GRANT EXECUTE ON FUNCTION pgarachne.generate_openapi_spec(text, text) TO public;

--

CREATE TABLE IF NOT EXISTS pgarachne.requests (
    idempotency_id uuid PRIMARY KEY,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

-- Index speeds up the periodic cleanup below. Without it, every sweep
-- would do a sequential scan on a table that grows without bound.
CREATE INDEX IF NOT EXISTS requests_created_at_idx
    ON pgarachne.requests (created_at);


CREATE OR REPLACE FUNCTION pgarachne.to_uuid(_text text)
RETURNS uuid
LANGUAGE sql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $fn$

	SELECT
		CASE
			WHEN pg_input_is_valid(_text, 'uuid') THEN _text
			WHEN pg_input_is_valid(_text, 'bigint') THEN lpad(to_hex(CASE WHEN pg_input_is_valid(_text, 'bigint') THEN _text END::bigint), 32, '0')
			ELSE md5(_text)
		END::uuid;

$fn$;

COMMENT ON FUNCTION pgarachne.to_uuid(text) IS 'Smart UUID cast: preserves valid UUIDs, converts BIGINTs to 32-char hex, and uses MD5 hash as fallback for other strings.';
GRANT EXECUTE ON FUNCTION pgarachne.to_uuid(text) TO public;

CREATE OR REPLACE FUNCTION pgarachne.save_idempotency_key(_key text)
RETURNS boolean
LANGUAGE SQL
STRICT
AS $fn$

   WITH inserted AS (
      INSERT INTO pgarachne.requests (idempotency_id)
      VALUES (pgarachne.to_uuid(_key))
      ON CONFLICT (idempotency_id) DO NOTHING
      RETURNING TRUE
   )
   SELECT EXISTS (SELECT FROM inserted);

$fn$;

COMMENT ON FUNCTION pgarachne.save_idempotency_key(text) IS 'Atomically reserves an idempotency key for the current request. Returns TRUE on first use, FALSE on a duplicate (caller should reject as a replay). Cleanup is the operator''s responsibility — see pgarachne.cleanup_idempotency_keys().';
GRANT EXECUTE ON FUNCTION pgarachne.save_idempotency_key(text) TO public;


-- =============================================================================
-- Function: pgarachne.cleanup_idempotency_keys
-- Description: Removes idempotency keys older than the given retention window.
--              Call this from cron, pg_cron, or any external scheduler. A
--              sensible default is once per hour with a 24h window.
--              Returns the number of rows deleted.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.cleanup_idempotency_keys(_older_than interval DEFAULT '24 hours')
RETURNS bigint
LANGUAGE SQL
AS $fn$

   WITH deleted AS (
      DELETE FROM pgarachne.requests
      WHERE created_at < NOW() - _older_than
      RETURNING 1
   )
   SELECT COUNT(*)::bigint FROM deleted;

$fn$;

COMMENT ON FUNCTION pgarachne.cleanup_idempotency_keys(interval) IS 'Deletes idempotency keys older than the supplied interval (default 24h). Schedule via pg_cron or an external scheduler. Returns the number of rows removed.';
GRANT EXECUTE ON FUNCTION pgarachne.cleanup_idempotency_keys(interval) TO public;
