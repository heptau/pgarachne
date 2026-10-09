-- PgArachne browser demo — run once in your database (after sql/schema.sql):
--   psql -d my_database -f setup.sql

-- 1. A login role for the browser user (change the password!).
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'webuser') THEN
        CREATE ROLE webuser LOGIN PASSWORD 'webuser_password';
    END IF;
END $$;

-- 2. A schema for functions that are exposed through the API.
CREATE SCHEMA IF NOT EXISTS api;
GRANT USAGE ON SCHEMA api TO webuser;

-- 3. JSON-RPC: a normal function — one jsonb argument, returns json.
CREATE OR REPLACE FUNCTION api.greet(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE sql AS $$
    SELECT json_build_object(
        'message', 'Hello, ' || COALESCE(params->>'name', 'world') || '!',
        'database_user', current_user,
        'server_time', now()
    );
$$;

-- 4. SSE: a function that publishes a message on the "demo" channel.
CREATE OR REPLACE FUNCTION api.send_message(params jsonb DEFAULT '{}'::jsonb)
RETURNS json
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('demo', json_build_object(
        'from', current_user,
        'text', params->>'text'
    )::text);
    RETURN '{"sent": true}'::json;
END;
$$;

-- 5. FILE: a set-returning function with the columns path + content (bytea),
--    optionally mime_type and store_only. One row = one file; more rows = ZIP.
CREATE OR REPLACE FUNCTION api.export_report(params jsonb DEFAULT '{}'::jsonb)
RETURNS TABLE (path text, content bytea, mime_type text, store_only boolean)
LANGUAGE sql AS $$
    SELECT 'report.csv',
           convert_to('id,name' || E'\n' || '1,Alice' || E'\n' || '2,Bob' || E'\n', 'UTF8'),
           'text/csv',
           false;
$$;

-- 6. Permissions: PostgreSQL decides who may call what. Grant EXECUTE on
--    exactly the functions the browser user needs — nothing else is reachable.
GRANT EXECUTE ON FUNCTION api.greet(jsonb)         TO webuser;
GRANT EXECUTE ON FUNCTION api.send_message(jsonb)  TO webuser;
GRANT EXECUTE ON FUNCTION api.export_report(jsonb) TO webuser;
