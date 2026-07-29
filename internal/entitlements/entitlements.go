// Package entitlements defines subscription-plan quotas shared by tenant
// services and platform usage reporting.
package entitlements

import (
	"errors"
	"fmt"
	"strings"
)

const (
	ResourceEmployees          = "employees"
	ResourceJourneyTemplates   = "journey_templates"
	ResourceKnowledgeDocuments = "knowledge_documents"
	ResourceIntegrations       = "integrations"
	Unlimited                  = -1
)

var ErrLimitExceeded = errors.New("subscription plan limit exceeded")

type Limits struct {
	Employees          int `json:"employees"`
	JourneyTemplates   int `json:"journeyTemplates"`
	KnowledgeDocuments int `json:"knowledgeDocuments"`
	Integrations       int `json:"integrations"`
}

type UsageItem struct {
	Resource string `json:"resource"`
	Used     int    `json:"used"`
	Limit    int    `json:"limit"`
}

type Usage struct {
	PlanCode string      `json:"planCode"`
	Items    []UsageItem `json:"items"`
}

func ForPlan(planCode string) Limits {
	switch strings.ToLower(strings.TrimSpace(planCode)) {
	case "growth":
		return Limits{Employees: 250, JourneyTemplates: 50, KnowledgeDocuments: 500, Integrations: 10}
	case "enterprise":
		return Limits{
			Employees: Unlimited, JourneyTemplates: Unlimited,
			KnowledgeDocuments: Unlimited, Integrations: Unlimited,
		}
	default:
		return Limits{Employees: 25, JourneyTemplates: 5, KnowledgeDocuments: 25, Integrations: 2}
	}
}

func Limit(planCode, resource string) int {
	limits := ForPlan(planCode)
	switch resource {
	case ResourceEmployees:
		return limits.Employees
	case ResourceJourneyTemplates:
		return limits.JourneyTemplates
	case ResourceKnowledgeDocuments:
		return limits.KnowledgeDocuments
	case ResourceIntegrations:
		return limits.Integrations
	default:
		return 0
	}
}

func Check(planCode, resource string, used int) error {
	limit := Limit(planCode, resource)
	if limit == Unlimited || used < limit {
		return nil
	}
	return fmt.Errorf("%w: %s allows %d %s", ErrLimitExceeded, planCode, limit, resource)
}

func NewUsage(planCode string, counts map[string]int) Usage {
	resources := []string{
		ResourceEmployees,
		ResourceJourneyTemplates,
		ResourceKnowledgeDocuments,
		ResourceIntegrations,
	}
	items := make([]UsageItem, 0, len(resources))
	for _, resource := range resources {
		items = append(items, UsageItem{
			Resource: resource,
			Used:     counts[resource],
			Limit:    Limit(planCode, resource),
		})
	}
	return Usage{PlanCode: planCode, Items: items}
}
