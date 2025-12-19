-- PostgreSQL schema for gohypo research system
-- Supports user-scoped sessions and hypotheses

-- Users table (multi-user ready, single user initially)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Research sessions (user-scoped)
CREATE TABLE IF NOT EXISTS research_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50) NOT NULL DEFAULT 'idle',
    progress DECIMAL(5,2) DEFAULT 0.0,
    current_hypothesis TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Research prompts (LLM context and instructions)
CREATE TABLE IF NOT EXISTS research_prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prompt_content TEXT NOT NULL,
    prompt_type VARCHAR(50) DEFAULT 'research_directive',
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Hypothesis results (structured storage for queryability)
CREATE TABLE IF NOT EXISTS hypothesis_results (
    id VARCHAR(50) PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_hypothesis TEXT NOT NULL,
    science_hypothesis TEXT NOT NULL,
    null_case TEXT,
    referee_results JSONB,
    tri_gate_result JSONB,
    passed BOOLEAN NOT NULL,
    validation_timestamp TIMESTAMP WITH TIME ZONE,
    standards_version VARCHAR(20),
    execution_metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON research_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_state ON research_sessions(user_id, state);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON research_sessions(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_prompts_user_id ON research_prompts(user_id);
CREATE INDEX IF NOT EXISTS idx_prompts_session_id ON research_prompts(session_id);
CREATE INDEX IF NOT EXISTS idx_prompts_user_session ON research_prompts(user_id, session_id);
CREATE INDEX IF NOT EXISTS idx_prompts_type ON research_prompts(prompt_type);
CREATE INDEX IF NOT EXISTS idx_prompts_created_at ON research_prompts(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_hypotheses_user_id ON hypothesis_results(user_id);
CREATE INDEX IF NOT EXISTS idx_hypotheses_session_id ON hypothesis_results(session_id);
CREATE INDEX IF NOT EXISTS idx_hypotheses_user_session ON hypothesis_results(user_id, session_id);
CREATE INDEX IF NOT EXISTS idx_hypotheses_user_created ON hypothesis_results(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_hypotheses_passed ON hypothesis_results(passed);
CREATE INDEX IF NOT EXISTS idx_hypotheses_created_at ON hypothesis_results(created_at DESC);

-- Insert default user for single-user mode
INSERT INTO users (id, email, username, is_active)
VALUES ('550e8400-e29b-41d4-a716-446655440000', 'default@grohypo.local', 'default', true)
ON CONFLICT (email) DO NOTHING;