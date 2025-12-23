package models

import (
	"testing"
)

func TestRefereeGates_Validate(t *testing.T) {
	tests := []struct {
		name         string
		refereeGates RefereeGates
		expectError  bool
	}{
		{
			name: "Valid three referees",
			refereeGates: RefereeGates{
				SelectedReferees: []RefereeSelection{
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 1},
					{Name: "Chow_Stability_Test", Category: "VALIDATION", Priority: 2},
					{Name: "Transfer_Entropy", Category: "CAUSALITY", Priority: 3},
				},
				Rationale: "Comprehensive validation",
			},
			expectError: false,
		},
		{
			name: "Valid - two referees",
			refereeGates: RefereeGates{
				SelectedReferees: []RefereeSelection{
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 1},
					{Name: "Chow_Stability_Test", Category: "VALIDATION", Priority: 2},
				},
				Rationale: "Two referees is valid",
			},
			expectError: false,
		},
		{
			name: "Invalid - duplicate referee",
			refereeGates: RefereeGates{
				SelectedReferees: []RefereeSelection{
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 1},
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 2},
					{Name: "Chow_Stability_Test", Category: "VALIDATION", Priority: 3},
				},
				Rationale: "Duplicate referee",
			},
			expectError: true,
		},
		{
			name: "Invalid - unknown referee",
			refereeGates: RefereeGates{
				SelectedReferees: []RefereeSelection{
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 1},
					{Name: "Chow_Stability_Test", Category: "VALIDATION", Priority: 2},
					{Name: "Unknown_Referee", Category: "UNKNOWN", Priority: 3},
				},
				Rationale: "Unknown referee",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.refereeGates.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}
