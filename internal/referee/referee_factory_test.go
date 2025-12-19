package referee

import (
	"testing"
)

func TestGetRefereeFactory(t *testing.T) {
	tests := []struct {
		name         string
		refereeName  string
		expectError  bool
		expectedType string
	}{
		{"Permutation_Shredder", "permutation_shuffling", false, "*referee.Shredder"},
		{"Chow_Stability_Test", "chow_stability_test", false, "*referee.ChowTest"},
		{"Transfer_Entropy", "transfer_entropy", false, "*referee.TransferEntropy"},
		{"Convergent_Cross_Mapping", "convergent_cross_mapping", false, "*referee.ConvergentCrossMapping"},
		{"Isotonic_Mechanism_Check", "isotonic_mechanism_check", false, "*referee.MonotonicityTest"},
		{"Invalid referee", "invalid_referee", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			referee, err := GetRefereeFactory(tt.refereeName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for invalid referee %s, got nil", tt.refereeName)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for referee %s: %v", tt.refereeName, err)
				return
			}

			if referee == nil {
				t.Errorf("Expected non-nil referee for %s", tt.refereeName)
			}
		})
	}
}

func TestGetRefereeConfigs(t *testing.T) {
	configs := GetRefereeConfigs()

	if len(configs) == 0 {
		t.Error("Expected non-empty referee configs")
		return
	}

	// Check that all expected referees are present
	expectedNames := []string{
		"Permutation_Shredder",
		"Chow_Stability_Test",
		"Transfer_Entropy",
		"Convergent_Cross_Mapping",
		"Isotonic_Mechanism_Check",
	}

	found := make(map[string]bool)
	for _, config := range configs {
		found[config.Name] = true
	}

	for _, expected := range expectedNames {
		if !found[expected] {
			t.Errorf("Expected referee %s not found in configs", expected)
		}
	}
}

func TestValidateRefereeCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		referees    []string
		expectError bool
	}{
		{
			"Valid three referees with required categories",
			[]string{"permutation_shuffling", "chow_stability_test", "transfer_entropy"},
			false,
		},
		{
			"Invalid - only two referees",
			[]string{"permutation_shuffling", "chow_stability_test"},
			true,
		},
		{
			"Invalid - missing SHREDDER category",
			[]string{"transfer_entropy", "chow_stability_test", "isotonic_mechanism_check"},
			true,
		},
		{
			"Invalid - duplicate referee",
			[]string{"permutation_shuffling", "permutation_shuffling", "chow_stability_test"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRefereeCompatibility(tt.referees)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for referees %v, got nil", tt.referees)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for referees %v: %v", tt.referees, err)
			}
		})
	}
}
