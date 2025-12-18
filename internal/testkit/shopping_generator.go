package testkit

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
)

// ShoppingGeneratorConfig configures the shopping data generator
type ShoppingGeneratorConfig struct {
	CustomerCount        int       `json:"customer_count"`
	ProductCount         int       `json:"product_count"`
	AvgOrdersPerCustomer float64   `json:"avg_orders_per_customer"`
	ReturnRateBase       float64   `json:"return_rate_base"`
	RefundRateBase       float64   `json:"refund_rate_base"`
	StartDate            time.Time `json:"start_date"`
	EndDate              time.Time `json:"end_date"`
	Seed                 int64     `json:"seed"`
	SchemaVersion        string    `json:"schema_version"`
}

// DefaultShoppingConfig returns sensible defaults for shopping data generation
func DefaultShoppingConfig() ShoppingGeneratorConfig {
	return ShoppingGeneratorConfig{
		CustomerCount:        1000,
		ProductCount:         500,
		AvgOrdersPerCustomer: 2.5,
		ReturnRateBase:       0.08,
		RefundRateBase:       0.06,
		StartDate:            time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC),
		Seed:                 42,
		SchemaVersion:        "1.0.0",
	}
}

// ShoppingDataGenerator generates realistic e-commerce event data
type ShoppingDataGenerator struct {
	config ShoppingGeneratorConfig
	rng    *rand.Rand
}

// NewShoppingDataGenerator creates a new shopping data generator
func NewShoppingDataGenerator(config ShoppingGeneratorConfig) *ShoppingDataGenerator {
	return &ShoppingDataGenerator{
		config: config,
		rng:    rand.New(rand.NewSource(config.Seed)),
	}
}

// GenerateEvents generates a complete set of shopping events
func (g *ShoppingDataGenerator) GenerateEvents() ([]ingestion.CanonicalEvent, error) {
	var events []ingestion.CanonicalEvent

	// Generate customers
	for i := 0; i < g.config.CustomerCount; i++ {
		customerID := core.ID(fmt.Sprintf("customer_%04d", i+1))
		customerEvents := g.generateCustomerJourney(customerID)
		events = append(events, customerEvents...)
	}

	return events, nil
}

// generateCustomerJourney generates the complete event journey for one customer
func (g *ShoppingDataGenerator) generateCustomerJourney(customerID core.ID) []ingestion.CanonicalEvent {
	var events []ingestion.CanonicalEvent

	// Customer creation
	signupTime := g.randomTimeInRange(g.config.StartDate, g.config.EndDate.AddDate(0, 0, -30)) // Leave room for activity
	events = append(events, g.customerCreatedEvent(customerID, signupTime)...)

	// Customer profile updates (occasional)
	if g.rng.Float64() < 0.3 { // 30% get profile updates
		updateTime := g.randomTimeInRange(signupTime.AddDate(0, 0, 7), g.config.EndDate)
		events = append(events, g.customerProfileUpdatedEvent(customerID, updateTime, signupTime)...)
	}

	// Sessions and orders (based on avg orders per customer)
	orderCount := int(math.Round(g.config.AvgOrdersPerCustomer + g.rng.NormFloat64()*0.5))
	if orderCount < 0 {
		orderCount = 0
	}
	// Ensure at least one order when the config implies orders should exist.
	// This keeps fixtures stable and prevents downstream tests from flaking when
	// small sample sizes + randomness produce zero orders.
	if orderCount == 0 && g.config.AvgOrdersPerCustomer > 0 {
		orderCount = 1
	}
	if orderCount > 10 { // Cap at 10 orders
		orderCount = 10
	}

	currentTime := signupTime
	for i := 0; i < orderCount; i++ {
		// Space out orders over time, but keep the first order inside the configured window.
		remainingDays := int(g.config.EndDate.Sub(currentTime).Hours() / 24)
		if remainingDays <= 0 {
			break
		}

		minGapDays, maxGapDays := 0, 7 // first order can happen quickly
		if i > 0 {
			minGapDays, maxGapDays = 7, 37 // subsequent orders are spaced out
		}

		// Clamp to remaining window to avoid generating zero-order cohorts for short date ranges.
		if remainingDays < minGapDays {
			minGapDays = 0
		}
		if remainingDays < maxGapDays {
			maxGapDays = remainingDays
		}

		daysUntilNext := 0
		if maxGapDays > 0 {
			daysUntilNext = g.rng.Intn(maxGapDays-minGapDays+1) + minGapDays
		}
		currentTime = currentTime.AddDate(0, 0, daysUntilNext)

		orderID := core.ID(fmt.Sprintf("order_%s_%02d", customerID, i+1))
		orderEvents := g.generateOrderJourney(customerID, orderID, currentTime)
		events = append(events, orderEvents...)
	}

	return events
}

