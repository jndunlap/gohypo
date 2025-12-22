package analysis

import (
	"strings"
)

// BusinessNameMapper provides business-friendly column name mappings
type BusinessNameMapper struct {
	mappings map[string][]string
}

// NewBusinessNameMapper creates a new business name mapper with Amazon.csv specific mappings
func NewBusinessNameMapper() *BusinessNameMapper {
	return &BusinessNameMapper{
		mappings: map[string][]string{
			"price":          {"price", "cost", "amount", "value", "rate", "fee", "pricing", "unitprice", "totalamount"},
			"customer_segment": {"segment", "category", "type", "class", "group", "tier", "cohort"},
			"conversion":     {"conversion", "purchase", "sale", "order", "transaction", "acquisition"},
			"revenue":        {"revenue", "income", "sales", "profit", "earnings", "gmv", "totalamount"},
			"customer_id":    {"id", "customer_id", "user_id", "client_id", "customer", "orderid", "productid", "sellerid"},
			"timestamp":      {"date", "time", "timestamp", "created_at", "updated_at", "period", "orderdate"},
			"age":           {"age", "years", "duration", "tenure", "lifetime"},
			"score":         {"score", "rating", "rank", "grade", "performance"},
			"quantity":      {"quantity", "count", "number", "volume", "amount"},
			"location":      {"location", "city", "state", "country", "region", "area", "geo"},
			"channel":       {"channel", "source", "medium", "platform", "touchpoint", "paymentmethod"},
			"device":        {"device", "browser", "mobile", "desktop", "platform"},
			"campaign":      {"campaign", "promotion", "offer", "deal", "initiative", "discount"},
			"engagement":    {"engagement", "activity", "interaction", "usage"},
			"satisfaction":  {"satisfaction", "nps", "csat", "feedback"},
			"status":        {"status", "orderstatus"},
			"product":       {"product", "productname", "brand"},
			"shipping":      {"shipping", "shippingcost"},
			"tax":          {"tax"},
		},
	}
}

// MapColumnToBusinessName converts a technical column name to a business-friendly name
func (m *BusinessNameMapper) MapColumnToBusinessName(columnName, outcomeCol string) string {
	colLower := strings.ToLower(columnName)

	// Handle outcome variable with natural business terms
	if colLower == strings.ToLower(outcomeCol) {
		if strings.Contains(colLower, "totalamount") {
			return "total order value"
		} else if strings.Contains(colLower, "orderstatus") {
			return "order delivery success"
		} else if m.containsAny(colLower, []string{"conversion", "purchase", "sale", "order", "transaction"}) {
			return "purchase conversion"
		} else if m.containsAny(colLower, []string{"revenue", "sales", "income", "profit"}) {
			return "revenue performance"
		} else if m.containsAny(colLower, []string{"satisfaction", "nps", "csat", "rating"}) {
			return "customer satisfaction"
		} else if strings.Contains(colLower, "churn") {
			return "customer retention"
		} else if strings.Contains(colLower, "engagement") {
			return "customer engagement"
		}
		return "business performance"
	}

	// Amazon.csv specific mappings
	switch colLower {
	case "orderid":
		return "order identifiers"
	case "orderdate":
		return "order dates"
	case "customerid":
		return "customer identification"
	case "customername":
		return "customer names"
	case "productid":
		return "product identification"
	case "productname":
		return "product names"
	case "category":
		return "product categories"
	case "brand":
		return "product brands"
	case "quantity":
		return "order quantities"
	case "unitprice":
		return "product unit pricing"
	case "totalamount":
		return "order total value"
	case "orderstatus":
		return "delivery status"
	case "customersegment":
		return "customer segments"
	case "paymentmethod":
		return "payment methods"
	case "shippingcost":
		return "shipping costs"
	}

	// Generic business term mappings
	for businessTerm, technicalTerms := range m.mappings {
		for _, term := range technicalTerms {
			if strings.Contains(colLower, term) {
				return m.businessTermToName(businessTerm)
			}
		}
	}

	// Default: convert camelCase/snake_case to readable form
	return m.toReadableName(columnName)
}

// containsAny checks if the string contains any of the provided substrings
func (m *BusinessNameMapper) containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// businessTermToName converts a business term key to a readable name
func (m *BusinessNameMapper) businessTermToName(term string) string {
	switch term {
	case "price":
		return "pricing"
	case "customer_segment":
		return "customer segments"
	case "conversion":
		return "conversion rates"
	case "revenue":
		return "revenue metrics"
	case "customer_id":
		return "customer identification"
	case "timestamp":
		return "time periods"
	case "age":
		return "customer age"
	case "score":
		return "performance scores"
	case "quantity":
		return "quantities"
	case "location":
		return "geographic data"
	case "channel":
		return "marketing channels"
	case "device":
		return "device types"
	case "campaign":
		return "campaign data"
	case "engagement":
		return "engagement metrics"
	case "satisfaction":
		return "satisfaction scores"
	case "status":
		return "status indicators"
	case "product":
		return "product information"
	case "shipping":
		return "shipping details"
	case "tax":
		return "tax information"
	default:
		return strings.ReplaceAll(term, "_", " ")
	}
}

// toReadableName converts technical names to readable business names
func (m *BusinessNameMapper) toReadableName(name string) string {
	// Handle camelCase
	if strings.ContainsAny(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		// Simple camelCase to space separation
		result := ""
		for i, r := range name {
			if i > 0 && r >= 'A' && r <= 'Z' {
				result += " "
			}
			if i == 0 {
				result += strings.ToUpper(string(r))
			} else {
				result += strings.ToLower(string(r))
			}
		}
		return result
	}

	// Handle snake_case
	if strings.Contains(name, "_") {
		parts := strings.Split(name, "_")
		for i, part := range parts {
			parts[i] = strings.Title(strings.ToLower(part))
		}
		return strings.Join(parts, " ")
	}

	// Default: capitalize first letter
	if len(name) > 0 {
		return strings.ToUpper(string(name[0])) + strings.ToLower(name[1:])
	}

	return name
}
