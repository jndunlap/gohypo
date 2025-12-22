-- GoHypo Migration 003: Add missing columns to hypothesis_results table
-- Adds columns that were referenced in code but missing from schema

-- Add missing columns to hypothesis_results table
ALTER TABLE hypothesis_results
ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES research_sessions(id),
ADD COLUMN IF NOT EXISTS current_e_value DECIMAL(10,4) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS normalized_e_value DECIMAL(3,2) CHECK (normalized_e_value >= 0 AND normalized_e_value <= 1.0),
ADD COLUMN IF NOT EXISTS confidence DECIMAL(3,2) CHECK (confidence >= 0 AND confidence <= 1.0),
ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'pending';

-- Update existing records with default values if needed
UPDATE hypothesis_results
SET
    current_e_value = 0.0,
    normalized_e_value = 0.0,
    confidence = 0.0,
    status = 'pending'
WHERE current_e_value IS NULL OR normalized_e_value IS NULL OR confidence IS NULL OR status IS NULL;

-- Add indexes for the new columns
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_workspace_id ON hypothesis_results(workspace_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_current_e_value ON hypothesis_results(current_e_value);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_normalized_e_value ON hypothesis_results(normalized_e_value);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_confidence ON hypothesis_results(confidence);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_status ON hypothesis_results(status);

-- Add comments for documentation
COMMENT ON COLUMN hypothesis_results.workspace_id IS 'Workspace association for multi-workspace support';
COMMENT ON COLUMN hypothesis_results.current_e_value IS 'Current evidence value from latest validation';
COMMENT ON COLUMN hypothesis_results.normalized_e_value IS 'Normalized e-value on 0-1 scale for UI display';
COMMENT ON COLUMN hypothesis_results.confidence IS 'Statistical confidence level (0.0 to 1.0)';
COMMENT ON COLUMN hypothesis_results.status IS 'Hypothesis validation status (pending, running, completed, failed)';
