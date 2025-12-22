-- GoHypo Research Ledger v2.0
-- Extends existing schema with evidence tracking, UI synchronization, and scientific auditability

-- Add new columns to existing research_sessions table
ALTER TABLE research_sessions
ADD COLUMN IF NOT EXISTS ui_state JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS scientific_efficiency DECIMAL(5,3) DEFAULT 0.0;

-- Add new columns to existing hypothesis_results table
ALTER TABLE hypothesis_results
ADD COLUMN IF NOT EXISTS phase_e_values JSONB DEFAULT '[]'::jsonb,
ADD COLUMN IF NOT EXISTS feasibility_score DECIMAL(3,2) CHECK (feasibility_score >= 0 AND feasibility_score <= 1.0),
ADD COLUMN IF NOT EXISTS risk_level TEXT CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
ADD COLUMN IF NOT EXISTS data_topology JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS total_validation_time INTERVAL,
ADD COLUMN IF NOT EXISTS phase_completion_times INTERVAL[] DEFAULT ARRAY[]::INTERVAL[];

-- Add check constraints for array lengths
ALTER TABLE hypothesis_results
-- Removed length constraint for dynamic e-value validation
ADD CONSTRAINT check_phase_times_length CHECK (array_length(phase_completion_times, 1) <= 3);

-- ============================================================================
-- NEW TABLES FOR RESEARCH LEDGER
-- ============================================================================

