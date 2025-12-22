-- Migration 004: Dataset storage with AI-powered naming
-- Supports user-uploaded datasets with Forensic Scout AI analysis

-- Datasets table for storing uploaded dataset metadata with AI-generated names
CREATE TABLE datasets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- File information
    original_filename VARCHAR(255) NOT NULL,
    file_path TEXT,
    file_size BIGINT,
    mime_type VARCHAR(100),

    -- AI-generated naming and context (from Forensic Scout)
    display_name VARCHAR(255),        -- AI-generated descriptive name (3-5 words in snake_case)
    domain VARCHAR(100),             -- AI-detected business domain (1-2 words)
    description TEXT,                -- AI-generated summary/description

    -- Dataset metadata
    record_count INTEGER,
    field_count INTEGER,
    missing_rate DECIMAL(5,4),        -- 0.0000 to 1.0000
    source VARCHAR(50) DEFAULT 'upload', -- 'upload', 'excel', 'api'

    -- Processing status
    status VARCHAR(50) DEFAULT 'processing', -- processing, ready, failed
    error_message TEXT,

    -- Rich metadata stored as JSONB (fields, samples, AI analysis)
    metadata JSONB,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_datasets_user_id ON datasets(user_id);
CREATE INDEX idx_datasets_status ON datasets(status);
CREATE INDEX idx_datasets_source ON datasets(source);
CREATE INDEX idx_datasets_created_at ON datasets(created_at DESC);
CREATE INDEX idx_datasets_domain ON datasets(domain);

-- Insert the existing "current" dataset as a special Excel-based record
-- This maintains backward compatibility with the existing Excel workflow
INSERT INTO datasets (
    id,
    user_id,
    original_filename,
    display_name,
    domain,
    source,
    status,
    description
) VALUES (
    '550e8400-e29b-41d4-a716-446655440000', -- Special ID for current dataset
    '550e8400-e29b-41d4-a716-446655440000', -- Default user
    'current_dataset.xlsx',
    'current_dataset',
    'Data Analysis',
    'excel',
    'ready',
    'Primary dataset loaded from Excel file for analysis'
) ON CONFLICT (id) DO NOTHING;