-- Migration 003: Add registry versioning for immutability
-- This creates proper audit trails for contract changes

-- Create registry_versions table (should have been in 001 but adding now)
CREATE TABLE IF NOT EXISTS registry_versions (
    registry_hash TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    payload JSONB NOT NULL -- Full contracts snapshot
);

-- Add foreign key constraint to snapshots
ALTER TABLE snapshots
ADD CONSTRAINT fk_snapshots_registry_hash
FOREIGN KEY (registry_hash) REFERENCES registry_versions(registry_hash);

-- Update existing snapshots to point to a default registry version
-- This is a one-time migration for existing data
INSERT INTO registry_versions (registry_hash, payload)
SELECT
    'default-registry-hash',
    jsonb_build_object('contracts', jsonb_agg(
        jsonb_build_object(
            'var_key', var_key,
            'as_of_mode', as_of_mode,
            'statistical_type', statistical_type,
            'window_days', window_days,
            'imputation_policy', imputation_policy,
            'scalar_guarantee', scalar_guarantee
        )
    ))
FROM variable_contracts
ON CONFLICT (registry_hash) DO NOTHING;

-- Update snapshots to use the registry hash
UPDATE snapshots
SET registry_hash = 'default-registry-hash'
WHERE registry_hash NOT IN (SELECT registry_hash FROM registry_versions);

-- Create a function to compute registry hash (for application use)
CREATE OR REPLACE FUNCTION compute_registry_hash() RETURNS TEXT AS $$
DECLARE
    contract_record RECORD;
    hash_data TEXT := '';
BEGIN
    FOR contract_record IN
        SELECT var_key, as_of_mode, statistical_type, window_days,
               imputation_policy, scalar_guarantee
        FROM variable_contracts
        ORDER BY var_key
    LOOP
        hash_data := hash_data || contract_record.var_key || '|' ||
                    contract_record.as_of_mode || '|' ||
                    contract_record.statistical_type || '|' ||
                    COALESCE(contract_record.window_days::TEXT, 'null') || '|' ||
                    contract_record.imputation_policy || '|' ||
                    contract_record.scalar_guarantee::TEXT || ';';
    END LOOP;

    RETURN encode(digest(hash_data, 'sha256'), 'hex');
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Function to snapshot current registry
CREATE OR REPLACE FUNCTION snapshot_registry() RETURNS TEXT AS $$
DECLARE
    current_hash TEXT;
    contracts_json JSONB;
BEGIN
    -- Compute current hash
    current_hash := compute_registry_hash();

    -- Check if we already have this version
    IF EXISTS (SELECT 1 FROM registry_versions WHERE registry_hash = current_hash) THEN
        RETURN current_hash;
    END IF;

    -- Build contracts JSON
    SELECT jsonb_agg(
        jsonb_build_object(
            'var_key', var_key,
            'as_of_mode', as_of_mode,
            'statistical_type', statistical_type,
            'window_days', window_days,
            'imputation_policy', imputation_policy,
            'scalar_guarantee', scalar_guarantee,
            'created_at', created_at
        ) ORDER BY var_key
    ) INTO contracts_json
    FROM variable_contracts;

    -- Insert new registry version
    INSERT INTO registry_versions (registry_hash, payload)
    VALUES (current_hash, jsonb_build_object('contracts', contracts_json));

    RETURN current_hash;
END;
$$ LANGUAGE plpgsql;
