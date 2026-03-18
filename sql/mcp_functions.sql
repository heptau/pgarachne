-- =============================================================================
-- PgArachne MCP Extensions
-- Adds SQL backing for the MCP resources and prompts protocol methods.
--
-- Append this block to sql/schema.sql after the existing content.
-- =============================================================================


-- =============================================================================
-- Table: pgarachne.prompts
-- Description: Named prompt templates exposed via the MCP prompts/* methods.
--
-- Templates use {{variable}} placeholders for argument substitution:
--   INSERT INTO pgarachne.prompts (name, description, template, arguments)
--   VALUES (
--     'summarize_table',
--     'Ask the model to summarize a table',
--     'Summarize the contents of the {{table}} table in plain language.',
--     '[{"name":"table","description":"Table to summarize","required":true}]'
--   );
-- =============================================================================
CREATE TABLE IF NOT EXISTS pgarachne.prompts (
    name        TEXT PRIMARY KEY,
    description TEXT,
    template    TEXT    NOT NULL,
    -- JSON array of {name, description, required} — mirrors the MCP argument
    -- descriptor shape so no transformation is needed on read.
    arguments   JSONB   NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT prompts_name_nonempty CHECK (name <> ''),
    CONSTRAINT prompts_template_nonempty CHECK (template <> '')
);

COMMENT ON TABLE pgarachne.prompts IS 'Named prompt templates for the MCP prompts/* methods.';

-- Allow any authenticated role to read prompt templates.
-- Prompts are non-sensitive read-only configuration; write access stays
-- restricted to the service user (pgarachne) who owns the schema.
GRANT SELECT ON pgarachne.prompts TO public;

-- Reuse the existing update_timestamp trigger function.
CREATE TRIGGER update_prompts_timestamp
BEFORE UPDATE ON pgarachne.prompts
FOR EACH ROW
EXECUTE FUNCTION pgarachne.update_timestamp();


-- =============================================================================
-- Function: pgarachne.mcp_list_resources
-- Description: Lists all tables and views the current role may SELECT from,
-- formatted as MCP resource descriptors.
--
-- Resource URI scheme: db:///{schema}/{relation}
-- Excludes system schemas (pg_catalog, information_schema, pgarachne).
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.mcp_list_resources(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    result json;
BEGIN
    SELECT json_build_object(
        'resources', COALESCE(json_agg(r ORDER BY r->>'uri'), '[]'::json)
    ) INTO result
    FROM (
        SELECT json_build_object(
            'uri',         'db:///' || n.nspname || '/' || c.relname,
            'name',        n.nspname || '.' || c.relname,
            'description', COALESCE(obj_description(c.oid, 'pg_class'), ''),
            'mimeType',    'application/json'
        ) AS r
        FROM pg_class c
        JOIN pg_namespace n ON c.relnamespace = n.oid
        WHERE c.relkind IN ('r', 'v', 'm', 'p')   -- tables, views, matviews, partitioned
          AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pgarachne')
          AND n.nspname NOT LIKE 'pg_%'
          AND has_table_privilege(current_user, c.oid, 'SELECT')
    ) sub;

    RETURN result;
END;
$$;

COMMENT ON FUNCTION pgarachne.mcp_list_resources(jsonb) IS 'Returns all tables and views selectable by the current role as MCP resource descriptors.';
GRANT EXECUTE ON FUNCTION pgarachne.mcp_list_resources(jsonb) TO public;


-- =============================================================================
-- Function: pgarachne.mcp_read_resource
-- Description: Reads up to 100 rows from a table or view identified by a
-- db:///schema/table URI and returns them as an MCP resource content block.
--
-- Params:
--   uri  TEXT  Resource URI in the form db:///{schema}/{relation}
--
-- Security:
--   1. Parses schema and table from the URI.
--   2. Validates existence AND has_table_privilege before the dynamic query.
--   3. Uses %I identifier quoting — not vulnerable to SQL injection.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.mcp_read_resource(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
AS $$
DECLARE
    uri          TEXT;
    uri_path     TEXT;
    parts        TEXT[];
    schema_name  TEXT;
    table_name   TEXT;
    row_data     json;
    max_rows     CONSTANT INT := 100;
BEGIN
    uri := params->>'uri';
    IF uri IS NULL OR uri = '' THEN
        RAISE EXCEPTION 'uri parameter is required';
    END IF;

    -- Strip the db:/// prefix, then split schema and table.
    uri_path := regexp_replace(uri, '^db:///', '');
    parts := string_to_array(uri_path, '/');

    IF array_length(parts, 1) IS DISTINCT FROM 2
        OR parts[1] = '' OR parts[2] = ''
    THEN
        RAISE EXCEPTION 'Invalid resource URI. Expected db:///schema/table, got: %', uri;
    END IF;

    schema_name := parts[1];
    table_name  := parts[2];

    -- Validate existence and privilege before running the dynamic query.
    -- This is the security gate — the %I format below is safe only because
    -- we've already confirmed the identifiers refer to a real, accessible object.
    IF NOT EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON c.relnamespace = n.oid
        WHERE n.nspname = schema_name
          AND c.relname = table_name
          AND c.relkind IN ('r', 'v', 'm', 'p')
          AND has_table_privilege(current_user, c.oid, 'SELECT')
    ) THEN
        RAISE EXCEPTION 'Resource not found or access denied: %', uri;
    END IF;

    EXECUTE format(
        'SELECT COALESCE(json_agg(t), ''[]''::json) FROM (SELECT * FROM %I.%I LIMIT %s) t',
        schema_name, table_name, max_rows
    ) INTO row_data;

    RETURN json_build_object(
        'contents', json_build_array(
            json_build_object(
                'uri',      uri,
                'mimeType', 'application/json',
                'text',     row_data::text
            )
        )
    );
END;
$$;

COMMENT ON FUNCTION pgarachne.mcp_read_resource(jsonb) IS 'Reads up to 100 rows from the table or view identified by a db:///schema/table URI.
--- PARAMS ---
{"uri": {"type": "string", "description": "Resource URI in the form db:///schema/table"}}';
GRANT EXECUTE ON FUNCTION pgarachne.mcp_read_resource(jsonb) TO public;


-- =============================================================================
-- Function: pgarachne.mcp_list_prompts
-- Description: Returns all rows from pgarachne.prompts as MCP prompt descriptors.
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.mcp_list_prompts(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE sql
STABLE
AS $$
    SELECT json_build_object(
        'prompts', COALESCE(
            json_agg(
                json_build_object(
                    'name',        name,
                    'description', COALESCE(description, ''),
                    'arguments',   COALESCE(arguments, '[]'::jsonb)
                ) ORDER BY name
            ),
            '[]'::json
        )
    )
    FROM pgarachne.prompts;
$$;

COMMENT ON FUNCTION pgarachne.mcp_list_prompts(jsonb) IS 'Returns all stored prompt templates as MCP prompt descriptors.';
GRANT EXECUTE ON FUNCTION pgarachne.mcp_list_prompts(jsonb) TO public;


-- =============================================================================
-- Function: pgarachne.mcp_get_prompt
-- Description: Returns a specific prompt template with {{variable}} placeholders
-- substituted by values from the arguments map.
--
-- Params:
--   name       TEXT    Prompt name (required)
--   arguments  OBJECT  Key/value pairs for template substitution (optional)
--
-- Example:
--   SELECT pgarachne.mcp_get_prompt('{"name":"summarize_table","arguments":{"table":"orders"}}');
-- =============================================================================
CREATE OR REPLACE FUNCTION pgarachne.mcp_get_prompt(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    prompt_name   TEXT;
    prompt_rec    RECORD;
    rendered_text TEXT;
    arg_key       TEXT;
    arg_val       TEXT;
BEGIN
    prompt_name := params->>'name';
    IF prompt_name IS NULL OR prompt_name = '' THEN
        RAISE EXCEPTION 'name parameter is required';
    END IF;

    SELECT * INTO prompt_rec
    FROM pgarachne.prompts
    WHERE name = prompt_name;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Prompt not found: %', prompt_name;
    END IF;

    -- Substitute {{key}} placeholders with values from params.arguments.
    rendered_text := prompt_rec.template;
    IF (params->'arguments') IS NOT NULL
        AND jsonb_typeof(params->'arguments') = 'object'
    THEN
        FOR arg_key, arg_val IN
            SELECT key, value FROM jsonb_each_text(params->'arguments')
        LOOP
            rendered_text := replace(rendered_text, '{{' || arg_key || '}}', arg_val);
        END LOOP;
    END IF;

    RETURN json_build_object(
        'description', COALESCE(prompt_rec.description, ''),
        'messages', json_build_array(
            json_build_object(
                'role', 'user',
                'content', json_build_object(
                    'type', 'text',
                    'text', rendered_text
                )
            )
        )
    );
END;
$$;

COMMENT ON FUNCTION pgarachne.mcp_get_prompt(jsonb) IS 'Returns a rendered prompt template with {{variable}} placeholders substituted.
--- PARAMS ---
{
    "name": {"type": "string", "description": "Prompt name"},
    "arguments": {"type": "object", "description": "Key/value pairs for template substitution"}
}';
GRANT EXECUTE ON FUNCTION pgarachne.mcp_get_prompt(jsonb) TO public;
