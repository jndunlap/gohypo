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
			"Valid three referees",
			RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Chow_Stability_Test", "Transfer_Entropy"},
				Rationale:        "Comprehensive validation",
			},
			false,
		},
		{
			"Invalid - only two referees",
			RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Chow_Stability_Test"},
				Rationale:        "Missing third referee",
			},
			true,
		},
		{
			"Invalid - duplicate referee",
			RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Permutation_Shredder", "Chow_Stability_Test"},
				Rationale:        "Duplicate referee",
			},
			true,
		},
		{
			"Invalid - unknown referee",
			RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Chow_Stability_Test", "Unknown_Referee"},
				Rationale:        "Unknown referee",
			},
			true,
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
