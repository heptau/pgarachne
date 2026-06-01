-- Universal Table Access Functions (PostgREST-like emulation)
-- These functions provide generic CRUD operations for ANY table in the database.
-- They mimic PostgREST behavior but accept parameters as a single JSONB object.
--
-- DEFAULT SCHEMA: 'api' (if not specified in params)
--
-- SECURITY: All four functions validate _schema and _table against a strict
-- whitelist pattern ([a-z_][a-z0-9_]*) before building dynamic SQL. format()'s
-- %I specifier also escapes identifiers safely, so this is defense in depth
-- — the goal is to fail fast with a clear error rather than bubble up a
-- cryptic "invalid identifier" message from PostgreSQL, and to keep the
-- surface area limited to plain lowercase names. Schemas like pg_catalog
-- and information_schema are reachable only by callers who can EXECUTE
-- these functions, so the validation pattern is the first line of guard.

-- We assume 'api' schema exists (created by main schema.sql conventions usually)
CREATE SCHEMA IF NOT EXISTS api;

-- =============================================================================
-- Helper: api._validate_identifier
-- Description: Raises an exception if the supplied string is not a safe
--              unquoted PostgreSQL identifier. Used by all universal_*
--              functions to validate _schema and _table before dynamic SQL.
-- =============================================================================
CREATE OR REPLACE FUNCTION api._validate_identifier(_name text, _role text)
RETURNS void
LANGUAGE plpgsql
IMMUTABLE
AS $fn$
BEGIN
	IF _name IS NULL OR _name !~ '^[a-z_][a-z0-9_]*$' THEN
		RAISE EXCEPTION 'Invalid %: % (must match [a-z_][a-z0-9_]*)', _role, _name;
	END IF;
END;
$fn$;

COMMENT ON FUNCTION api._validate_identifier(text, text) IS 'Internal helper. Raises if the identifier is not a safe unquoted PostgreSQL name ([a-z_][a-z0-9_]*). Note: this validates *shape* only — it does NOT restrict which schemas/tables may be targeted. Access control still relies on the calling role''s privileges, just like direct SQL.';

-- =============================================================================
-- 1. READ (GET)
-- =============================================================================
CREATE OR REPLACE FUNCTION api.universal_read(_params jsonb)
RETURNS json AS $$
DECLARE
	_schema text;
	_table  text;
	_cols   text;
	_limit  int;
	_offset int;
	_order  text;
	_filters jsonb;
	_where  text := 'TRUE';
	_key    text;
	_val    text;
	_query  text;
	_result json;
BEGIN
	-- Extract parameters
	_schema := COALESCE(_params->>'schema', 'api');
	_table  := _params->>'table';
	IF _table IS NULL THEN
		RAISE EXCEPTION 'Parameter "table" is required.';
	END IF;
	PERFORM api._validate_identifier(_schema, 'schema');
	PERFORM api._validate_identifier(_table, 'table');

	_cols   := COALESCE(_params->>'select', '*');
	_limit  := COALESCE((_params->>'limit')::int, 10);
	_offset := COALESCE((_params->>'offset')::int, 0);
	_order  := _params->>'order';
	_filters := COALESCE(_params->'filters', '{}'::jsonb);

	-- Security: Validate _cols (prevent SQL Injection)
	-- Only allow alphanumeric, underscores, commas, spaces, dots, and asterisk.
	IF _cols !~* '^[a-z0-9_,\.\* ]+$' THEN
		RAISE EXCEPTION 'Invalid characters in "select" parameter';
	END IF;

	-- Security: Validate _order (prevent SQL Injection).
	-- Whitelist pattern: comma-separated identifiers, each optionally followed
	-- by ASC or DESC. Rejects subqueries, semicolons, parentheses, and any
	-- other SQL keywords.
	IF _order IS NOT NULL THEN
		IF _order !~* '^\s*[a-z_][a-z0-9_]*(\s+(ASC|DESC))?\s*(,\s*[a-z_][a-z0-9_]*(\s+(ASC|DESC))?\s*)*$' THEN
			RAISE EXCEPTION 'Invalid "order" parameter: % (expected comma-separated column names with optional ASC/DESC)', _order;
		END IF;
	END IF;

	-- Build WHERE clause
	FOR _key, _val IN
		SELECT * FROM jsonb_each_text(_filters)
	LOOP
		-- Note: This only supports equality checks. Safe due to %I and %L.
		_where := _where || format(' AND %I = %L', _key, _val);
	END LOOP;

	-- Construct the query
	_query := format('SELECT COALESCE(json_agg(t), ''[]'') FROM (SELECT %s FROM %I.%I WHERE %s', _cols, _schema, _table, _where);

	IF _order IS NOT NULL THEN
		-- Note: %s is used because _order acts as a clause, but we validated it above.
		_query := _query || format(' ORDER BY %s', _order);
	END IF;

	_query := _query || format(' LIMIT %L OFFSET %L) t', _limit, _offset);

	EXECUTE _query INTO _result;

	RETURN _result;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION api.universal_read(jsonb) IS 'Generic Read function.