// customerCreatedEvent generates customer creation events
func (g *ShoppingDataGenerator) customerCreatedEvent(customerID core.ID, signupTime time.Time) []ingestion.CanonicalEvent {
	events := []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "customer_created",
			Value:      ingestion.NewTimestampValue(signupTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "country",
			Value:      ingestion.NewStringValue(g.randomCountry()),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "signup_channel",
			Value:      ingestion.NewStringValue(g.randomSignupChannel()),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "marketing_opt_in",
			Value:      ingestion.NewBooleanValue(g.rng.Float64() < 0.6), // 60% opt in
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "tenure_days",
			Value:      ingestion.NewNumericValue(0.0),
		},
	}

	// Risk score (missing for new customers sometimes)
	if g.rng.Float64() < 0.8 { // 80% have risk scores
		riskScore := g.rng.Float64() * 0.5 // 0-0.5 for new customers
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "risk_score",
			Value:      ingestion.NewNumericValue(riskScore),
		})
	}

	// Add confounding: loyalty_tier correlates with tenure but not causally with conversion
	// (tenure_days will be the real driver)
	var loyaltyTier string
	tenureDays := 0.0 // New customer
	if tenureDays > 180 {
		loyaltyTier = "gold"
	} else if tenureDays > 90 {
		loyaltyTier = "silver"
	} else if tenureDays > 30 {
		loyaltyTier = "bronze"
	} else {
		loyaltyTier = ""
	}

	if loyaltyTier != "" {
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   "loyalty_tier",
			Value:      ingestion.NewStringValue(loyaltyTier),
		})
	}

	// Add some noise fields (should not correlate with anything)
	for i := 0; i < 5; i++ {
		noiseValue := g.rng.Float64() * 100 // Random noise 0-100
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(signupTime),
			Source:     "customer_service",
			FieldKey:   fmt.Sprintf("random_noise_%d", i+1),
			Value:      ingestion.NewNumericValue(noiseValue),
		})
	}

	// Add deprecated field (always 0, should be filtered out)
	events = append(events, ingestion.CanonicalEvent{
		EntityID:   customerID,
		ObservedAt: core.NewTimestamp(signupTime),
		Source:     "customer_service",
		FieldKey:   "deprecated_field",
		Value:      ingestion.NewNumericValue(0.0),
	})

	return events
}

// customerProfileUpdatedEvent generates profile update events
func (g *ShoppingDataGenerator) customerProfileUpdatedEvent(customerID core.ID, updateTime time.Time, signupTime time.Time) []ingestion.CanonicalEvent {
	tenureDays := updateTime.Sub(signupTime).Hours() / 24

	events := []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(updateTime),
			Source:     "customer_service",
			FieldKey:   "customer_profile_updated",
			Value:      ingestion.NewTimestampValue(updateTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(updateTime),
			Source:     "customer_service",
			FieldKey:   "tenure_days",
			Value:      ingestion.NewNumericValue(tenureDays),
		},
	}

	// Loyalty tier based on tenure and orders
	var tier string
	if tenureDays > 180 {
		tier = "gold"
	} else if tenureDays > 90 {
		tier = "silver"
	} else if tenureDays > 30 {
		tier = "bronze"
	} else {
		tier = ""
	}

	if tier != "" {
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(updateTime),
			Source:     "customer_service",
			FieldKey:   "loyalty_tier",
			Value:      ingestion.NewStringValue(tier),
		})
	}

	return events
}

