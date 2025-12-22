-- Migration 006: LLM Usage Tracking
-- Adds token usage tracking per user, model, and provider for FinOps analysis

-- Raw usage events table
CREATE TABLE llm_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID REFERENCES research_sessions(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,        -- 'openai', 'anthropic', 'google', etc.
    model VARCHAR(100) NOT NULL,          -- 'gpt-5.2', 'gpt-5.2', 'claude-3', etc.
    operation_type VARCHAR(50),           -- 'hypothesis_generation', 'dataset_analysis', etc.
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Constraints for data integrity
ALTER TABLE llm_usage
ADD CONSTRAINT check_tokens_positive
CHECK (prompt_tokens >= 0 AND completion_tokens >= 0 AND total_tokens >= 0);

ALTER TABLE llm_usage
ADD CONSTRAINT check_total_tokens
CHECK (total_tokens = prompt_tokens + completion_tokens);

-- Indexes for performance (critical for FinOps analytics)
CREATE INDEX idx_llm_usage_user_id ON llm_usage(user_id);
CREATE INDEX idx_llm_usage_session_id ON llm_usage(session_id);
CREATE INDEX idx_llm_usage_provider ON llm_usage(provider);
CREATE INDEX idx_llm_usage_model ON llm_usage(model);
CREATE INDEX idx_llm_usage_operation_type ON llm_usage(operation_type);
CREATE INDEX idx_llm_usage_created_at ON llm_usage(created_at DESC);

-- Compound indexes for common query patterns
CREATE INDEX idx_llm_usage_user_created ON llm_usage(user_id, created_at DESC);
CREATE INDEX idx_llm_usage_user_provider ON llm_usage(user_id, provider);
CREATE INDEX idx_llm_usage_user_model ON llm_usage(user_id, model);
CREATE INDEX idx_llm_usage_provider_model ON llm_usage(provider, model);

-- Index for time-range queries (essential for usage reporting)
CREATE INDEX idx_llm_usage_user_time_range ON llm_usage(user_id, created_at);

-- Partial indexes for active data (performance optimization)
CREATE INDEX idx_llm_usage_recent ON llm_usage(created_at DESC) WHERE created_at > NOW() - INTERVAL '30 days';

-- Comments for documentation
COMMENT ON TABLE llm_usage IS 'Tracks token usage for all LLM API calls, enabling FinOps cost analysis and optimization';
COMMENT ON COLUMN llm_usage.operation_type IS 'Categorizes usage by feature area (hypothesis_generation, dataset_analysis, etc.)';
COMMENT ON COLUMN llm_usage.provider IS 'LLM provider (openai, anthropic, google, etc.)';
COMMENT ON COLUMN llm_usage.model IS 'Specific model used (gpt-5.2, gpt-5.2, claude-3, etc.)';