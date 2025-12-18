package testkit

import (
	"testing"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
)

// TestShoppingData_Patterns verifies the shopping data contains expected relationships
func TestShoppingData_Patterns(t *testing.T) {
	config := ShoppingGeneratorConfig{
		CustomerCount:        100, // Larger sample for statistical tests
		ProductCount:         50,
		AvgOrdersPerCustomer: 2.0,
		ReturnRateBase:       0.08,
		RefundRateBase:       0.06,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 2, 1, 23, 59, 59, 0, time.UTC),
		Seed:                 12345, // Fixed seed for reproducible tests
		SchemaVersion:        "1.0.0",
	}

	generator := NewShoppingDataGenerator(config)
	events, err := generator.GenerateEvents()
	if err != nil {
		t.Fatalf("Failed to generate events: %v", err)
	}

	// Extract customer data for analysis
	customerData := extractCustomerData(events)

	t.Run("discount_conversion_relationship", func(t *testing.T) {
		// discount_pct should positively correlate with conversion_7d
		discountConversions := 0
		totalDiscounts := 0
		conversionsWithoutDiscount := 0
		totalWithoutDiscount := 0

		for _, customer := range customerData {
			for _, order := range customer.Orders {
				if order.DiscountPct != nil {
					totalDiscounts++
					if order.HasConversion {
						discountConversions++
					}
				} else {
					totalWithoutDiscount++
					if order.HasConversion {
						conversionsWithoutDiscount++
					}
				}
			}
		}

		if totalDiscounts == 0 || totalWithoutDiscount == 0 {
			t.Skip("Not enough data for discount analysis")
		}

		discountRate := float64(discountConversions) / float64(totalDiscounts)
		noDiscountRate := float64(conversionsWithoutDiscount) / float64(totalWithoutDiscount)

		t.Logf("Discount conversion rate: %.2f (%d/%d)", discountRate, discountConversions, totalDiscounts)
		t.Logf("No-discount conversion rate: %.2f (%d/%d)", noDiscountRate, conversionsWithoutDiscount, totalWithoutDiscount)

		// Discounted orders should have higher conversion (this is baked into the generator)
		if discountRate <= noDiscountRate {
			t.Errorf("Expected discounted orders to have higher conversion rate")
		}
	})

	t.Run("shipping_returns_relationship", func(t *testing.T) {
		// shipping_days should positively correlate with returns
		fastShippingReturns := 0
		fastShippingTotal := 0
		slowShippingReturns := 0
		slowShippingTotal := 0

		for _, customer := range customerData {
			for _, order := range customer.Orders {
				if order.ShippingDays <= 2.0 {
					fastShippingTotal++
					if order.WasReturned {
						fastShippingReturns++
					}
				} else {
					slowShippingTotal++
					if order.WasReturned {
						slowShippingReturns++
					}
				}
			}
		}

		if fastShippingTotal == 0 || slowShippingTotal == 0 {
			t.Skip("Not enough shipping data")
		}

		fastReturnRate := float64(fastShippingReturns) / float64(fastShippingTotal)
		slowReturnRate := float64(slowShippingReturns) / float64(slowShippingTotal)

		t.Logf("Fast shipping return rate: %.2f (%d/%d)", fastReturnRate, fastShippingReturns, fastShippingTotal)
		t.Logf("Slow shipping return rate: %.2f (%d/%d)", slowReturnRate, slowShippingReturns, slowShippingTotal)

		// Slow shipping should have higher return rate
		if slowReturnRate <= fastReturnRate {
			t.Errorf("Expected slow shipping to have higher return rate")
		}
	})

	t.Run("confounding_loyalty_tenure", func(t *testing.T) {
		// loyalty_tier should correlate with conversion but tenure_days should be stronger
		highTierConversions := 0
		highTierTotal := 0
		lowTierConversions := 0
		lowTierTotal := 0

		for _, customer := range customerData {
			if customer.LoyaltyTier == "gold" || customer.LoyaltyTier == "silver" {
				highTierTotal++
				if customer.HasAnyConversion {
					highTierConversions++
				}
			} else {
				lowTierTotal++
				if customer.HasAnyConversion {
					lowTierConversions++
				}
			}
		}

		if highTierTotal == 0 || lowTierTotal == 0 {
			t.Skip("Not enough loyalty tier data")
		}

		highTierRate := float64(highTierConversions) / float64(highTierTotal)
		lowTierRate := float64(lowTierConversions) / float64(lowTierTotal)

		t.Logf("High tier conversion rate: %.2f (%d/%d)", highTierRate, highTierConversions, highTierTotal)
		t.Logf("Low tier conversion rate: %.2f (%d/%d)", lowTierRate, lowTierConversions, lowTierTotal)

		// High tier should have higher conversion (confounding relationship)
		if highTierRate <= lowTierRate {
			t.Logf("Warning: Expected high tier to have higher conversion rate (may be weak signal)")
		}
	})

	t.Run("missing_data_patterns", func(t *testing.T) {
		// Check realistic missing data patterns
		totalOrders := 0
		missingDiscount := 0
		missingRiskScore := 0

		for _, customer := range customerData {
			for _, order := range customer.Orders {
				totalOrders++
				if order.DiscountPct == nil {
					missingDiscount++
				}
			}
			if customer.RiskScore == nil {
				missingRiskScore++
			}
		}

		discountMissingRate := float64(missingDiscount) / float64(totalOrders)
		riskScoreMissingRate := float64(missingRiskScore) / float64(len(customerData))

		t.Logf("Discount missing rate: %.2f (%d/%d)", discountMissingRate, missingDiscount, totalOrders)
		t.Logf("Risk score missing rate: %.2f (%d/%d)", riskScoreMissingRate, missingRiskScore, len(customerData))

		// Should have realistic missing rates
		if discountMissingRate < 0.3 || discountMissingRate > 0.8 {
			t.Logf("Warning: Discount missing rate %.2f seems unrealistic", discountMissingRate)
		}
	})

	t.Run("noise_fields_uncorrelated", func(t *testing.T) {
		// Noise fields should not correlate strongly with conversion
		noiseCorrelations := make([]float64, 5)

		for noiseIdx := 0; noiseIdx < 5; noiseIdx++ {
			conversionsWithHighNoise := 0
			totalHighNoise := 0
			conversionsWithLowNoise := 0
			totalLowNoise := 0

			for _, customer := range customerData {
				if customer.NoiseFields[noiseIdx] > 50 { // High noise
					totalHighNoise++
					if customer.HasAnyConversion {
						conversionsWithHighNoise++
					}
				} else { // Low noise
					totalLowNoise++
					if customer.HasAnyConversion {
						conversionsWithLowNoise++
					}
				}
			}

			if totalHighNoise > 0 && totalLowNoise > 0 {
				highRate := float64(conversionsWithHighNoise) / float64(totalHighNoise)
				lowRate := float64(conversionsWithLowNoise) / float64(totalLowNoise)
				noiseCorrelations[noiseIdx] = highRate - lowRate
			}
		}

		t.Logf("Noise field correlations with conversion: %v", noiseCorrelations)

		// Noise correlations should be small (random)
		totalAbsCorrelation := 0.0
		for _, corr := range noiseCorrelations {
			if corr < 0 {
				corr = -corr
			}
			totalAbsCorrelation += corr
		}
		avgAbsCorrelation := totalAbsCorrelation / 5

		if avgAbsCorrelation > 0.1 { // More than 10% difference
			t.Logf("Warning: Noise fields show unexpected correlation (%.3f avg)", avgAbsCorrelation)
		}
	})

	t.Run("deprecated_field_zero_variance", func(t *testing.T) {
		// Deprecated field should always be 0
		nonZeroDeprecated := 0
		totalDeprecated := 0

		for _, customer := range customerData {
			if customer.DeprecatedField != nil {
				totalDeprecated++
				if *customer.DeprecatedField != 0.0 {
					nonZeroDeprecated++
				}
			}
		}

		t.Logf("Deprecated field: %d/%d non-zero values", nonZeroDeprecated, totalDeprecated)

		if nonZeroDeprecated > 0 {
			t.Errorf("Deprecated field should always be 0, found %d non-zero values", nonZeroDeprecated)
		}
	})
}

