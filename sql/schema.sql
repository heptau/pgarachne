-- Create api schema if it does not exist
CREATE SCHEMA IF NOT EXISTS pgarachne;

COMMENT ON SCHEMA pgarachne
   IS 'Schema for PgArachne functionality contain tokens and base api function';


-- -----------------------------------------------------------------------------
-- Extension: pgcrypto (Required for hashing and random generation)
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- -----------------------------------------------------------------------------
-- User: pgarachne (Service User)
-- -----------------------------------------------------------------------------
DO $$
BEGIN
	IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'pgarachne')
	THEN
		CREATE ROLE pgarachne WITH LOGIN PASSWORD 'change_me_in_production';
		-- IMPORTANT: In production, change the password and grant necessary roles:
		-- GRANT my_app_user TO pgarachne;
	END IF;
END
$$;
COMMENT ON ROLE pgarachne IS 'Service user for the PgArachne gateway. Must be granted permission to SET ROLE to application users.';
-- -----------------------------------------------------------------------------
-- Table: pgarachne.api_tokens
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pgarachne.api_tokens (
	token_hash TEXT PRIMARY KEY, -- Stores the SHA-256 hash of the token (PK implies unique)
	role TEXT NOT NULL, -- The database role this token authenticates as
	description TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	valid_to TIMESTAMPTZ, -- Optional expiration

	CONSTRAINT updated_at_check CHECK (updated_at >= created_at),
	CONSTRAINT valid_to_check CHECK (valid_to IS NULL OR valid_to >= updated_at)
);

-- Index on token_hash is automatic due to PRIMARY KEY
-- CREATE INDEX IF NOT EXISTS api_tokens_token_hash_idx ON pgarachne.api_tokens (token_hash);

COMMENT ON TABLE pgarachne.api_tokens IS 'Stores long-lived API tokens for authentication.';

-- Trigger for updated_at
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

-- -----------------------------------------------------------------------------
-- Function: pgarachne.create_api_token
-- -----------------------------------------------------------------------------
-- -----------------------------------------------------------------------------
-- Function: pgarachne.add_api_token
-- Generates a random token, hashes it, and saves it. Returns the raw token.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION pgarachne.add_api_token(
	token_description TEXT,
	target_role TEXT DEFAULT CURRENT_USER,
	token_valid_to TIMESTAMPTZ DEFAULT NULL
)
RETURNS TEXT -- Returns the raw new token (one-time view)
LANGUAGE plpgsql
AS $$
DECLARE
	raw_token TEXT;
	hashed_token TEXT;
BEGIN
	-- Check if valid_to is in the past (if provided)
	IF token_valid_to IS NOT NULL AND token_valid_to <= NOW() THEN
		RAISE EXCEPTION 'valid_to must be in the future';
	END IF;

	-- Generate a securely random token (32 bytes hex = 64 chars)
	raw_token := encode(gen_random_bytes(32), 'hex');

	-- Hash it using SHA-256
	hashed_token := encode(digest(raw_token, 'sha256'), 'hex');

	-- Insert into table
	INSERT INTO pgarachne.api_tokens (role, token_hash, description, valid_to)
	VALUES (target_role, hashed_token, token_description, token_valid_to);

	-- Return the raw token so it can be shown to the user once
	RETURN raw_token;
END;
$$;

COMMENT ON FUNCTION pgarachne.add_api_token(TEXT, TEXT, TIMESTAMPTZ)
IS 'Generates a new API token for the specified role.
--- PARAMS ---
{
	"token_description": {"type": "string", "description": "Human readable description"},
	"target_role": {"type": "string", "description": "Database role to impersonate (default: current_user)"},
	"token_valid_to": {"type": "string", "format": "date-time", "description": "Expiration time (optional)"}
}';

-- -----------------------------------------------------------------------------
-- Function: pgarachne.verify_app_token
-- Verifies a raw token and returns the associated role if valid.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION pgarachne.verify_api_token(
	input_raw_token TEXT
)
RETURNS TEXT -- Returns the role name if valid, NULL otherwise
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
   found_role TEXT;
   input_hash TEXT;
BEGIN
   -- Hash the input token
   input_hash := encode(digest(input_raw_token, 'sha256'), 'hex');

   SELECT role INTO found_role
   FROM pgarachne.api_tokens
   WHERE token_hash = input_hash
   	AND (valid_to IS NULL OR valid_to > NOW());

   RETURN found_role;
END;
$$;

CREATE OR REPLACE FUNCTION pgarachne.allowed_schemas()
RETURNS TEXT[]
LANGUAGE sql
IMMUTABLE
AS $$

	SELECT ARRAY['api'];

$$;

COMMENT ON FUNCTION pgarachne.allowed_schemas() IS 'Returns an array of allowed schemas for API access.';

