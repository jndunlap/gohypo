-- Test data migration for development and testing
-- This creates sample data for smoke tests and demonstrations

-- Insert test snapshot
INSERT INTO snapshots (id, dataset, snapshot_at, lag_buffer, cohort_spec_hash, registry_hash, seed)
VALUES ('test-snapshot-001', 'test_dataset', CURRENT_TIMESTAMP, '24 hours',
        'cohort-hash-123', 'registry-hash-456', 42);

-- Insert test variable contracts
INSERT INTO variable_contracts (var_key, as_of_mode, statistical_type, window_days, imputation_policy, scalar_guarantee)
VALUES
    ('inspection_count', 'count_over_window', 'numeric', 30, 'zero_fill', true),
    ('severity_score', 'latest_value_as_of', 'numeric', NULL, 'zero_fill', true),
    ('has_violation', 'exists_as_of', 'binary', NULL, 'false_fill', true),
    ('violation_type', 'latest_value_as_of', 'categorical', NULL, 'unknown_fill', true);

-- Insert sample raw events for testing
-- Generate 100 entities with correlated synthetic data
INSERT INTO raw_events (snapshot_id, source_name, entity_keys, observed_at, payload)
SELECT
    'test-snapshot-001',
    'test_source',
    json_build_object('id', 'entity_' || i::text),
    CURRENT_TIMESTAMP - (random() * interval '365 days'),
    json_build_object(
        'inspection_count', floor(random() * 10)::int,
        'severity_score', round((random() * 100)::numeric, 2),
        'has_violation', random() > 0.7,
        'violation_type', CASE
            WHEN random() > 0.8 THEN 'critical'
            WHEN random() > 0.6 THEN 'major'
            WHEN random() > 0.4 THEN 'minor'
            ELSE 'none'
        END
    )
FROM generate_series(1, 100) AS i;

-- Insert sample artifacts from a hypothetical run
INSERT INTO runs (id, fingerprint, seed, snapshot_id, registry_hash, cohort_hash, stage_list_hash, code_version, status, started_at, completed_at)
VALUES ('test-run-001', 'test-fingerprint-123', 12345, 'test-snapshot-001',
        'registry-hash-456', 'cohort-hash-789', 'stage-hash-abc', 'v1.0.0',
        'completed', CURRENT_TIMESTAMP - interval '1 hour', CURRENT_TIMESTAMP);

-- Insert sample relationship artifacts
INSERT INTO artifacts (id, run_id, kind, payload)
VALUES
    ('artifact-rel-001', 'test-run-001', 'relationship', '{
        "variable_x": "inspection_count",
        "variable_y": "severity_score",
        "test_used": "pearson",
        "effect_size": 0.65,
        "p_value": 0.001,
        "stability_score": 0.85,
        "cohort_size": 100
    }'),
    ('artifact-rel-002', 'test-run-001', 'relationship', '{
        "variable_x": "has_violation",
        "variable_y": "severity_score",
        "test_used": "ttest",
        "effect_size": 0.45,
        "p_value": 0.01,
        "stability_score": 0.78,
        "cohort_size": 100
    }');

-- Insert sample hypothesis
INSERT INTO hypotheses (id, run_id, cause_key, effect_key, mechanism_category,
                       mechanism_desc, confounder_keys, rationale, suggested_rigor)
VALUES ('hypothesis-001', 'test-run-001', 'inspection_count', 'severity_score',
        'direct_causal', 'More inspections may identify more severe issues',
        '["use_defaults"]', 'Based on correlation analysis', 'decision');

-- Insert sample validation result
INSERT INTO validation_results (hypothesis_id, run_id, verdict, rejection_reasons,
                               triage_results, decision_results)
VALUES ('hypothesis-001', 'test-run-001', 'provisional_signal', '[]',
        '{"baseline_signal": true, "beats_phantom": true, "confounder_stress": true}',
        '{"conditional_independence": true, "nested_model_comparison": 0.15, "stability_score": 0.82}');