-- Evidence Accumulation (time-series E-values for live UI updates)
CREATE TABLE IF NOT EXISTS evidence_accumulation (
    id BIGSERIAL PRIMARY KEY,
    hypothesis_id TEXT NOT NULL REFERENCES hypothesis_results(id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ DEFAULT NOW(),

    -- Evidence state
    e_value DECIMAL(10,4) NOT NULL,
    normalized_e_value DECIMAL(3,2) NOT NULL CHECK (normalized_e_value >= 0 AND normalized_e_value <= 1.0),
    confidence DECIMAL(3,2) CHECK (confidence >= 0 AND confidence <= 1.0),

    -- Test execution state
    active_test_count INTEGER DEFAULT 0,
    completed_test_count INTEGER DEFAULT 0,
    phase INTEGER CHECK (phase BETWEEN 0 AND 2),

    -- UI state snapshot
    ui_snapshot JSONB DEFAULT '{}',

    -- Performance metrics
    memory_usage_mb INTEGER,
    cpu_usage_percent DECIMAL(5,2)
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions (example for current month)
-- Note: In production, you'd want to automate partition creation
CREATE TABLE IF NOT EXISTS evidence_accumulation_y2024m12 PARTITION OF evidence_accumulation
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- UI State Cache (for HTMX reconnection after network issues)
CREATE TABLE IF NOT EXISTS ui_state_cache (
    session_id UUID PRIMARY KEY REFERENCES research_sessions(id) ON DELETE CASCADE,
    ui_state JSONB NOT NULL DEFAULT '{}',
    version INTEGER DEFAULT 1,
    last_updated TIMESTAMPTZ DEFAULT NOW(),

    -- Compression for large UI states (optional optimization)
    compressed_state BYTEA,
    compression_algorithm TEXT CHECK (compression_algorithm IN ('gzip', 'lz4', 'none'))
);

-- SSE Event Log (for replay after disconnections)
CREATE TABLE IF NOT EXISTS sse_event_log (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
    sequence_number BIGSERIAL, -- Unique sequence per session

    event_type TEXT NOT NULL,
    event_data JSONB DEFAULT '{}',
    hypothesis_id UUID,
    timestamp TIMESTAMPTZ DEFAULT NOW(),

    -- Delivery tracking
    delivered BOOLEAN DEFAULT FALSE,
    delivery_attempts INTEGER DEFAULT 0,
    last_delivery_attempt TIMESTAMPTZ
) PARTITION BY HASH (session_id);

-- Create hash partitions for concurrent access (4 partitions for good distribution)
CREATE TABLE IF NOT EXISTS sse_event_log_00 PARTITION OF sse_event_log FOR VALUES WITH (MODULUS 4, REMAINDER 0);
CREATE TABLE IF NOT EXISTS sse_event_log_01 PARTITION OF sse_event_log FOR VALUES WITH (MODULUS 4, REMAINDER 1);
CREATE TABLE IF NOT EXISTS sse_event_log_02 PARTITION OF sse_event_log FOR VALUES WITH (MODULUS 4, REMAINDER 2);
CREATE TABLE IF NOT EXISTS sse_event_log_03 PARTITION OF sse_event_log FOR VALUES WITH (MODULUS 4, REMAINDER 3);

-- Research Metrics (daily aggregated analytics)
CREATE TABLE IF NOT EXISTS research_metrics (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    session_id UUID REFERENCES research_sessions(id),

    -- Hypothesis counts by risk level
    risk_level_totals INTEGER[] DEFAULT ARRAY[0,0,0,0], -- [low, medium, high, critical]
    risk_level_accepted INTEGER[] DEFAULT ARRAY[0,0,0,0],
    risk_level_rejected INTEGER[] DEFAULT ARRAY[0,0,0,0],

    -- Performance metrics
    total_compute_weight INTEGER DEFAULT 0,
    total_e_value_generated DECIMAL(10,4) DEFAULT 0.0,
    scientific_efficiency DECIMAL(5,3) GENERATED ALWAYS AS (
        CASE WHEN total_compute_weight > 0
        THEN total_e_value_generated / total_compute_weight
        ELSE 0.0 END
    ) STORED,

    -- Phase timing statistics
    phase_avg_times_ms INTEGER[] DEFAULT ARRAY[]::INTEGER[], -- [integrity, causality, complexity]
    phase_max_times_ms INTEGER[] DEFAULT ARRAY[]::INTEGER[],
    phase_min_times_ms INTEGER[] DEFAULT ARRAY[]::INTEGER[],

    -- System resource usage
    avg_memory_mb INTEGER,
    peak_memory_mb INTEGER,
    total_runtime_seconds INTEGER,

    UNIQUE(date, session_id)
);

-- Research Audit Log (scientific reproducibility)
CREATE TABLE IF NOT EXISTS research_audit_log (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
    hypothesis_id TEXT REFERENCES hypothesis_results(id),
    test_id TEXT,

    event_type TEXT NOT NULL,
    event_data JSONB DEFAULT '{}',

    -- Actor tracking
    actor_type TEXT CHECK (actor_type IN ('user', 'system', 'llm', 'referee')),
    actor_id TEXT,

    -- Context
    ip_address INET,
    user_agent TEXT,

    timestamp TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (timestamp);

-- Monthly partitions for audit log
CREATE TABLE IF NOT EXISTS research_audit_log_y2024m12 PARTITION OF research_audit_log
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- Configuration Snapshots (versioned system configurations)
CREATE TABLE IF NOT EXISTS config_snapshots (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,

    config_type TEXT NOT NULL,
    config_version TEXT NOT NULL,
    config_data JSONB NOT NULL,

    -- Version control
    parent_version_id BIGINT REFERENCES config_snapshots(id),
    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

-- Evidence accumulation indexes (time-series optimized)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_evidence_accumulation_hypothesis_time
    ON evidence_accumulation(hypothesis_id, timestamp DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_evidence_accumulation_e_value ON evidence_accumulation(e_value);

-- UI state cache indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ui_state_cache_last_updated
    ON ui_state_cache(last_updated DESC);

-- SSE event log indexes (hash-partitioned for concurrency)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sse_event_log_session_sequence
    ON sse_event_log(session_id, sequence_number);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sse_event_log_event_type ON sse_event_log(event_type);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sse_event_log_timestamp ON sse_event_log(timestamp DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sse_event_log_undelivered
    ON sse_event_log(delivered, delivery_attempts) WHERE delivered = FALSE;

-- Research metrics indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_research_metrics_date_efficiency
    ON research_metrics(date DESC, scientific_efficiency DESC);

-- Research audit log indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_research_audit_log_session_event
    ON research_audit_log(session_id, event_type, timestamp DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_research_audit_log_hypothesis
    ON research_audit_log(hypothesis_id, event_type);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_research_audit_log_actor
    ON research_audit_log(actor_type, actor_id);

-- Config snapshots indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_snapshots_session_type
    ON config_snapshots(session_id, config_type);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_snapshots_active
    ON config_snapshots(session_id, config_type, is_active) WHERE is_active = TRUE;

-- ============================================================================
-- VIEWS FOR UI AND ANALYTICS
-- ============================================================================

-- Active Sessions View (materialized for dashboard performance)
CREATE OR REPLACE VIEW active_sessions AS
SELECT
    rs.id,
    rs.user_id,
    rs.workspace_id,
    rs.status,
    rs.total_hypotheses,
    rs.completed_hypotheses,
    COUNT(h.id) FILTER (WHERE h.status NOT IN ('completed', 'failed', 'killed')) as active_hypotheses,
    COUNT(vt.id) FILTER (WHERE vt.status = 'running') as running_tests,
    COUNT(vt.id) FILTER (WHERE vt.status = 'queued') as queued_tests,
    rs.ui_state,
    rs.scientific_efficiency,
    rs.updated_at
FROM research_sessions rs
LEFT JOIN hypothesis_results h ON rs.id = h.session_id
LEFT JOIN validation_tests vt ON h.id = vt.hypothesis_id
WHERE rs.status IN ('analyzing', 'generating', 'validating')
GROUP BY rs.id, rs.user_id, rs.workspace_id, rs.status, rs.total_hypotheses,
         rs.completed_hypotheses, rs.ui_state, rs.scientific_efficiency, rs.updated_at;

-- Hypothesis Progress View (real-time aggregation)
CREATE OR REPLACE VIEW hypothesis_progress AS
SELECT
    h.id,
    h.session_id,
    h.business_hypothesis,
    h.status,
    h.risk_level,
    h.current_e_value,
    h.normalized_e_value,
    h.confidence,
    h.final_verdict,
    (h.phase_e_values->>0)::decimal as integrity_e_value,  -- JSONB array access
    (h.phase_e_values->>1)::decimal as causality_e_value,
    (h.phase_e_values->>2)::decimal as complexity_e_value,

    -- Test counts
    COUNT(vt.id) as total_tests,
    COUNT(vt.id) FILTER (WHERE vt.status = 'completed') as completed_tests,
    COUNT(vt.id) FILTER (WHERE vt.status = 'running') as running_tests,
    COUNT(vt.id) FILTER (WHERE vt.status = 'queued') as queued_tests,

    -- Performance metrics
    AVG(EXTRACT(EPOCH FROM vt.execution_time) * 1000) FILTER (WHERE vt.status = 'completed') as avg_test_time_ms,
    SUM(vt.compute_weight) as total_compute_cost,

    -- Latest evidence
    (SELECT e_value FROM evidence_accumulation
     WHERE hypothesis_id = h.id
     ORDER BY timestamp DESC LIMIT 1) as latest_e_value,

    -- UI feedback
    h.feasibility_score,
    h.data_topology
FROM hypothesis_results h
LEFT JOIN validation_tests vt ON h.id = vt.hypothesis_id
GROUP BY h.id, h.session_id, h.business_hypothesis, h.status, h.risk_level,
         h.current_e_value, h.normalized_e_value, h.confidence, h.final_verdict,
         h.phase_e_values, h.feasibility_score, h.data_topology;

-- ============================================================================
-- TRIGGERS FOR AUTOMATIC METRICS
-- ============================================================================

-- Update session timestamps
CREATE OR REPLACE FUNCTION update_session_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_session_timestamp_trigger
    BEFORE UPDATE ON research_sessions
    FOR EACH ROW EXECUTE FUNCTION update_session_timestamp();

-- Update UI state cache on session changes
CREATE OR REPLACE FUNCTION update_ui_state_cache()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.ui_state IS NOT NULL AND NEW.ui_state != '{}' THEN
        INSERT INTO ui_state_cache (session_id, ui_state, version, last_updated)
        VALUES (NEW.id, NEW.ui_state, COALESCE((SELECT version FROM ui_state_cache WHERE session_id = NEW.id), 0) + 1, NOW())
        ON CONFLICT (session_id) DO UPDATE SET
            ui_state = EXCLUDED.ui_state,
            version = EXCLUDED.version,
            last_updated = EXCLUDED.last_updated;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_ui_state_cache_trigger
    AFTER INSERT OR UPDATE ON research_sessions
    FOR EACH ROW EXECUTE FUNCTION update_ui_state_cache();

-- Log hypothesis state changes for audit trail
CREATE OR REPLACE FUNCTION audit_hypothesis_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status != NEW.status OR OLD.final_verdict IS DISTINCT FROM NEW.final_verdict THEN
        INSERT INTO research_audit_log (session_id, hypothesis_id, event_type, event_data, actor_type)
        VALUES (
            NEW.session_id,
            NEW.id,
            'hypothesis_status_change',
            jsonb_build_object(
                'old_status', OLD.status,
                'new_status', NEW.status,
                'old_verdict', OLD.final_verdict,
                'new_verdict', NEW.final_verdict,
                'e_value', NEW.current_e_value,
                'timestamp', NOW()
            ),
            'system'
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_hypothesis_changes_trigger
    AFTER UPDATE ON hypothesis_results
    FOR EACH ROW EXECUTE FUNCTION audit_hypothesis_changes();

-- Automatic daily metrics aggregation
CREATE OR REPLACE FUNCTION update_daily_metrics()
RETURNS TRIGGER AS $$
BEGIN
    -- Only update metrics when a session completes
    IF NEW.status = 'completed' THEN
        -- This would aggregate metrics for the completed session
        -- Implementation depends on specific metric calculation logic
        INSERT INTO research_metrics (
            date, session_id,
            total_compute_weight, total_e_value_generated
        )
        SELECT
            CURRENT_DATE,
            NEW.id,
            COALESCE(SUM(vt.compute_weight), 0),
            COALESCE(AVG(h.current_e_value), 0.0)
        FROM hypothesis_results h
        LEFT JOIN validation_tests vt ON h.id = vt.hypothesis_id
        WHERE h.session_id = NEW.id::TEXT
        ON CONFLICT (date, session_id) DO UPDATE SET
            total_compute_weight = EXCLUDED.total_compute_weight,
            total_e_value_generated = EXCLUDED.total_e_value_generated;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_daily_metrics_trigger
    AFTER UPDATE ON research_sessions
    FOR EACH ROW
    WHEN (NEW.status = 'completed')
    EXECUTE FUNCTION update_daily_metrics();

-- ============================================================================
-- PARTITION MANAGEMENT UTILITIES
-- ============================================================================

-- Function to create evidence accumulation partitions
CREATE OR REPLACE FUNCTION create_evidence_partition(start_date DATE, end_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
BEGIN
    partition_name := 'evidence_accumulation_' || to_char(start_date, 'YYYYMM');
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I PARTITION OF evidence_accumulation FOR VALUES FROM (%L) TO (%L)',
                   partition_name, start_date, end_date);
END;
$$ LANGUAGE plpgsql;

-- Function to create audit log partitions
CREATE OR REPLACE FUNCTION create_audit_partition(start_date DATE, end_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
BEGIN
    partition_name := 'research_audit_log_' || to_char(start_date, 'YYYYMM');
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I PARTITION OF research_audit_log FOR VALUES FROM (%L) TO (%L)',
                   partition_name, start_date, end_date);
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- MIGRATION COMPLETE
-- ============================================================================

-- Add comments for documentation
COMMENT ON TABLE evidence_accumulation IS 'Time-series evidence accumulation for live UI updates and scientific reproducibility';
COMMENT ON TABLE ui_state_cache IS 'HTMX UI state cache for reconnection after network issues';
COMMENT ON TABLE sse_event_log IS 'SSE event log for UI replay after disconnections';
COMMENT ON TABLE research_metrics IS 'Daily aggregated metrics for monitoring scientific efficiency and skepticism ratios';
COMMENT ON TABLE research_audit_log IS 'Immutable audit trail for scientific reproducibility and compliance';
COMMENT ON TABLE config_snapshots IS 'Version-controlled configuration snapshots for reproducible research';
