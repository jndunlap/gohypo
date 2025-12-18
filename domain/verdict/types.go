package verdict

import (
	"gohypo/domain/core"
)

// VerdictStatus represents the validation status of a hypothesis
type VerdictStatus string

const (
	StatusValidated VerdictStatus = "validated"
	StatusRejected  VerdictStatus = "rejected"
	StatusMarginal  VerdictStatus = "marginal"
)

// Verdict represents a judgment on a hypothesis
type Verdict struct {
	Status     VerdictStatus
	Reason     RejectionReason
	PValue     float64
	Confidence float64
}

// TestResult contains the outcome of a specific test
type TestResult struct {
	TestName   string
	PValue     float64
	EffectSize float64
	Confidence float64
	Passed     bool
}

// RejectionReason explains why a hypothesis was rejected
type RejectionReason string

const (
	ReasonStatisticallySignificant   RejectionReason = "statistically_significant"
	ReasonStatisticallyInsignificant RejectionReason = "statistically_insignificant"
	ReasonLikelyRandom               RejectionReason = "likely_random"
	ReasonMarginallySignificant      RejectionReason = "marginally_significant"
	ReasonNoData                     RejectionReason = "no_data"
	ReasonInvalidData                RejectionReason = "invalid_data"
)

// FalsificationLog provides detailed audit trail for rejected hypotheses
type FalsificationLog struct {
	Reason             RejectionReason
	PermutationPValue  float64
	ObservedEffectSize float64
	NullDistribution   NullDistributionSummary
	SampleSize         int
	TestUsed           string
	VariableX          core.VariableKey
	VariableY          core.VariableKey
	RejectedAt         core.Timestamp
}

// NullDistributionSummary provides key statistics about the null distribution
type NullDistributionSummary struct {
	Mean         float64
	StdDev       float64
	Min          float64
	Max          float64
	Percentile95 float64
	Percentile99 float64
}
