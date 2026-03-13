-- =============================================================================
-- MCP Utility Functions
-- =============================================================================

CREATE SCHEMA IF NOT EXISTS pgarachne_mcp;
GRANT USAGE ON SCHEMA pgarachne_mcp TO public;

CREATE OR REPLACE FUNCTION pgarachne_mcp.mcp_wrap(
    data jsonb,
    status TEXT DEFAULT 'success',
    trace_id TEXT DEFAULT NULL,
    meta jsonb DEFAULT '{}'::jsonb
)
RETURNS jsonb
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT jsonb_build_object(
        'status', status,
        'data', data,
        'meta', meta,
        'trace_id':= trace_id
    );
$$;

CREATE OR REPLACE FUNCTION pgarachne_mcp.rpc_to_mcp(rpc_json json)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    rpc_obj json;
    method TEXT;
    params json;
    id TEXT;
    result_json json;
    error_obj json;
BEGIN
    rpc_obj = rpc_json;
    
    IF rpc_obj->>'jsonrpc' = '2.0' THEN
        IF rpc_obj->>'method' = 'capabilities' THEN
            -- Capabilities response transformation
            result_json = rpc_obj->'result';
            RETURN pgarachne_mcp.mcp_wrap(
                result_json,
                CASE WHEN rpc_obj->>'error' IS NOT NULL THEN 'failure' ELSE 'success' END,
                rpc_obj->>'id'
            );
        ELSE
            -- Regular method response
            IF rpc_obj->>'error' IS NOT NULL THEN
                error_obj = rpc_obj->'error';
                RETURN pgarachne_mcp.mcp_wrap(
                    jsonb_build_object(
                        'error', error_obj->>'message',
                        'code', error_obj->>'code'
                    ),
                    'failure',
                    rpc_obj->>'id'
                );
            ELSE
                result_json = rpc_obj->'result';
                RETURN pgarachne_mcp.mcp_wrap(
                    result_json,
                    'success',
                    rpc_obj->>'id'
                );
            END IF;
        END IF;
    ELSE
        RETURN pgarachne_mcp.mcp_wrap(
            jsonb_build_object('error', 'Unknown JSON-RPC version'),
            'failure'
        );
    END IF;
END;
$$;


-- =============================================================================
-- Updated: pgarachne.capabilities
-- =============================================================================