// Data structures for analysis

type CustomerOrder struct {
	OrderTotal    float64
	DiscountPct   *float64
	ShippingDays  float64
	WasReturned   bool
	HasConversion bool
}

type CustomerData struct {
	ID               core.ID
	Country          string
	SignupChannel    string
	RiskScore        *float64
	TenureDays       *float64
	LoyaltyTier      string
	Orders           []CustomerOrder
	HasAnyConversion bool
	NoiseFields      [5]float64
	DeprecatedField  *float64
}

func extractCustomerData(events []ingestion.CanonicalEvent) map[core.ID]*CustomerData {
	customers := make(map[core.ID]*CustomerData)

	for _, event := range events {
		customerID := event.EntityID

		if customers[customerID] == nil {
			customers[customerID] = &CustomerData{
				ID:     customerID,
				Orders: []CustomerOrder{},
			}
		}

		customer := customers[customerID]

		switch event.FieldKey {
		case "country":
			if event.Value.IsString() {
				customer.Country = event.Value.AsString()
			}
		case "signup_channel":
			if event.Value.IsString() {
				customer.SignupChannel = event.Value.AsString()
			}
		case "risk_score":
			if event.Value.IsNumeric() {
				score := event.Value.AsFloat64()
				customer.RiskScore = &score
			}
		case "tenure_days":
			if event.Value.IsNumeric() {
				tenure := event.Value.AsFloat64()
				customer.TenureDays = &tenure
			}
		case "loyalty_tier":
			if event.Value.IsString() {
				customer.LoyaltyTier = event.Value.AsString()
			}
		case "order_total":
			if len(customer.Orders) > 0 {
				// Update last order
				lastIdx := len(customer.Orders) - 1
				if event.Value.IsNumeric() {
					customer.Orders[lastIdx].OrderTotal = event.Value.AsFloat64()
				}
			}
		case "discount_pct":
			if len(customer.Orders) > 0 {
				lastIdx := len(customer.Orders) - 1
				if event.Value.IsNumeric() {
					discount := event.Value.AsFloat64()
					customer.Orders[lastIdx].DiscountPct = &discount
				}
			}
		case "shipping_days":
			if len(customer.Orders) > 0 {
				lastIdx := len(customer.Orders) - 1
				if event.Value.IsNumeric() {
					customer.Orders[lastIdx].ShippingDays = event.Value.AsFloat64()
				}
			}
		case "was_returned":
			if len(customer.Orders) > 0 {
				lastIdx := len(customer.Orders) - 1
				if event.Value.IsBoolean() {
					customer.Orders[lastIdx].WasReturned = event.Value.AsBoolean()
				}
			}
		case "conversion_7d":
			if len(customer.Orders) > 0 {
				lastIdx := len(customer.Orders) - 1
				if event.Value.IsBoolean() {
					customer.Orders[lastIdx].HasConversion = event.Value.AsBoolean()
					if event.Value.AsBoolean() {
						customer.HasAnyConversion = true
					}
				}
			}
		case "order_placed":
			// Start new order
			customer.Orders = append(customer.Orders, CustomerOrder{})
		default:
			// Check for noise fields
			if len(event.FieldKey) >= 13 && event.FieldKey[:12] == "random_noise" {
				if event.Value.IsNumeric() {
					idx := event.FieldKey[12] - '1' // 1-based to 0-based
					if idx >= 0 && idx < 5 {
						customer.NoiseFields[idx] = event.Value.AsFloat64()
					}
				}
			}
			if event.FieldKey == "deprecated_field" && event.Value.IsNumeric() {
				deprecated := event.Value.AsFloat64()
				customer.DeprecatedField = &deprecated
			}
		}
	}

	return customers
}
