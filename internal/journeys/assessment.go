package journeys

import (
	"fmt"
	"strings"
)

// assessmentConfigKey is the Config key linking an assessment step to a
// published assessment (PRD §5.3.6).
const assessmentConfigKey = "assessmentId"

// ValidateAssessmentConfig enforces the assessment step invariant: the step
// must link an assessment by id. Existence is not checked here — assessments
// live in another module and are verified when the step is taken.
func ValidateAssessmentConfig(config map[string]any) error {
	if AssessmentIDFromConfig(config) == "" {
		return fmt.Errorf("%w: assessment steps require an assessmentId", ErrInvalidInput)
	}

	return nil
}

// AssessmentIDFromConfig extracts the linked assessment id from an
// assessment step's Config, or "" when absent or malformed.
func AssessmentIDFromConfig(config map[string]any) string {
	raw, ok := config[assessmentConfigKey].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(raw)
}