CREATE OR REPLACE FUNCTION pgarachne.capabilities(params jsonb DEFAULT '{}'::jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
    result jsonb;
BEGIN
    WITH api_functions AS (
        SELECT
            n.nspname AS schema_name,
            p.proname AS function_name,
            obj_description(p.oid, 'pg_proc') AS full_comment,
            p.proargtypes[0]::regtype AS return_type,
            p.protransform AS is_transformed
        FROM pg_proc AS p
        JOIN pg_namespace AS n ON p.pronamespace = n.oid
        WHERE (n.nspname = ANY(pgarachne.allowed_schemas())
                OR (n.nspname = 'pgarachne' AND p.proname = 'capabilities'))
            AND p.pronargs = 1
            AND p.proargtypes[0] IN ((SELECT oid FROM pg_type WHERE typname IN ('jsonb', 'json')))
            AND has_function_privilege(current_user, p.oid, 'EXECUTE')
    )
    SELECT jsonb_agg(
        jsonb_build_object(
            'method',
            CASE WHEN af.schema_name || '.' || af.function_name = 'pgarachne.capabilities'
                 THEN 'capabilities'
                 ELSE af.schema_name || '.' || af.function_name
            END,
            'description', COALESCE(split_part(af.full_comment, E'\n', 1), 'No description'),
            'parameters', jsonb_build_object(
                'type', 'object',
                'properties', COALESCE(
                    (substring(af.full_comment from '--- PARAMS ---\s*(\{.*\})'))::jsonb,
                    jsonb_build_object('params', jsonb_build_object('type', 'object', 'description', 'Arguments'))
                ),
                'required', jsonb_build_array()
            ),
            'response', jsonb_build_object(
                'type', 'object',
                'properties', jsonb_build_object(
                    'data', jsonb_build_object(
                        'type', 'object',
                        'description', 'MCP-formatted response'
                    ),
                    'meta', jsonb_build_object(
                        'type', 'object',
                        'description', 'Additional metadata'
                    ),
                    'status', jsonb_build_object(
                        'type', 'string',
                        'enum', jsonb_build_array('success', 'failure'),
                        'description', 'Call status'
                    )
                ),
                'required', jsonb_build_array('data', 'status')
            ),
            'http_method', 'POST',
            'endpoint', '/api/' || current_catalog
        )
    ) INTO result
    FROM api_functions af;

    RETURN pgarachne_mcp.mcp_wrap(
        result,
        'success'
    );
END;
$$;


-- =============================================================================
-- Updated: pgarachne.generate_openapi_spec
-- =============================================================================

CREATE OR REPLACE FUNCTION pgarachne.generate_openapi_spec(
   server_url_base TEXT,
   db_name TEXT DEFAULT CURRENT_CATALOG
)
RETURNS JSONB
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
STABLE
AS $$
DECLARE
    paths_object JSONB;
BEGIN
    paths_object := jsonb_build_object(
        '/',
        jsonb_build_object(
            'post', jsonb_build_object(
                'summary', 'MCP endpoint',
                'description', 'Call database functions with MCP format.',
                'tags', ARRAY['MCP'],
                'requestBody', jsonb_build_object(
                    'required', true,
                    'content', jsonb_build_object(
                        'application/json', jsonb_build_object(
                            'schema', jsonb_build_object(
                                'type', 'object',
                                'properties', jsonb_build_object(
                                    'data', jsonb_build_object(
                                        'type', 'object',
                                        'properties', jsonb_build_object(
                                            'method', jsonb_build_object('type', 'string', 'example', 'api.hello_world'),
                                            'params', jsonb_build_object('type', 'object', 'description', 'Function arguments'),
                                            'id', jsonb_build_object('type', 'string', 'example', 'abc123')
                                        ),
                                        'required', jsonb_build_array('method')
                                    ),
                                    'meta', jsonb_build_object(
                                        'type', 'object',
                                        'properties', jsonb_build_object(
                                            'trace_id', jsonb_build_object('type', 'string', 'example', 'trace-123456')
                                        )
                                    )
                                ),
                                'required', jsonb_build_array('data')
                            )
                        )
                    )
                ),
                'responses', jsonb_build_object(
                    '200', jsonb_build_object(
                        'description', 'Successful MCP response',
                        'content', jsonb_build_object(
                            'application/json', jsonb_build_object(
                                'schema', jsonb_build_object(
                                    'type', 'object',
                                    'properties', jsonb_build_object(
                                        'data', jsonb_build_object(
                                            'type', 'object',
                                            'properties', jsonb_build_object(
                                                'result', jsonb_build_object('type', 'object'),
                                                'error', jsonb_build_object('type', 'object')
                                            )
                                        ),
                                        'meta', jsonb_build_object(
                                            'type', 'object',
                                            'properties', jsonb_build_object(
                                                'trace_id', jsonb_build_object('type', 'string')
                                            )
                                        ),
                                        'status', jsonb_build_object(
                                            'type', 'string',
                                            'enum', jsonb_build_array('success', 'failure')
                                        )
                                    ),
                                    'required', jsonb_build_array('data', 'status')
                                )
                            )
                        )
                    ),
                    '400', jsonb_build_object(
                        'description', 'Invalid request format'
                    ),
                    '401', jsonb_build_object(
                        'description', 'Authentication failed'
                    )
                ),
                'security', jsonb_build_array(
                    jsonb_build_object('BearerAuth', '{}'::jsonb)
                )
            )
        )
    );

    RETURN jsonb_build_object(
        'openapi', '3.0.1',
        'info', jsonb_build_object(
            'title', 'PgArachne API for ''' || CURRENT_CATALOG || ''' database',
            'version', '1.0.0',
            'description', 'Auto-generated OpenAPI spec. MCP format.'
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
            ),
            'examples', jsonb_build_object(
                'mcp_response', jsonb_build_object(
                    'summary', 'Sample MCP response',
                    'value', jsonb_build_object(
                        'data', jsonb_build_object(
                            'result', jsonb_build_object(
                                'greeting', 'Hello, world!'
                            ),
                            'meta', jsonb_build_object(
                                'request_id', 'req-12345',
                                'response_time_ms', 12
                            )
                        ),
                        'meta', jsonb_build_object(
                            'trace_id', 'trace-123456',
                            'span_id', 'span-789'
                        ),
                        'status', 'success'
                    )
                )
            )
        )
    );
END;
$$;


-- =============================================================================
-- Updated: JSON-RPC wrapper functions
-- =============================================================================

CREATE OR REPLACE FUNCTION pgarachne.rpc_response(
    result TEXT,
    id TEXT,
    error TEXT DEFAULT NULL,
    code TEXT DEFAULT NULL
)
RETURNS json
LANGUAGE sql
AS $$
    SELECT json_build_object(
        'jsonrpc', '2.0',
        'id', id,
        'result':= result,
        'error':= json_build_object('message', error, 'code', code)
    );
$$;


CREATE OR REPLACE FUNCTION pgarachne.rpc_response(
    result jsonb,
    id TEXT,
    error TEXT DEFAULT NULL,
    code TEXT DEFAULT NULL
)
RETURNS json
LANGUAGE sql
AS $$
    SELECT json_build_object(
        'jsonrpc', '2.0',
        'id', id,
        'result':= result,
        'error':= json_build_object('message', error, 'code', code)
    );
$$;


CREATE OR REPLACE FUNCTION pgarachne.rpc_error(
    error TEXT,
    id TEXT,
    code TEXT DEFAULT '500'
)
RETURNS json
LANGUAGE sql
AS $$
    SELECT json_build_object(
        'jsonrpc', '2.0',
        'id', id,
        'error':= json_build_object('message', error, 'code', code)
    );
$$;


-- =============================================================================
-- Sample API function demonstrating MCP format
-- =============================================================================

CREATE OR REPLACE FUNCTION api.hello_world(params jsonb)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN pgarachne_mcp.mcp_wrap(
        jsonb_build_object(
            'greeting', 'Hello, world!',
            'params_received', params
        ),
        'success',
        'hello_world_' || md5(random()::text)
    );
END;
$$;

COMMENT ON FUNCTION api.hello_world(jsonb) IS 'Returns a greeting message.';
GRANT EXECUTE ON FUNCTION api.hello_world(jsonb) TO public;

