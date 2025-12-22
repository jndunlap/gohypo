package dataset

import (
	"context"
	"testing"

	"gohypo/ai"
	"gohypo/domain/core"
	domainDataset "gohypo/domain/dataset"
	"gohypo/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing
type MockDatasetRepository struct {
	mock.Mock
	datasets []*domainDataset.Dataset
}

func (m *MockDatasetRepository) Create(ctx context.Context, ds *domainDataset.Dataset) error {
	args := m.Called(ctx, ds)
	m.datasets = append(m.datasets, ds)
	return args.Error(0)
}

func (m *MockDatasetRepository) GetByID(ctx context.Context, id core.ID) (*domainDataset.Dataset, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domainDataset.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) GetByUserID(ctx context.Context, userID core.ID, limit, offset int) ([]*domainDataset.Dataset, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]*domainDataset.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) GetByWorkspace(ctx context.Context, workspaceID core.ID, limit, offset int) ([]*domainDataset.Dataset, error) {
	args := m.Called(ctx, workspaceID, limit, offset)
	return m.datasets, args.Error(1)
}

func (m *MockDatasetRepository) Update(ctx context.Context, ds *domainDataset.Dataset) error {
	args := m.Called(ctx, ds)
	return args.Error(0)
}

func (m *MockDatasetRepository) Delete(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDatasetRepository) GetCurrent(ctx context.Context) (*domainDataset.Dataset, error) {
	args := m.Called(ctx)
	return args.Get(0).(*domainDataset.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) ListByStatus(ctx context.Context, status domainDataset.DatasetStatus) ([]*domainDataset.Dataset, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*domainDataset.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) ListByDomain(ctx context.Context, domain string) ([]*domainDataset.Dataset, error) {
	args := m.Called(ctx, domain)
	return args.Get(0).([]*domainDataset.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) UpdateStatus(ctx context.Context, id core.ID, status domainDataset.DatasetStatus, errorMsg string) error {
	args := m.Called(ctx, id, status, errorMsg)
	return args.Error(0)
}

type MockWorkspaceRepository struct {
	mock.Mock
	relations []*domainDataset.DatasetRelation
}

func (m *MockWorkspaceRepository) Create(ctx context.Context, workspace *domainDataset.Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) GetByID(ctx context.Context, id core.ID) (*domainDataset.Workspace, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domainDataset.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) GetByUserID(ctx context.Context, userID core.ID) ([]*domainDataset.Workspace, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domainDataset.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) Update(ctx context.Context, workspace *domainDataset.Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Delete(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) GetDefaultForUser(ctx context.Context, userID core.ID) (*domainDataset.Workspace, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*domainDataset.Workspace), args.Error(1)
}

func (m *MockWorkspaceRepository) GetWithDatasets(ctx context.Context, id core.ID) (*ports.WorkspaceWithDatasets, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*ports.WorkspaceWithDatasets), args.Error(1)
}

func (m *MockWorkspaceRepository) CreateRelation(ctx context.Context, relation *domainDataset.DatasetRelation) error {
	args := m.Called(ctx, relation)
	m.relations = append(m.relations, relation)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) GetRelations(ctx context.Context, workspaceID core.ID) ([]*domainDataset.DatasetRelation, error) {
	args := m.Called(ctx, workspaceID)
	return m.relations, args.Error(1)
}

func (m *MockWorkspaceRepository) DeleteRelation(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Simple mock that implements the needed interface
type MockForensicScout struct {
	responses map[string]*ai.ScoutResponse
}

func NewMockForensicScout() *MockForensicScout {
	return &MockForensicScout{
		responses: make(map[string]*ai.ScoutResponse),
	}
}

func (m *MockForensicScout) AnalyzeFields(ctx context.Context, fields []string) (*ai.ScoutResponse, error) {
	// Simple mock implementation - return a generic response
	return &ai.ScoutResponse{
		Domain:      "Test Domain",
		DatasetName: "Test Dataset",
	}, nil
}

func TestRelationshipDiscoveryEngine_DiscoverRelationships(t *testing.T) {
	// Setup mocks
	mockDatasetRepo := &MockDatasetRepository{}
	mockWorkspaceRepo := &MockWorkspaceRepository{}

	workspaceID := core.ID("workspace-1")

	// Create test datasets with common fields to trigger schema matching
	datasets := []*domainDataset.Dataset{
		{
			ID:          core.ID("dataset-1"),
			WorkspaceID: workspaceID,
			DisplayName: "customer_data",
			Domain:      "Retail",
			RecordCount: 1000,
			FieldCount:  3,
			Metadata: domainDataset.DatasetMetadata{
				Fields: []domainDataset.FieldInfo{
					{Name: "customer_id", DataType: "string"},
					{Name: "name", DataType: "string"},
					{Name: "email", DataType: "string"},
				},
			},
		},
		{
			ID:          core.ID("dataset-2"),
			WorkspaceID: workspaceID,
			DisplayName: "purchase_history",
			Domain:      "Retail",
			RecordCount: 5000,
			FieldCount:  4,
			Metadata: domainDataset.DatasetMetadata{
				Fields: []domainDataset.FieldInfo{
					{Name: "customer_id", DataType: "string"}, // Common field
					{Name: "product_name", DataType: "string"},
					{Name: "purchase_date", DataType: "date"},
					{Name: "amount", DataType: "numeric"},
				},
			},
		},
	}

	mockDatasetRepo.datasets = datasets

	// Setup repository expectations
	mockDatasetRepo.On("GetByWorkspace", mock.Anything, workspaceID, 1000, 0).Return(datasets, nil)
	mockWorkspaceRepo.On("CreateRelation", mock.Anything, mock.AnythingOfType("*dataset.DatasetRelation")).Return(nil).Maybe()

	// Create engine with nil forensic scout (will skip semantic analysis)
	engine := NewRelationshipDiscoveryEngine(nil, mockDatasetRepo, mockWorkspaceRepo, nil, nil)

	// Execute discovery
	result, err := engine.DiscoverRelationships(context.Background(), workspaceID)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, workspaceID, result.WorkspaceID)

	// Should find schema compatibility relationship due to common "customer_id" field
	assert.Greater(t, len(result.Relationships), 0, "Should find at least one relationship")
	assert.Greater(t, result.ConfidenceScore, 0.0, "Should have confidence score")

	// Check that relationships were stored
	assert.Greater(t, len(mockWorkspaceRepo.relations), 0, "Should have stored relationships")
}

func TestCalculateStringSimilarity(t *testing.T) {
	engine := &RelationshipDiscoveryEngine{}

	tests := []struct {
		s1       string
		s2       string
		expected float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "hello", 0.5},                        // ["hello", "world"] vs ["hello"] = intersection:1, union:2 = 0.5
		{"customer data", "customer information", 1.0 / 3.0}, // ["customer", "data"] vs ["customer", "information"] = intersection:1, union:3 = 0.333...
		{"", "", 1.0},
		{"abc", "def", 0.0},
	}

	for _, test := range tests {
		result := engine.calculateStringSimilarity(test.s1, test.s2)
		assert.Equal(t, test.expected, result, "Similarity calculation failed for %s vs %s", test.s1, test.s2)
	}
}

func TestCalculateExpectedDimensions(t *testing.T) {
	engine := &RelationshipDiscoveryEngine{}

	datasets := []*domainDataset.Dataset{
		{RecordCount: 100, FieldCount: 5},
		{RecordCount: 200, FieldCount: 7},
		{RecordCount: 150, FieldCount: 3},
	}

	tests := []struct {
		mergeType    MergeType
		expectedRows int
		expectedCols int
	}{
		{AutoUnion, 450, 7},
		{AutoAppend, 450, 7},
		{AutoJoin, 315, 7},        // 450 * 0.7
		{AutoConsolidate, 360, 7}, // 450 * 0.8
	}

	for _, test := range tests {
		rows, cols := engine.calculateExpectedDimensions(datasets, test.mergeType)
		assert.Equal(t, test.expectedRows, rows, "Row calculation failed for %s", test.mergeType)
		assert.Equal(t, test.expectedCols, cols, "Column calculation failed for %s", test.mergeType)
	}
}
