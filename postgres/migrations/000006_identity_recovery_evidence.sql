-- +migrations Up
-- Read only the retained identity, never a prunable payload. This routine has
-- privileged table access without granting a writer direct table visibility.
CREATE FUNCTION audit.identity_record_status(
    supplied_tenant_id text,
    supplied_command_id bytea,
    supplied_event_index integer,
    supplied_policy_version numeric,
    supplied_binding_digest bytea
) RETURNS smallint
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, audit, pg_temp
AS $$
DECLARE
    binding audit.identity_command_bindings%ROWTYPE;
    observed integer := 0;
BEGIN
    IF supplied_tenant_id IS NULL OR octet_length(supplied_tenant_id) NOT BETWEEN 1 AND 64
       OR supplied_command_id IS NULL OR octet_length(supplied_command_id) <> 16
       OR supplied_event_index IS NULL OR supplied_event_index NOT BETWEEN 0 AND 65535
       OR supplied_policy_version IS NULL OR supplied_policy_version <> trunc(supplied_policy_version)
       OR supplied_policy_version NOT BETWEEN 1 AND 18446744073709551615
       OR supplied_binding_digest IS NULL OR octet_length(supplied_binding_digest) <> 32 THEN
        RAISE EXCEPTION 'invalid identity audit evidence scope' USING ERRCODE = '22023';
    END IF;

    -- The existing primary key provides a bounded tenant/command prefix lookup.
    -- Two rows are enough to reject an extra binding in the single-audit profile.
    FOR binding IN
        SELECT bindings.* FROM audit.identity_command_bindings AS bindings
        WHERE bindings.tenant_id = supplied_tenant_id
          AND bindings.command_id = supplied_command_id
        ORDER BY bindings.event_index
        LIMIT 2
    LOOP
        observed := observed + 1;
        IF observed > 1
           OR binding.event_index IS DISTINCT FROM supplied_event_index
           OR binding.retention_policy_version IS DISTINCT FROM supplied_policy_version
           OR binding.binding_digest IS DISTINCT FROM supplied_binding_digest
           OR NOT EXISTS (
               SELECT 1 FROM audit.record_identities AS identities
               WHERE identities.record_id = binding.event_id
                 AND identities.canonical_sha256 = binding.canonical_sha256
           ) THEN
            RETURN 2; -- Unknown/conflicting, never negative evidence.
        END IF;
    END LOOP;
    IF observed = 0 THEN
        RETURN 0; -- No committed participant binding.
    END IF;
    RETURN 1; -- Exact participant effect, not a command success receipt.
END;
$$;

REVOKE ALL ON FUNCTION audit.identity_record_status(text, bytea, integer, numeric, bytea) FROM PUBLIC;
COMMENT ON FUNCTION audit.identity_record_status(text, bytea, integer, numeric, bytea) IS
    'Bounded read-only retained audit evidence for the single-audit identity recovery profile';

-- +migrations Down
DO $identity_audit_recovery_down$
BEGIN
    RAISE EXCEPTION 'identity recovery support requires roll-forward recovery'
        USING ERRCODE = '55000';
END
$identity_audit_recovery_down$;
