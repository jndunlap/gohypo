-- Migration 005: Workspaces for dataset organization
-- Enables users to organize datasets into workspaces with relationships

-- Workspaces table
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#3B82F6', -- Hex color for UI theming
    is_default BOOLEAN DEFAULT false, -- One default workspace per user
    metadata JSONB DEFAULT '{}', -- Workspace settings, tags, etc.

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_user_default_workspace UNIQUE (user_id, is_default) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT check_color_format CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);

-- Add workspace_id to datasets table
ALTER TABLE datasets ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;

-- Create default workspace for existing user
INSERT INTO workspaces (id, user_id, name, description, is_default, metadata)
VALUES (
    '550e8400-e29b-41d4-a716-446655440001', -- Special ID for default workspace
    '550e8400-e29b-41d4-a716-446655440000', -- Default user
    'Default Workspace',
    'Your primary workspace for data analysis and research',
    true,
    '{"auto_discover_relations": true, "max_datasets": 50}'
);

-- Assign existing datasets to default workspace
UPDATE datasets SET workspace_id = '550e8400-e29b-41d4-a716-446655440001' WHERE workspace_id IS NULL;

-- Make workspace_id NOT NULL after migration
ALTER TABLE datasets ALTER COLUMN workspace_id SET NOT NULL;

-- Dataset relationships within workspaces
CREATE TABLE workspace_dataset_relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    target_dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    relation_type VARCHAR(50) NOT NULL, -- 'entity_link', 'field_mapping', 'data_flow', 'reference'
    confidence DECIMAL(3,2), -- 0.00 to 1.00 confidence score
    metadata JSONB DEFAULT '{}', -- Relationship details, mapping rules, etc.
    discovered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Prevent self-references and duplicate relations
    CONSTRAINT no_self_reference CHECK (source_dataset_id != target_dataset_id),
    CONSTRAINT unique_relation UNIQUE (workspace_id, source_dataset_id, target_dataset_id, relation_type),

    -- Ensure datasets belong to the same workspace
    CONSTRAINT datasets_in_same_workspace CHECK (
        EXISTS (
            SELECT 1 FROM datasets d1, datasets d2
            WHERE d1.id = source_dataset_id AND d2.id = target_dataset_id
            AND d1.workspace_id = workspace_id AND d2.workspace_id = workspace_id
        )
    )
);

-- Indexes for performance
CREATE INDEX idx_workspaces_user_id ON workspaces(user_id);
CREATE INDEX idx_workspaces_user_default ON workspaces(user_id, is_default);
CREATE INDEX idx_datasets_workspace_id ON datasets(workspace_id);
CREATE INDEX idx_workspace_relations_workspace ON workspace_dataset_relations(workspace_id);
CREATE INDEX idx_workspace_relations_datasets ON workspace_dataset_relations(source_dataset_id, target_dataset_id);
CREATE INDEX idx_workspace_relations_type ON workspace_dataset_relations(relation_type);

-- Function to automatically discover dataset relationships
CREATE OR REPLACE FUNCTION discover_dataset_relations(workspace_uuid UUID)
RETURNS INTEGER AS $$
DECLARE
    relation_count INTEGER := 0;
    source_record RECORD;
    target_record RECORD;
BEGIN
    -- Find datasets with similar field names (potential entity links)
    FOR source_record IN
        SELECT d1.id as source_id, d1.metadata->'fields' as source_fields
        FROM datasets d1
        WHERE d1.workspace_id = workspace_uuid
    LOOP
        FOR target_record IN
            SELECT d2.id as target_id, d2.metadata->'fields' as target_fields
            FROM datasets d2
            WHERE d2.workspace_id = workspace_uuid
            AND d2.id != source_record.source_id
        LOOP
            -- Check for common field names (simplified relationship discovery)
            IF source_record.source_fields IS NOT NULL AND target_record.target_fields IS NOT NULL THEN
                -- This is a simplified version - in production you'd implement
                -- more sophisticated relationship discovery algorithms
                INSERT INTO workspace_dataset_relations (
                    workspace_id, source_dataset_id, target_dataset_id,
                    relation_type, confidence, metadata
                )
                SELECT workspace_uuid, source_record.source_id, target_record.target_id,
                       'potential_link', 0.5, '{"auto_discovered": true}'
                WHERE NOT EXISTS (
                    SELECT 1 FROM workspace_dataset_relations
                    WHERE workspace_id = workspace_uuid
                    AND source_dataset_id = source_record.source_id
                    AND target_dataset_id = target_record.target_id
                    AND relation_type = 'potential_link'
                );
                GET DIAGNOSTICS relation_count = ROW_COUNT;
            END IF;
        END LOOP;
    END LOOP;

    RETURN relation_count;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-discover relations when datasets are added
CREATE OR REPLACE FUNCTION trigger_auto_discover_relations()
RETURNS TRIGGER AS $$
BEGIN
    -- Auto-discover relations for the workspace (async in production)
    PERFORM discover_dataset_relations(NEW.workspace_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to datasets table
CREATE TRIGGER auto_discover_relations_trigger
    AFTER INSERT ON datasets
    FOR EACH ROW
    EXECUTE FUNCTION trigger_auto_discover_relations();