// generateOrderJourney generates the complete journey for one order
func (g *ShoppingDataGenerator) generateOrderJourney(customerID, orderID core.ID, orderTime time.Time) []ingestion.CanonicalEvent {
	var events []ingestion.CanonicalEvent

	// Pre-order events (session, cart, checkout)
	sessionEvents := g.generateSessionEvents(customerID, orderTime.Add(-time.Duration(g.rng.Intn(3600))*time.Second))
	events = append(events, sessionEvents...)

	// Order placement
	orderEvents := g.generateOrderPlacementEvents(customerID, orderID, orderTime)
	events = append(events, orderEvents...)

	// Fulfillment and delivery
	fulfillmentEvents := g.generateFulfillmentEvents(customerID, orderID, orderTime)
	events = append(events, fulfillmentEvents...)

	// Post-delivery (returns, refunds)
	postDeliveryEvents := g.generatePostDeliveryEvents(customerID, orderID, orderTime)
	events = append(events, postDeliveryEvents...)

	return events
}

// generateSessionEvents generates session, cart, and checkout events
func (g *ShoppingDataGenerator) generateSessionEvents(customerID core.ID, sessionTime time.Time) []ingestion.CanonicalEvent {
	events := []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(sessionTime),
			Source:     "session_tracking",
			FieldKey:   "session_started",
			Value:      ingestion.NewTimestampValue(sessionTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(sessionTime),
			Source:     "session_tracking",
			FieldKey:   "device_type",
			Value:      ingestion.NewStringValue(g.randomDeviceType()),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(sessionTime),
			Source:     "session_tracking",
			FieldKey:   "traffic_source",
			Value:      ingestion.NewStringValue(g.randomTrafficSource()),
		},
	}

	// Session metrics
	pagesViewed := 2 + g.rng.Intn(8)         // 2-10 pages
	sessionDuration := 60 + g.rng.Intn(1800) // 1-30 minutes

	events = append(events, []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(sessionTime),
			Source:     "session_tracking",
			FieldKey:   "pages_viewed",
			Value:      ingestion.NewNumericValue(float64(pagesViewed)),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(sessionTime),
			Source:     "session_tracking",
			FieldKey:   "session_duration_sec",
			Value:      ingestion.NewNumericValue(float64(sessionDuration)),
		},
	}...)

	// Cart events
	cartTime := sessionTime.Add(time.Duration(g.rng.Intn(600)) * time.Second) // Within 10 minutes
	productID := fmt.Sprintf("product_%04d", g.rng.Intn(g.config.ProductCount)+1)
	cartValue := 20.0 + g.rng.Float64()*180.0 // $20-$200

	events = append(events, []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(cartTime),
			Source:     "cart_service",
			FieldKey:   "add_to_cart",
			Value:      ingestion.NewStringValue(productID),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(cartTime),
			Source:     "cart_service",
			FieldKey:   "cart_value",
			Value:      ingestion.NewNumericValue(cartValue),
		},
	}...)

	return events
}

