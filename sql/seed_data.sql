-- Insert test users into api.users
-- Password hash is MD5 of 'password' (prefixed with 'md5' if using postgres md5 auth, but schema says application does SHA256 usually, 
-- however schema comments say: "Ukládáme MD5 hash ('md5' + hash) pro kompatibilitu". 
-- Wait, `schema.sql` line 137 says: "Aplikace přímo dotazuje tabulku api_tokens s SHA-256 hashem."
-- But `api.users` line 91 says: "Ukládáme MD5 hash ('md5' + hash) pro kompatibilitu".
-- Let's stick to what likely works or put placeholders. Since this is "seed data", let's use some dummy hashes.

-- Users table is removed.
-- Instead, ensure that PostgreSQL roles exist if you want to test with them.
-- For example:
-- CREATE ROLE johndoe WITH LOGIN PASSWORD 'password';
-- GRANT pgarachne_anon TO johndoe; -- If using inheritance for permissions

-- Insert test tokens
-- Note: In a real scenario, these hashes should match actual tokens.
-- Here we just insert dummy data for demonstration.
-- Insert test tokens
-- Using the new function to generate a token for testing. Not idempotent, so wrapped in DO block or just allowed to fail if run multiple times (though it returns random token, so it won't conflict on hash).
-- Actually, for seed data, we might want a KNOWN token.
-- But the hashed storage prevents us from inserting a known "raw" token easily unless we pre-calculate hash.
-- Let's manual insert a known hash for testing 'seed_token'.
-- SHA256('seed_token') = 14aca461.....
INSERT INTO pgarachne.api_tokens (role, token_hash, description)
VALUES ('pgarachne_anon', encode(digest('seed_token', 'sha256'), 'hex'), 'Seed token (raw: seed_token)')
ON CONFLICT (token_hash) DO NOTHING;




-- -----------------------------------------------------------------------------
-- Function: api.server_info
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

