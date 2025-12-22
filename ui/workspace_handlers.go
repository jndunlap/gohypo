package ui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	processor "gohypo/internal/dataset"

	"github.com/gin-gonic/gin"
)

// handleGetWorkspaces returns all workspaces for the current user
func (s *Server) handleGetWorkspaces(c *gin.Context) {
	if s.workspaceRepository == nil {
		log.Printf("[handleGetWorkspaces] ERROR: Workspace repository is nil")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		log.Printf("[handleGetWorkspaces] ERROR: Failed to get default user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Ensure default workspace exists
	_, err = s.ensureDefaultWorkspace(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetWorkspaces] WARNING: Failed to ensure default workspace: %v", err)
		// Continue anyway - don't fail the request
	}

	workspaces, err := s.workspaceRepository.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetWorkspaces] ERROR: Failed to retrieve workspaces from repository: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workspaces"})
		return
	}

	// Ensure workspaces is never nil (return empty slice instead)
	if workspaces == nil {
		log.Printf("[handleGetWorkspaces] WARNING: Repository returned nil workspaces, converting to empty slice")
		workspaces = []*dataset.Workspace{}
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

// handleCreateWorkspace creates a new workspace
func (s *Server) handleCreateWorkspace(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	workspace := dataset.NewWorkspace(userID, req.Name)
	if req.Description != "" {
		workspace.Description = req.Description
	}
	if req.Color != "" {
		workspace.Color = req.Color
	}

	if err := s.workspaceRepository.Create(c.Request.Context(), workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}

	c.JSON(http.StatusCreated, workspace)
}

// handleGetWorkspace returns a specific workspace with its datasets
func (s *Server) handleGetWorkspace(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user for ownership validation
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	workspace, err := s.workspaceRepository.GetWithDatasets(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	// Verify ownership
	if workspace.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

// handleUpdateWorkspace updates a workspace
func (s *Server) handleUpdateWorkspace(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Get existing workspace
	workspace, err := s.workspaceRepository.GetByID(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	// Verify ownership
	if workspace.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Update fields
	if req.Name != "" {
		workspace.Name = req.Name
	}
	if req.Description != "" {
		workspace.Description = req.Description
	}
	if req.Color != "" {
		workspace.Color = req.Color
	}

	if err := s.workspaceRepository.Update(c.Request.Context(), workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workspace"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

// handleDeleteWorkspace deletes a workspace
func (s *Server) handleDeleteWorkspace(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership before deletion
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	if err := s.workspaceRepository.Delete(c.Request.Context(), workspaceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workspace"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace deleted successfully"})
}

// handleGetWorkspaceDatasets returns datasets for a specific workspace
func (s *Server) handleGetWorkspaceDatasets(c *gin.Context) {
	if s.datasetRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	// Parse pagination
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	datasets, err := s.datasetRepository.GetByWorkspace(c.Request.Context(), workspaceID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve datasets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"datasets": datasets,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
		},
	})
}

// handleGetWorkspaceRelations returns dataset relationships for a workspace
func (s *Server) handleGetWorkspaceRelations(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	relations, err := s.workspaceRepository.GetRelations(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve relations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"relations": relations})
}

// handleDiscoverRelationships triggers relationship discovery for a workspace
func (s *Server) handleDiscoverRelationships(c *gin.Context) {
	relationshipEngine := s.datasetProcessor.GetRelationshipEngine()
	if s.datasetProcessor == nil || relationshipEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relationship discovery not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	// Run relationship discovery
	result, err := relationshipEngine.DiscoverRelationships(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to discover relationships"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleGetWorkspaceRelationships retrieves stored relationships for visualization with enhanced LLM data
func (s *Server) handleGetWorkspaceRelationships(c *gin.Context) {
	if s.workspaceRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Workspace service not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	// Get stored relationships
	relations, err := s.workspaceRepository.GetRelations(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve relationships"})
		return
	}

	// Get datasets in workspace for context
	workspaceWithDatasets, err := s.workspaceRepository.GetWithDatasets(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workspace datasets"})
		return
	}

	// Enhance datasets with LLM-generated names and business context
	enhancedDatasets := make([]gin.H, len(workspaceWithDatasets.Datasets))
	domains := make(map[string]bool)

	for i, dataset := range workspaceWithDatasets.Datasets {
		// Extract domain for statistics
		domain := dataset.Domain
		if domain == "" {
			domain = "General"
		}
		domains[domain] = true

		// Create enhanced dataset object with LLM context
		enhancedDatasets[i] = gin.H{
			"id":           dataset.ID,
			"display_name": dataset.DisplayName,
			"llm_name":     dataset.DisplayName, // Could be enhanced with actual LLM names
			"domain":       domain,
			"description":  dataset.Description,
			"record_count": dataset.RecordCount,
			"field_count":  dataset.FieldCount,
			"metadata":     dataset.Metadata,
		}
	}

	// Enhance relationships with business context
	enhancedRelations := make([]gin.H, len(relations))
	for i, relation := range relations {
		// Extract business context from metadata if available
		businessContext := ""
		if relation.Metadata != nil {
			if ctx, ok := relation.Metadata["business_context"].(string); ok {
				businessContext = ctx
			}
		}

		enhancedRelations[i] = gin.H{
			"id":                relation.ID,
			"workspace_id":      relation.WorkspaceID,
			"source_dataset_id": relation.SourceDatasetID,
			"target_dataset_id": relation.TargetDatasetID,
			"relation_type":     relation.RelationType,
			"confidence":        relation.Confidence,
			"metadata":          relation.Metadata,
			"business_context":  businessContext,
			"discovered_at":     relation.DiscoveredAt,
		}
	}

	response := gin.H{
		"workspace_id":       workspaceID,
		"relationships":      enhancedRelations,
		"datasets":           enhancedDatasets,
		"relationship_count": len(relations),
		"dataset_count":      len(workspaceWithDatasets.Datasets),
		"domain_count":       len(domains),
		"domains":            getDomainKeys(domains),
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to get domain keys
func getDomainKeys(domains map[string]bool) []string {
	keys := make([]string, 0, len(domains))
	for domain := range domains {
		keys = append(keys, domain)
	}
	return keys
}

// clearWorkspaceRelationships removes all relationships for a workspace
func (s *Server) clearWorkspaceRelationships(ctx context.Context, workspaceID core.ID) error {
	relations, err := s.workspaceRepository.GetRelations(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get existing relations: %w", err)
	}

	// Delete each relationship
	for _, relation := range relations {
		err := s.workspaceRepository.DeleteRelation(ctx, relation.ID)
		if err != nil {
			log.Printf("[clearWorkspaceRelationships] Failed to delete relation %s: %v", relation.ID, err)
			// Continue with other deletions
		}
	}

	return nil
}

// handleAutoMergeSuggestions executes automatic merges based on high-confidence suggestions
func (s *Server) handleAutoMergeSuggestions(c *gin.Context) {
	relationshipEngine := s.datasetProcessor.GetRelationshipEngine()
	if s.datasetProcessor == nil || relationshipEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Auto-merge not available"})
		return
	}

	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify ownership
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	// Get relationship discovery results
	discoveryResult, err := relationshipEngine.DiscoverRelationships(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze relationships"})
		return
	}

	var executedMerges []map[string]interface{}

	// Execute high-confidence merges
	for _, suggestion := range discoveryResult.MergeSuggestions {
		if suggestion.Confidence > 0.85 { // High confidence threshold
			mergeResult, err := s.datasetProcessor.Merger.MergeDatasets(c.Request.Context(), suggestion.SourceDatasets,
				fmt.Sprintf("auto_merged_%d", time.Now().Unix()), &processor.MergeConfig{
					Strategy:       processor.InMemoryMerge,
					JoinType:       processor.UnionJoin,
					ValidateSchema: true,
				})

			if err != nil {
				log.Printf("[handleAutoMergeSuggestions] Merge failed: %v", err)
				continue
			}

			executedMerges = append(executedMerges, map[string]interface{}{
				"source_datasets": suggestion.SourceDatasets,
				"merge_type":      suggestion.MergeType,
				"confidence":      suggestion.Confidence,
				"result_dataset":  mergeResult.OutputPath,
				"row_count":       mergeResult.RowCount,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Auto-merge completed",
		"executed_merges":   executedMerges,
		"total_suggestions": len(discoveryResult.MergeSuggestions),
		"executed_count":    len(executedMerges),
	})
}

// handleGetWorkspaceHypotheses returns hypotheses for a specific workspace
func (s *Server) handleGetWorkspaceHypotheses(c *gin.Context) {
	workspaceIDStr := c.Param("id")
	if workspaceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace ID is required"})
		return
	}

	workspaceID := core.ID(workspaceIDStr)

	// Get default user
	userID, err := s.getDefaultUserID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify workspace exists and belongs to user
	if err := s.validateWorkspaceOwnership(c.Request.Context(), workspaceID, userID); err != nil {
		if err.Error() == "workspace not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		}
		return
	}

	// Get hypotheses for this workspace
	hypotheses, err := s.researchStorage.ListByWorkspace(c.Request.Context(), string(workspaceID), 50)
	if err != nil {
		log.Printf("[API] Failed to list hypotheses for workspace %s: %v", workspaceID, err)

		if c.GetHeader("HX-Request") == "true" {
			html := s.renderService.RenderHypothesisError("Failed to retrieve hypotheses for this workspace")
			c.Header("Content-Type", "text/html")
			c.String(http.StatusInternalServerError, html)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve hypotheses"})
		return
	}

	workspaceHypotheses := hypotheses

	// Check if HTMX request (for dynamic updates)
	if c.GetHeader("HX-Request") == "true" {
		html := s.renderService.RenderHypothesisCards(workspaceHypotheses)
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hypotheses": workspaceHypotheses,
		"count":      len(workspaceHypotheses),
	})
}