CREATE OR REPLACE FUNCTION pgarachne.generate_openapi_spec(
   server_url_base TEXT, -- E.g., 'http://localhost:8080'
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

	-- Step 1: Find all suitable functions and extract their metadata.
	WITH api_functions AS (
		SELECT
			p.proname AS function_name,
			-- Get the full comment of the function
			obj_description(p.oid, 'pg_proc') AS full_comment
		FROM pg_proc AS p
		JOIN pg_namespace AS n ON p.pronamespace = n.oid
		WHERE n.nspname = ANY(pgarachne.allowed_schemas())
			AND p.pronargs = 1 -- Must have exactly one argument
			-- Argument must be of type jsonb
			AND p.proargtypes[0] = (SELECT oid FROM pg_type WHERE typname = 'jsonb')
			-- Return type must be json or jsonb
			AND p.prorettype IN (
				(SELECT oid FROM pg_type WHERE typname = 'json'),
				(SELECT oid FROM pg_type WHERE typname = 'jsonb')
			)
			-- Exclude the generator function itself
			AND p.proname <> 'generate_openapi_spec'
	),
	-- Step 2: Process comments to extract summary, description, and parameter schema.
	processed_functions AS (
		SELECT
			function_name,
			-- First line of the comment is the summary
			split_part(full_comment, E'\n', 1) as summary,
			-- The full comment is the description
			full_comment as description,
			-- Regex to extract the JSON block after '--- PARAMS ---'
			COALESCE(
				(substring(full_comment from '--- PARAMS ---\s*(\{.*\})'))::jsonb,
				'{}'::jsonb
			) AS parameter_schema
		FROM api_functions
	)
	-- Step 3: Build the 'paths' object for OpenAPI.
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
										'params', pf.parameter_schema -- Inject extracted schema
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

	-- Step 4: Build the final OpenAPI JSONB object.
	RETURN jsonb_build_object(
		'openapi', '3.0.1',
		'info', jsonb_build_object(
			'title', 'PgArachne API for ''' || CURRENT_CATALOG || ''' database',
			'version', '1.0.0',
			'description', 'Dynamically generated OpenAPI specification for a PostgreSQL JSON-RPC API. The true documentation for each function is in its comment within the database.'
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

----

CREATE SCHEMA IF NOT EXISTS api;

COMMENT ON SCHEMA api
   IS 'Schema for PgArachne functions that return JSON for JSON-RPC endpoints, optimized for AI consumption with detailed metadata';


-- Function: api.capabilities
-- Lists available functions in the api schema dynamically by parsing comments
CREATE OR REPLACE FUNCTION api.capabilities(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
SECURITY DEFINER
AS $$
DECLARE
	result json;
BEGIN
	WITH api_functions AS (
		SELECT
			p.proname AS function_name,
			obj_description(p.oid, 'pg_proc') AS full_comment,
			pg_get_function_arguments(p.oid) as args
		FROM pg_proc AS p
		JOIN pg_namespace AS n ON p.pronamespace = n.oid
		WHERE n.nspname IN ANY(pgarachne.allowed_schemas())
		  AND (
				-- API functions: 1 arg of type jsonb/json
				(n.nspname = 'api' AND p.pronargs = 1
				 AND p.proargtypes[0] IN ((SELECT oid FROM pg_type WHERE typname = 'json'), (SELECT oid FROM pg_type WHERE typname = 'jsonb')))
				OR
				-- PgArachne functions: add_api_token
				(n.nspname = 'pgarachne' AND p.proname = 'add_api_token')
		  )
		  AND p.proname NOT IN ('capabilities', 'generate_openapi_spec', 'verify_api_token', 'update_timestamp') -- Exclude utility functions
	)
	SELECT json_agg(json_build_object(
		'method', af.function_name,
		'description', COALESCE(split_part(af.full_comment, E'\n', 1), 'No description'),
		'parameters', json_build_object(
			'type', 'object',
			'properties', COALESCE(
				(substring(af.full_comment from '--- PARAMS ---\s*(\{.*\})'))::jsonb,
				jsonb_build_object('params', jsonb_build_object('type', 'object', 'description', 'Arguments'))
			),
			'required', jsonb_build_array() -- Dynamic required fields parsing is complex, defaulting empty for now or could parse from JSON schema if detailed
		),
		'http_method', 'POST',
		'endpoint', '/api/' || af.function_name
	)) INTO result
	FROM api_functions af;

	RETURN result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION api.capabilities(jsonb)
	IS 'Returns a JSON array of available JSON-RPC methods in the api schema with their names, descriptions, parameter schemas, HTTP method (POST), and endpoint paths. Takes an empty JSONB object as input. Accessible via POST /capabilities with JSON-RPC payload {"jsonrpc": "2.0", "method": "capabilities", "params": {}, "id": 1}.';