// generateOrderPlacementEvents generates checkout and order placement events
func (g *ShoppingDataGenerator) generateOrderPlacementEvents(customerID, orderID core.ID, orderTime time.Time) []ingestion.CanonicalEvent {
	checkoutTime := orderTime.Add(-time.Duration(g.rng.Intn(300)) * time.Second) // Within 5 minutes before order

	events := []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(checkoutTime),
			Source:     "checkout_service",
			FieldKey:   "checkout_started",
			Value:      ingestion.NewTimestampValue(checkoutTime),
		},
	}

	// Discount (sometimes missing based on traffic source)
	// This creates a strong positive relationship: discount_pct → conversion_7d
	trafficSource := g.randomTrafficSource()
	var discountPct *float64
	if trafficSource != "organic" && g.rng.Float64() < 0.7 { // 70% get discounts on paid traffic
		pct := 5.0 + float64(g.rng.Intn(20)) // 5-25% discount
		discountPct = &pct
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(checkoutTime),
			Source:     "checkout_service",
			FieldKey:   "discount_pct",
			Value:      ingestion.NewNumericValue(*discountPct),
		})
	} else if trafficSource == "organic" && g.rng.Float64() < 0.1 { // 10% get discounts on organic
		pct := 5.0 + float64(g.rng.Intn(15)) // 5-20% discount
		discountPct = &pct
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(checkoutTime),
			Source:     "checkout_service",
			FieldKey:   "discount_pct",
			Value:      ingestion.NewNumericValue(*discountPct),
		})
	} else {
		// Missing discount
		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(checkoutTime),
			Source:     "checkout_service",
			FieldKey:   "discount_pct",
			Value:      ingestion.NewMissingValue(),
		})
	}

	// Payment events
	paymentTime := orderTime.Add(-time.Duration(g.rng.Intn(60)) * time.Second) // Within 1 minute before order
	events = append(events, []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(paymentTime),
			Source:     "payment_service",
			FieldKey:   "payment_authorized",
			Value:      ingestion.NewTimestampValue(paymentTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(paymentTime),
			Source:     "payment_service",
			FieldKey:   "payment_method",
			Value:      ingestion.NewStringValue(g.randomPaymentMethod()),
		},
	}...)

	// Order placement
	orderTotal := 50.0 + g.rng.Float64()*450.0 // $50-$500
	itemsCount := 1 + g.rng.Intn(5)            // 1-5 items

	// Add conversion_7d field (will be true for this order)
	conversionTime := orderTime.AddDate(0, 0, 1) // 1 day later
	events = append(events, ingestion.CanonicalEvent{
		EntityID:   customerID,
		ObservedAt: core.NewTimestamp(conversionTime),
		Source:     "analytics_service",
		FieldKey:   "conversion_7d",
		Value:      ingestion.NewBooleanValue(true),
	})

	events = append(events, []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(orderTime),
			Source:     "order_service",
			FieldKey:   "order_placed",
			Value:      ingestion.NewTimestampValue(orderTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(orderTime),
			Source:     "order_service",
			FieldKey:   "order_id",
			Value:      ingestion.NewStringValue(string(orderID)),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(orderTime),
			Source:     "order_service",
			FieldKey:   "order_total",
			Value:      ingestion.NewNumericValue(orderTotal),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(orderTime),
			Source:     "order_service",
			FieldKey:   "items_count",
			Value:      ingestion.NewNumericValue(float64(itemsCount)),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(orderTime),
			Source:     "order_service",
			FieldKey:   "shipping_speed",
			Value:      ingestion.NewStringValue(g.randomShippingSpeed()),
		},
	}...)

	return events
}

// generateFulfillmentEvents generates shipping and delivery events
func (g *ShoppingDataGenerator) generateFulfillmentEvents(customerID, orderID core.ID, orderTime time.Time) []ingestion.CanonicalEvent {
	// Shipping delay based on shipping speed
	var shippingDelay time.Duration
	switch g.randomShippingSpeed() {
	case "standard":
		shippingDelay = time.Duration(2+g.rng.Intn(3)) * 24 * time.Hour // 2-4 days
	case "expedited":
		shippingDelay = time.Duration(1+g.rng.Intn(2)) * 24 * time.Hour // 1-2 days
	default:
		shippingDelay = time.Duration(3+g.rng.Intn(2)) * 24 * time.Hour // 3-4 days
	}

	shipTime := orderTime.Add(shippingDelay)
	deliveryTime := shipTime.Add(time.Duration(1+g.rng.Intn(3)) * 24 * time.Hour) // 1-3 days after shipping

	events := []ingestion.CanonicalEvent{
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(shipTime),
			Source:     "fulfillment_service",
			FieldKey:   "order_shipped",
			Value:      ingestion.NewTimestampValue(shipTime),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(shipTime),
			Source:     "fulfillment_service",
			FieldKey:   "shipping_days",
			Value:      ingestion.NewNumericValue(float64(shippingDelay.Hours() / 24)),
		},
		{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(deliveryTime),
			Source:     "delivery_service",
			FieldKey:   "delivered",
			Value:      ingestion.NewTimestampValue(deliveryTime),
		},
	}

	// Was returned? (based on return rate + shipping days)
	// This creates: shipping_days ↑ → return_rate ↑ (positive relationship)
	returnRate := g.config.ReturnRateBase
	if shippingDelay.Hours()/24 > 3 { // Higher return rate for slow shipping
		returnRate += 0.1 // Strong effect
	} else if shippingDelay.Hours()/24 > 2 {
		returnRate += 0.05 // Moderate effect
	}
	wasReturned := g.rng.Float64() < returnRate

	events = append(events, ingestion.CanonicalEvent{
		EntityID:   customerID,
		ObservedAt: core.NewTimestamp(deliveryTime),
		Source:     "delivery_service",
		FieldKey:   "was_returned",
		Value:      ingestion.NewBooleanValue(wasReturned),
	})

	return events
}

