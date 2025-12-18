package testkit

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestShoppingDataGenerator_Basic(t *testing.T) {
	config := ShoppingGeneratorConfig{
		CustomerCount:        10, // Small for testing
		ProductCount:         50,
		AvgOrdersPerCustomer: 1.5,
		ReturnRateBase:       0.1,
		RefundRateBase:       0.05,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
		Seed:                 42,
		SchemaVersion:        "1.0.0",
	}

	generator := NewShoppingDataGenerator(config)
	events, err := generator.GenerateEvents()
	if err != nil {
		t.Fatalf("Failed to generate events: %v", err)
	}

	if len(events) == 0 {
		t.Error("Expected events to be generated")
	}

	// Verify basic structure
	for i, event := range events {
		if event.EntityID.IsEmpty() {
			t.Errorf("Event %d has empty entity ID", i)
		}
		if event.FieldKey == "" {
			t.Errorf("Event %d has empty field key", i)
		}
		if event.Source == "" {
			t.Errorf("Event %d has empty source", i)
		}
	}
}

func TestShoppingDataGenerator_WriteToFile(t *testing.T) {
	config := ShoppingGeneratorConfig{
		CustomerCount:        5, // Very small for testing
		ProductCount:         20,
		AvgOrdersPerCustomer: 1.0,
		ReturnRateBase:       0.1,
		RefundRateBase:       0.05,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC),
		Seed:                 42,
		SchemaVersion:        "1.0.0",
	}

	generator := NewShoppingDataGenerator(config)
	events, err := generator.GenerateEvents()
	if err != nil {
		t.Fatalf("Failed to generate events: %v", err)
	}

	// Write to file for inspection
	file, err := os.Create("test_shopping_output.json")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()
	// defer os.Remove("test_shopping_output.json") // Keep for inspection

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(events); err != nil {
		t.Fatalf("Failed to write events to file: %v", err)
	}

	t.Logf("Generated %d events for %d customers", len(events), config.CustomerCount)
}

func TestShoppingDataGenerator_Deterministic(t *testing.T) {
	config := ShoppingGeneratorConfig{
		CustomerCount:        3,
		ProductCount:         10,
		AvgOrdersPerCustomer: 1.0,
		ReturnRateBase:       0.1,
		RefundRateBase:       0.05,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 1, 10, 23, 59, 59, 0, time.UTC),
		Seed:                 12345,
		SchemaVersion:        "1.0.0",
	}

	// Generate twice with same seed
	gen1 := NewShoppingDataGenerator(config)
	events1, err := gen1.GenerateEvents()
	if err != nil {
		t.Fatalf("First generation failed: %v", err)
	}

	gen2 := NewShoppingDataGenerator(config)
	events2, err := gen2.GenerateEvents()
	if err != nil {
		t.Fatalf("Second generation failed: %v", err)
	}

	// Should be identical
	if len(events1) != len(events2) {
		t.Errorf("Event counts differ: %d vs %d", len(events1), len(events2))
	}

	minLen := len(events1)
	if len(events2) < minLen {
		minLen = len(events2)
	}

	for i := 0; i < minLen; i++ {
		if events1[i].EntityID != events2[i].EntityID ||
			events1[i].FieldKey != events2[i].FieldKey ||
			events1[i].Source != events2[i].Source {
			t.Errorf("Events differ at index %d", i)
			break
		}
	}
}

func TestShoppingDataGenerator_HasExpectedFields(t *testing.T) {
	config := ShoppingGeneratorConfig{
		CustomerCount:        2,
		ProductCount:         5,
		AvgOrdersPerCustomer: 1.0,
		ReturnRateBase:       0.0, // No returns for simplicity
		RefundRateBase:       0.0,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 1, 5, 23, 59, 59, 0, time.UTC),
		Seed:                 42,
		SchemaVersion:        "1.0.0",
	}

	generator := NewShoppingDataGenerator(config)
	events, err := generator.GenerateEvents()
	if err != nil {
		t.Fatalf("Failed to generate events: %v", err)
	}

	// Check for expected field types
	expectedFields := map[string]bool{
		"customer_created": false,
		"country":          false,
		"signup_channel":   false,
		"marketing_opt_in": false,
		"session_started":  false,
		"device_type":      false,
		"traffic_source":   false,
		"order_placed":     false,
		"order_total":      false,
		"shipping_speed":   false,
		"delivered":        false,
		"was_returned":     false,
	}

	for _, event := range events {
		if found, exists := expectedFields[event.FieldKey]; exists && !found {
			expectedFields[event.FieldKey] = true
		}
	}

	// Verify all expected fields were generated
	for field, found := range expectedFields {
		if !found {
			t.Errorf("Expected field %s was not generated", field)
		}
	}
}
