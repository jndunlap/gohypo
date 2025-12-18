-- GoHypo Database Schema v2 - Fixed for Replayability
-- Append-only design with proper temporal queries

-- Registry versions - immutable contract snapshots
CREATE TABLE registry_versions (
    registry_hash TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    payload JSONB NOT NULL -- Full contracts snapshot
);

-- Snapshots - time machine handles (no snapshot_id in raw_events)
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,
    dataset TEXT NOT NULL,
    snapshot_at TIMESTAMP NOT NULL,
    lag_seconds INTEGER NOT NULL DEFAULT 86400, -- 24 hours in seconds
    registry_hash TEXT NOT NULL REFERENCES registry_versions(registry_hash),
    cohort_hash TEXT NOT NULL,
    seed BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Variable contracts - current state (mutable for evolution)
CREATE TABLE variable_contracts (
    var_key TEXT PRIMARY KEY,
    as_of_mode TEXT NOT NULL CHECK (as_of_mode IN ('latest_value_as_of', 'count_over_window', 'sum_over_window', 'exists_as_of')),
    statistical_type TEXT NOT NULL CHECK (statistical_type IN ('numeric', 'categorical', 'binary', 'timestamp')),
    window_days INTEGER,
    imputation_policy TEXT NOT NULL DEFAULT 'zero_fill',
    scalar_guarantee BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Raw events - append-only fact storage (no snapshot_id for cross-snapshot queries)
CREATE TABLE raw_events (
    id SERIAL PRIMARY KEY,
    source_name TEXT NOT NULL,
    entity_id TEXT NOT NULL, -- Generated column for performance
    entity_keys JSONB NOT NULL, -- {"id": "entity_123", ...}
    observed_at TIMESTAMP NOT NULL,
    payload JSONB NOT NULL, -- {"field1": "value1", ...}
    ingested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add generated columns for indexing performance
ALTER TABLE raw_events ADD COLUMN entity_id TEXT
    GENERATED ALWAYS AS (entity_keys->>'id') STORED;

-- Runs - execution tracking with immutable fingerprint
CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,
    seed BIGINT NOT NULL,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    registry_hash TEXT NOT NULL REFERENCES registry_versions(registry_hash),
    cohort_hash TEXT NOT NULL,
    stage_list_hash TEXT NOT NULL,
    code_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Run stages - pipeline execution tracking
CREATE TABLE run_stages (
    id SERIAL PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    stage_name TEXT NOT NULL,
    stage_kind TEXT NOT NULL CHECK (stage_kind IN ('stats', 'battery', 'audit')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms BIGINT,
    error_message TEXT,
    metrics JSONB, -- {"processed_count": 100, "success_count": 95, ...}
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Artifacts - append-only knowledge base
CREATE TABLE artifacts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    kind TEXT NOT NULL, -- 'relationship', 'hypothesis', 'run_verdict', etc.
    payload JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Hypotheses - immutable once created
CREATE TABLE hypotheses (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    cause_key TEXT NOT NULL,
    effect_key TEXT NOT NULL,
    mechanism_category TEXT NOT NULL,
    mechanism_desc TEXT,
    confounder_keys JSONB,
    rationale TEXT,
    suggested_rigor TEXT CHECK (suggested_rigor IN ('triage', 'decision')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Validation results - immutable falsification records
CREATE TABLE validation_results (
    id SERIAL PRIMARY KEY,
    hypothesis_id TEXT NOT NULL REFERENCES hypotheses(id),
    run_id TEXT NOT NULL,
    verdict TEXT NOT NULL CHECK (verdict IN ('provisional_signal', 'noise', 'confounded', 'unstable', 'inadmissible')),
    rejection_reasons JSONB,
    triage_results JSONB,
    decision_results JSONB,
    validated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance (optimized for temporal queries)
CREATE INDEX idx_raw_events_entity_observed ON raw_events(entity_id, observed_at DESC);
CREATE INDEX idx_raw_events_observed ON raw_events(observed_at);
CREATE INDEX idx_raw_events_payload ON raw_events USING GIN(payload);
CREATE INDEX idx_artifacts_run_kind ON artifacts(run_id, kind);
CREATE INDEX idx_artifacts_kind ON artifacts(kind);
CREATE INDEX idx_hypotheses_run ON hypotheses(run_id);
CREATE INDEX idx_validation_hypothesis ON validation_results(hypothesis_id);

-- Constraints for data integrity
ALTER TABLE snapshots ADD CONSTRAINT snapshot_at_future_check CHECK (snapshot_at <= CURRENT_TIMESTAMP);
ALTER TABLE raw_events ADD CONSTRAINT observed_at_not_future CHECK (observed_at <= CURRENT_TIMESTAMP);
ALTER TABLE raw_events ADD CONSTRAINT ingested_at_not_future CHECK (ingested_at <= CURRENT_TIMESTAMP);