// generatePostDeliveryEvents generates return and refund events
func (g *ShoppingDataGenerator) generatePostDeliveryEvents(customerID, orderID core.ID, orderTime time.Time) []ingestion.CanonicalEvent {
	var events []ingestion.CanonicalEvent

	// Find if this order was returned (we'd need to check the delivery event, but for simplicity...)
	if g.rng.Float64() < g.config.ReturnRateBase {
		// Return initiated 1-7 days after delivery
		returnDelay := time.Duration(1+g.rng.Intn(7)) * 24 * time.Hour
		returnTime := orderTime.AddDate(0, 0, 7).Add(returnDelay) // Base 7 days + random

		events = append(events, ingestion.CanonicalEvent{
			EntityID:   customerID,
			ObservedAt: core.NewTimestamp(returnTime),
			Source:     "return_service",
			FieldKey:   "return_started",
			Value:      ingestion.NewTimestampValue(returnTime),
		})

		// Refund (if applicable)
		if g.rng.Float64() < g.config.RefundRateBase {
			refundTime := returnTime.Add(time.Duration(1+g.rng.Intn(5)) * 24 * time.Hour) // 1-5 days after return
			refundAmount := 50.0 + g.rng.Float64()*450.0                                  // Same range as order total

			events = append(events, []ingestion.CanonicalEvent{
				{
					EntityID:   customerID,
					ObservedAt: core.NewTimestamp(refundTime),
					Source:     "refund_service",
					FieldKey:   "refund_issued",
					Value:      ingestion.NewTimestampValue(refundTime),
				},
				{
					EntityID:   customerID,
					ObservedAt: core.NewTimestamp(refundTime),
					Source:     "refund_service",
					FieldKey:   "refund_amount",
					Value:      ingestion.NewNumericValue(refundAmount),
				},
			}...)
		}
	}

	return events
}

// Helper methods for random value generation

func (g *ShoppingDataGenerator) randomTimeInRange(start, end time.Time) time.Time {
	if start.After(end) {
		start, end = end, start // Swap if in wrong order
	}
	duration := end.Sub(start)
	if duration <= 0 {
		return start
	}
	randomDuration := time.Duration(g.rng.Int63n(int64(duration)))
	return start.Add(randomDuration)
}

func (g *ShoppingDataGenerator) randomCountry() string {
	countries := []string{"US", "CA", "GB", "DE", "FR", "AU", "JP"}
	return countries[g.rng.Intn(len(countries))]
}

func (g *ShoppingDataGenerator) randomSignupChannel() string {
	channels := []string{"organic", "paid_search", "social", "email", "direct"}
	weights := []float64{0.4, 0.3, 0.15, 0.1, 0.05} // Organic most common

	r := g.rng.Float64()
	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return channels[i]
		}
	}
	return channels[0]
}

func (g *ShoppingDataGenerator) randomDeviceType() string {
	devices := []string{"mobile", "desktop", "tablet"}
	weights := []float64{0.6, 0.35, 0.05} // Mobile most common

	r := g.rng.Float64()
	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return devices[i]
		}
	}
	return devices[0]
}

func (g *ShoppingDataGenerator) randomTrafficSource() string {
	sources := []string{"organic", "paid_search", "email", "social", "direct"}
	weights := []float64{0.35, 0.25, 0.2, 0.15, 0.05}

	r := g.rng.Float64()
	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return sources[i]
		}
	}
	return sources[0]
}

func (g *ShoppingDataGenerator) randomPaymentMethod() string {
	methods := []string{"credit_card", "debit_card", "paypal", "apple_pay", "bank_transfer"}
	weights := []float64{0.5, 0.2, 0.15, 0.1, 0.05}

	r := g.rng.Float64()
	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return methods[i]
		}
	}
	return methods[0]
}

func (g *ShoppingDataGenerator) randomShippingSpeed() string {
	speeds := []string{"standard", "expedited"}
	weights := []float64{0.8, 0.2} // Most choose standard

	r := g.rng.Float64()
	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return speeds[i]
		}
	}
	return speeds[0]
}
