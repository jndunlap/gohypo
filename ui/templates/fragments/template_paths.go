// Package fragments provides template path constants for organized template management
package fragments

import "strings"

// Template path constants for organized fragment access
const (
	// Audit templates
	AuditDashboardWidget    = "audit/audit_dashboard_widget.html"
	UserInteractionAudit    = "audit/user_interaction_audit.html"

	// Evidence templates
	EvidenceCard            = "evidence/evidence_card.html"
	EvidenceList            = "evidence/evidence_list.html"
	EvidenceProvenance      = "evidence/evidence_provenance.html"
	EvidenceQualityDashboard = "evidence/evidence_quality_dashboard.html"
	EvidenceTimeline        = "evidence/evidence_timeline.html"

	// Hypothesis templates
	HypothesisBase          = "hypothesis/hypothesis_base.html"
	HypothesisComponents    = "hypothesis/components.html"
	HypothesisViews         = "hypothesis/views.html"
	HypothesisAuditSummary  = "hypothesis/states/hypothesis_audit_summary.html"
	HypothesisCard          = "hypothesis/states/hypothesis_card.html"
	HypothesisCompleted     = "hypothesis/states/hypothesis_completed.html"
	HypothesisEvidence      = "hypothesis/states/hypothesis_evidence.html"
	HypothesisExpanded      = "hypothesis/states/hypothesis_expanded.html"
	HypothesisExplanation   = "hypothesis/states/hypothesis_explanation.html"
	HypothesisPending       = "hypothesis/states/hypothesis_pending.html"
	HypothesisRiskAssessed  = "hypothesis/states/hypothesis_risk_assessed.html"
	HypothesisValidating    = "hypothesis/states/hypothesis_validating.html"

	// Layout templates
	CenterPanel             = "layout/center_panel.html"
	LeftSidebar             = "layout/left_sidebar.html"
	UnifiedHeader           = "layout/unified_header.html"
	WorkspaceSidebar        = "layout/workspace_sidebar.html"

	// Status templates
	ProgressUpdate          = "status/progress_update.html"
	TestStatusUpdate        = "status/test_status_update.html"

	// Modal templates
	DataModal               = "modals/data_modal.html"
	EntityDetectionFailure  = "modals/entity_detection_failure.html"
)

// GetAllTemplatePaths returns all template paths for registration
func GetAllTemplatePaths() []string {
	return []string{
		// Audit
		AuditDashboardWidget,
		UserInteractionAudit,

		// Evidence
		EvidenceCard,
		EvidenceList,
		EvidenceProvenance,
		EvidenceQualityDashboard,
		EvidenceTimeline,

		// Hypothesis
		HypothesisBase,
		HypothesisComponents,
		HypothesisViews,
		HypothesisAuditSummary,
		HypothesisCard,
		HypothesisCompleted,
		HypothesisEvidence,
		HypothesisExpanded,
		HypothesisExplanation,
		HypothesisPending,
		HypothesisRiskAssessed,
		HypothesisValidating,

		// Layout
		CenterPanel,
		LeftSidebar,
		UnifiedHeader,
		WorkspaceSidebar,

		// Status
		ProgressUpdate,
		TestStatusUpdate,

		// Modals
		DataModal,
		EntityDetectionFailure,
	}
}

// GetTemplateCategory returns the category for a given template path
func GetTemplateCategory(templatePath string) string {
	switch {
	case strings.HasPrefix(templatePath, "audit/"):
		return "audit"
	case strings.HasPrefix(templatePath, "evidence/"):
		return "evidence"
	case strings.HasPrefix(templatePath, "hypothesis/"):
		return "hypothesis"
	case strings.HasPrefix(templatePath, "layout/"):
		return "layout"
	case strings.HasPrefix(templatePath, "status/"):
		return "status"
	case strings.HasPrefix(templatePath, "modals/"):
		return "modals"
	default:
		return "unknown"
	}
}