--- PARAMS ---
{
  "schema": "api (default)",
  "table": "users (required)",
  "select": "* (default)",
  "limit": 10,
  "offset": 0,
  "order": "id DESC",
  "filters": { "id": 1 }
}';


-- =============================================================================
-- 2. CREATE (POST)
-- =============================================================================
CREATE OR REPLACE FUNCTION api.universal_create(_params jsonb)
RETURNS json AS $$
DECLARE
	_schema text;
	_table  text;
	_data   jsonb;
	_result json;
BEGIN
	_schema := COALESCE(_params->>'schema', 'api');
	_table  := _params->>'table';
	_data   := _params->'data';

	IF _table IS NULL OR _data IS NULL THEN
		RAISE EXCEPTION 'Parameters "table" and "data" are required.';
	END IF;
	PERFORM api._validate_identifier(_schema, 'schema');
	PERFORM api._validate_identifier(_table, 'table');

	-- Note: Returns a single object. If strict PostgREST emulation is desired, should return array.
	-- Keeping as single object for now as input is single object.
	EXECUTE format(
		'INSERT INTO %I.%I SELECT * FROM json_populate_record(NULL::%I.%I, $1) RETURNING row_to_json(*)',
		_schema, _table, _schema, _table
	) USING _data INTO _result;

	RETURN _result;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION api.universal_create(jsonb) IS 'Generic Create function.
--- PARAMS ---
{
  "schema": "api (default)",
  "table": "users (required)",
  "data": { "name": "John", "role": "admin" }
}';


-- =============================================================================
-- 3. UPDATE (PATCH)
-- =============================================================================
CREATE OR REPLACE FUNCTION api.universal_update(_params jsonb)
RETURNS json AS $$
DECLARE
	_schema text;
	_table  text;
	_data   jsonb;
	_filters jsonb;
	_where  text := 'TRUE';
	_set_clause text;
	_key    text;
	_val    text;
	_result json;
BEGIN
	_schema := COALESCE(_params->>'schema', 'api');
	_table  := _params->>'table';
	_data   := _params->'data';
	_filters := COALESCE(_params->'filters', '{}'::jsonb);

	IF _table IS NULL OR _data IS NULL THEN
		RAISE EXCEPTION 'Parameters "table" and "data" are required.';
	END IF;
	PERFORM api._validate_identifier(_schema, 'schema');
	PERFORM api._validate_identifier(_table, 'table');

	-- Build SET clause
	SELECT string_agg(format('%I = %L', key, value), ', ')
	INTO _set_clause
	FROM jsonb_each_text(_data);

	IF _set_clause IS NULL THEN
		RAISE EXCEPTION 'No data provided for update';
	END IF;

	-- Build WHERE clause
	FOR _key, _val IN
		SELECT * FROM jsonb_each_text(_filters)
	LOOP
		_where := _where || format(' AND %I = %L', _key, _val);
	END LOOP;

	-- Use CTE to handle multiple updated rows and return them as a JSON array
	EXECUTE format(
		'WITH rows AS (UPDATE %I.%I SET %s WHERE %s RETURNING *) SELECT COALESCE(json_agg(rows), ''[]''::json) FROM rows',
		_schema, _table, _set_clause, _where
	) INTO _result;

	RETURN _result;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION api.universal_update(jsonb) IS 'Generic Update function.
--- PARAMS ---
{
  "schema": "api (default)",
  "table": "users (required)",
  "data": { "active": true },
  "filters": { "id": 1 }
}';


-- =============================================================================
-- 4. DELETE
-- =============================================================================
CREATE OR REPLACE FUNCTION api.universal_delete(_params jsonb)
RETURNS json AS $$
DECLARE
	_schema text;
	_table  text;
	_filters jsonb;
	_where  text := 'TRUE';
	_key    text;
	_val    text;
	_result json;
BEGIN
	_schema := COALESCE(_params->>'schema', 'api');
	_table  := _params->>'table';
	_filters := COALESCE(_params->'filters', '{}'::jsonb);

	IF _table IS NULL THEN
		RAISE EXCEPTION 'Parameter "table" is required.';
	END IF;
	PERFORM api._validate_identifier(_schema, 'schema');
	PERFORM api._validate_identifier(_table, 'table');

	-- Build WHERE clause
	FOR _key, _val IN
		SELECT * FROM jsonb_each_text(_filters)
	LOOP
		_where := _where || format(' AND %I = %L', _key, _val);
	END LOOP;

	-- Use CTE to handle multiple deleted rows and return them as a JSON array
	EXECUTE format(
		'WITH rows AS (DELETE FROM %I.%I WHERE %s RETURNING *) SELECT COALESCE(json_agg(rows), ''[]''::json) FROM rows',
		_schema, _table, _where
	) INTO _result;

	RETURN _result;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION api.universal_delete(jsonb) IS 'Generic Delete function.
--- PARAMS ---
{
  "schema": "api (default)",
  "table": "users (required)",
  "filters": { "id": 1 }
}';
