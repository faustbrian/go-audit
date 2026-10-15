-- +migrations Up
CREATE TABLE audit.identity_command_bindings (
    tenant_id text COLLATE "C" NOT NULL CHECK (octet_length(tenant_id) BETWEEN 1 AND 64),
    command_id bytea NOT NULL CHECK (octet_length(command_id) = 16),
    event_index integer NOT NULL CHECK (event_index BETWEEN 0 AND 65535),
    event_id text COLLATE "C" NOT NULL UNIQUE,
    canonical_sha256 bytea NOT NULL CHECK (octet_length(canonical_sha256) = 32),
    binding_digest bytea NOT NULL CHECK (octet_length(binding_digest) = 32),
    retention_policy_version numeric(20,0) NOT NULL
        CHECK (retention_policy_version BETWEEN 1 AND 18446744073709551615),
    PRIMARY KEY (tenant_id, command_id, event_index)
);

-- Bindings retain replay identity without copying audit payloads. They are not
-- foreign-keyed to records: legal retention may remove a payload without making
-- its immutable event ID/index reusable or resurrecting it on command replay.
CREATE TRIGGER identity_command_bindings_reject_change
BEFORE UPDATE OR DELETE ON audit.identity_command_bindings
FOR EACH ROW EXECUTE FUNCTION audit.reject_record_identity_change();

CREATE FUNCTION audit.append_identity_record(
    supplied_tenant_id text,
    supplied_command_id bytea,
    supplied_event_index integer,
    supplied_policy_version numeric,
    supplied_binding_digest bytea,
    supplied_canonical bytea
) RETURNS smallint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, audit, pg_temp
AS $$
DECLARE
    document jsonb;
    event_id text;
    fingerprint bytea;
    existing audit.identity_command_bindings%ROWTYPE;
    status smallint;
    inserted integer;
BEGIN
    IF supplied_tenant_id IS NULL OR octet_length(supplied_tenant_id) NOT BETWEEN 1 AND 64
       OR supplied_command_id IS NULL OR octet_length(supplied_command_id) <> 16
       OR supplied_event_index IS NULL OR supplied_event_index NOT BETWEEN 0 AND 65535
       OR supplied_policy_version IS NULL OR supplied_policy_version <> trunc(supplied_policy_version)
       OR supplied_policy_version NOT BETWEEN 1 AND 18446744073709551615
       OR supplied_binding_digest IS NULL OR octet_length(supplied_binding_digest) <> 32
       OR supplied_canonical IS NULL OR octet_length(supplied_canonical) NOT BETWEEN 1 AND 16384 THEN
        RAISE EXCEPTION 'invalid identity audit binding' USING ERRCODE = '22023';
    END IF;
    document := convert_from(supplied_canonical, 'UTF8')::jsonb;
    IF (document #>> '{context,tenant_id}') COLLATE "C" IS DISTINCT FROM supplied_tenant_id COLLATE "C"
       OR (document #>> '{attributes,command.id}') COLLATE "C" IS DISTINCT FROM
          rtrim(translate(encode(supplied_command_id, 'base64'), '+/', '-_'), '=') COLLATE "C"
       OR (document #>> '{attributes,identity.profile}') COLLATE "C" IS DISTINCT FROM 'identity.security.v1' COLLATE "C"
       OR (document #>> '{attributes,retention.class}') IS NULL
       OR (document #>> '{attributes,retention.class}') COLLATE "C" NOT IN ('standard', 'protected') THEN
        RAISE EXCEPTION 'invalid identity audit binding' USING ERRCODE = '22023';
    END IF;
    event_id := document ->> 'id';
    fingerprint := sha256(supplied_canonical);

    INSERT INTO audit.identity_command_bindings AS bindings (
        tenant_id, command_id, event_index, event_id, canonical_sha256,
        binding_digest, retention_policy_version
    ) VALUES (
        supplied_tenant_id, supplied_command_id, supplied_event_index, event_id,
        fingerprint, supplied_binding_digest, supplied_policy_version
    ) ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS inserted = ROW_COUNT;

    SELECT bindings.* INTO existing FROM audit.identity_command_bindings AS bindings
    WHERE bindings.tenant_id = supplied_tenant_id
      AND bindings.command_id = supplied_command_id
      AND bindings.event_index = supplied_event_index;
    IF NOT FOUND OR existing.event_id IS DISTINCT FROM event_id
       OR existing.canonical_sha256 IS DISTINCT FROM fingerprint
       OR existing.binding_digest IS DISTINCT FROM supplied_binding_digest
       OR existing.retention_policy_version IS DISTINCT FROM supplied_policy_version THEN
        RAISE EXCEPTION 'identity audit binding conflict' USING ERRCODE = '23505';
    END IF;
    IF inserted = 0 THEN
        -- Missing retained event identity is corruption, not permission to
        -- recreate a deleted payload or fabricate recovery evidence.
        IF NOT EXISTS (SELECT 1 FROM audit.record_identities AS identities
            WHERE identities.record_id = event_id AND identities.canonical_sha256 = fingerprint) THEN
            RAISE EXCEPTION 'identity audit binding integrity failure' USING ERRCODE = '55000';
        END IF;
        RETURN 2;
    END IF;

    status := audit.append_record(
        event_id, (document ->> 'occurred_at')::timestamptz,
        (document ->> 'recorded_at')::timestamptz, supplied_tenant_id,
        (document #>> '{actor,kind}')::smallint, NULLIF(document #>> '{actor,id}', ''),
        document #>> '{subject,type}', document #>> '{subject,id}', document ->> 'action',
        (document ->> 'outcome')::smallint, NULLIF(document #>> '{context,correlation_id}', ''),
        supplied_canonical, fingerprint
    );
    IF status NOT IN (1, 2) THEN
        RAISE EXCEPTION 'identity audit event conflict' USING ERRCODE = '23505';
    END IF;
    RETURN status;
END;
$$;

REVOKE ALL ON audit.identity_command_bindings FROM PUBLIC;
REVOKE ALL ON FUNCTION audit.append_identity_record(text, bytea, integer, numeric, bytea, bytea) FROM PUBLIC;

COMMENT ON TABLE audit.identity_command_bindings IS
    'Internal immutable same-transaction command/index to canonical audit identity binding';

-- +migrations Down
DO $identity_audit_down$
BEGIN
    RAISE EXCEPTION 'identity audit bindings cannot be reversed without losing replay evidence'
        USING ERRCODE = '55000';
END
$identity_audit_down$;
