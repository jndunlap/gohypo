-- GoHypo Migration 004: Add explanation_markdown column to hypothesis_results table
-- Adds column to store markdown explanations of why hypotheses were selected

-- Add explanation_markdown column to hypothesis_results table
ALTER TABLE hypothesis_results
ADD COLUMN IF NOT EXISTS explanation_markdown TEXT;

-- Add index for potential text search on explanations
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hypotheses_explanation_md ON hypothesis_results USING gin (to_tsvector('english', explanation_markdown));

-- Add comment for documentation
COMMENT ON COLUMN hypothesis_results.explanation_markdown IS 'Markdown-formatted explanation of why this hypothesis was selected by the research process';

-- Update existing records with empty explanation (they were created before this feature)
UPDATE hypothesis_results
SET explanation_markdown = ''
WHERE explanation_markdown IS NULL;
