-- Universal Table Access Functions (PostgREST-like emulation)
-- These functions provide generic CRUD operations for ANY table in the database.
-- They mimic PostgREST behavior but accept parameters as a single JSONB object.
--
-- DEFAULT SCHEMA: 'api' (if not specified in params)

-- We assume 'api' schema exists (created by main schema.sql conventions usually)
CREATE SCHEMA IF NOT EXISTS api;

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

	-- Security: Validate _order (prevent SQL Injection)
	-- Only allow alphanumeric, underscores, commas, spaces, dots, and standard keywords.
	IF _order IS NOT NULL THEN
		IF _order ~* '[;''"()--]' THEN
			RAISE EXCEPTION 'Potential SQL injection in "order" parameter';
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
