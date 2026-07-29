package assessments

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
)

// Service implements assessment use cases.
type Service struct {
	repo      Repository
	employees EmployeeReader
	// shuffle randomizes question order per take; injectable for tests.
	shuffle func(n int, swap func(i, j int))
}

// NewService constructs a Service.
func NewService(repo Repository, employeeReader EmployeeReader) *Service {
	return &Service{repo: repo, employees: employeeReader, shuffle: rand.Shuffle}
}

// SetShuffle overrides the question shuffler. It exists for tests; production
// wiring keeps the default math/rand/v2 shuffle.
func (s *Service) SetShuffle(shuffle func(n int, swap func(i, j int))) {
	s.shuffle = shuffle
}

// Create registers a new draft assessment for a tenant.
func (s *Service) Create(
	ctx context.Context,
	organizationID, createdByUserID string,
	in CreateAssessmentInput,
) (Assessment, error) {
	title := strings.TrimSpace(in.Title)
	if organizationID == "" || createdByUserID == "" || title == "" {
		return Assessment{}, ErrInvalidInput
	}

	questions, err := normalizeQuestions(in.Questions)
	if err != nil {
		return Assessment{}, err
	}

	if err := validateSettings(in.PassingScore, in.MaxAttempts); err != nil {
		return Assessment{}, err
	}

	now := time.Now().UTC()

	assessment := Assessment{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Title:          title,
		Description:    strings.TrimSpace(in.Description),
		Questions:      questions,
		PassingScore:   in.PassingScore,
		MaxAttempts:    in.MaxAttempts,
		Randomize:      in.Randomize,
		Status:         StatusDraft,
		CreatedBy:      createdByUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.CreateAssessment(ctx, assessment); err != nil {
		return Assessment{}, fmt.Errorf("create assessment: %w", err)
	}

	return assessment, nil
}

// List returns a tenant's assessments, newest first.
func (s *Service) List(ctx context.Context, organizationID string) ([]Assessment, error) {
	if organizationID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.ListAssessments(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}

	return items, nil
}

// Get returns one assessment scoped to the tenant, answer keys included
// (manager view).
func (s *Service) Get(ctx context.Context, organizationID, assessmentID string) (Assessment, error) {
	return s.load(ctx, organizationID, assessmentID)
}

// Update mutates editable fields of a draft assessment.
func (s *Service) Update(
	ctx context.Context,
	organizationID, assessmentID string,
	in UpdateAssessmentInput,
) (Assessment, error) {
	assessment, err := s.load(ctx, organizationID, assessmentID)
	if err != nil {
		return Assessment{}, err
	}

	if assessment.Status != StatusDraft {
		return Assessment{}, ErrInvalidState
	}

	assessment, err = applyUpdate(assessment, in)
	if err != nil {
		return Assessment{}, err
	}

	assessment.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateAssessment(ctx, assessment); err != nil {
		return Assessment{}, fmt.Errorf("update assessment: %w", err)
	}

	return assessment, nil
}

// applyUpdate overlays the non-nil fields of in onto assessment, validating
// each.
func applyUpdate(assessment Assessment, in UpdateAssessmentInput) (Assessment, error) {
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return Assessment{}, ErrInvalidInput
		}

		assessment.Title = title
	}

	if in.Description != nil {
		assessment.Description = strings.TrimSpace(*in.Description)
	}

	if in.Questions != nil {
		questions, err := normalizeQuestions(*in.Questions)
		if err != nil {
			return Assessment{}, err
		}

		assessment.Questions = questions
	}

	passingScore := assessment.PassingScore
	if in.PassingScore != nil {
		passingScore = *in.PassingScore
	}

	maxAttempts := assessment.MaxAttempts
	if in.MaxAttempts != nil {
		maxAttempts = *in.MaxAttempts
	}

	if err := validateSettings(passingScore, maxAttempts); err != nil {
		return Assessment{}, err
	}

	assessment.PassingScore = passingScore
	assessment.MaxAttempts = maxAttempts

	if in.Randomize != nil {
		assessment.Randomize = *in.Randomize
	}

	return assessment, nil
}

// Publish moves a draft assessment to published, making it available to
// employees and journey assessment steps.
func (s *Service) Publish(ctx context.Context, organizationID, assessmentID string) (Assessment, error) {
	assessment, err := s.load(ctx, organizationID, assessmentID)
	if err != nil {
		return Assessment{}, err
	}

	if assessment.Status != StatusDraft {
		return Assessment{}, ErrInvalidState
	}

	if len(assessment.Questions) == 0 {
		return Assessment{}, fmt.Errorf("%w: cannot publish without questions", ErrInvalidInput)
	}

	assessment.Status = StatusPublished
	assessment.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateAssessment(ctx, assessment); err != nil {
		return Assessment{}, fmt.Errorf("publish assessment: %w", err)
	}

	return assessment, nil
}

// Archive withdraws an assessment from use.
func (s *Service) Archive(ctx context.Context, organizationID, assessmentID string) (Assessment, error) {
	assessment, err := s.load(ctx, organizationID, assessmentID)
	if err != nil {
		return Assessment{}, err
	}

	if assessment.Status == StatusArchived {
		return Assessment{}, ErrInvalidState
	}

	assessment.Status = StatusArchived
	assessment.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateAssessment(ctx, assessment); err != nil {
		return Assessment{}, fmt.Errorf("archive assessment: %w", err)
	}

	return assessment, nil
}

// Take returns the answer-key-free questions for an employee, shuffled when
// the assessment randomizes, plus the employee's remaining attempt budget.
func (s *Service) Take(
	ctx context.Context,
	organizationID, userID, assessmentID string,
) (TakeView, error) {
	assessment, err := s.loadPublished(ctx, organizationID, assessmentID)
	if err != nil {
		return TakeView{}, err
	}

	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return TakeView{}, err
	}

	used, err := s.repo.CountAttempts(ctx, organizationID, assessmentID, employee.ID)
	if err != nil {
		return TakeView{}, fmt.Errorf("count assessment attempts: %w", err)
	}

	questions := publicQuestions(assessment.Questions)
	if assessment.Randomize {
		s.shuffle(len(questions), func(i, j int) {
			questions[i], questions[j] = questions[j], questions[i]
		})
	}

	return TakeView{
		AssessmentID:      assessment.ID,
		Title:             assessment.Title,
		Description:       assessment.Description,
		PassingScore:      assessment.PassingScore,
		Questions:         questions,
		AttemptsUsed:      int(used),
		AttemptsRemaining: attemptsRemaining(assessment.MaxAttempts, int(used)),
	}, nil
}

// SubmitAttempt grades an employee's submission server-side: client-supplied
// scores are never trusted. Attempt limits are enforced before grading. A
// fully auto-graded attempt is final immediately; an attempt with an
// unmatched short answer stays pending_review until a manager finalizes it.
// A passing graded attempt issues a certificate (once per employee and
// assessment).
func (s *Service) SubmitAttempt(
	ctx context.Context,
	organizationID, userID, assessmentID string,
	in SubmitAttemptInput,
) (Attempt, error) {
	assessment, err := s.loadPublished(ctx, organizationID, assessmentID)
	if err != nil {
		return Attempt{}, err
	}

	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return Attempt{}, err
	}

	if len(in.Answers) == 0 {
		return Attempt{}, fmt.Errorf("%w: answers are required", ErrInvalidInput)
	}

	used, err := s.repo.CountAttempts(ctx, organizationID, assessmentID, employee.ID)
	if err != nil {
		return Attempt{}, fmt.Errorf("count assessment attempts: %w", err)
	}

	if assessment.MaxAttempts > 0 && int(used) >= assessment.MaxAttempts {
		return Attempt{}, ErrAttemptsExhausted
	}

	score, pendingReview := gradeAttempt(assessment.Questions, in.Answers)

	now := time.Now().UTC()
	attempt := Attempt{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		AssessmentID:   assessmentID,
		EmployeeID:     employee.ID,
		Answers:        in.Answers,
		Score:          score,
		Passed:         false,
		Status:         AttemptGraded,
		AttemptNumber:  int(used) + 1,
		StartedAt:      now,
		SubmittedAt:    now,
	}

	if pendingReview {
		attempt.Status = AttemptPendingReview
	} else {
		attempt.Passed = score >= assessment.PassingScore
	}

	if err := s.repo.CreateAttempt(ctx, attempt); err != nil {
		return Attempt{}, fmt.Errorf("create assessment attempt: %w", err)
	}

	if attempt.Passed {
		if err := s.issueCertificate(ctx, assessment, employee, attempt); err != nil {
			return Attempt{}, err
		}
	}

	return attempt, nil
}

// ReviewAttempt finalizes a pending-review attempt with the manager's score
// and note. The final pass/fail derives from the assessment's passing score;
// a pass issues the certificate.
func (s *Service) ReviewAttempt(
	ctx context.Context,
	organizationID, reviewerUserID, assessmentID, attemptID string,
	in ReviewAttemptInput,
) (Attempt, error) {
	if reviewerUserID == "" || in.Score < 0 || in.Score > percentScale {
		return Attempt{}, ErrInvalidInput
	}

	assessment, err := s.load(ctx, organizationID, assessmentID)
	if err != nil {
		return Attempt{}, err
	}

	attempt, err := s.repo.GetAttempt(ctx, organizationID, assessmentID, attemptID)
	if err != nil {
		return Attempt{}, fmt.Errorf("get assessment attempt: %w", err)
	}

	if attempt.Status != AttemptPendingReview {
		return Attempt{}, ErrInvalidState
	}

	attempt.Score = in.Score
	attempt.Passed = in.Score >= assessment.PassingScore
	attempt.Status = AttemptGraded
	attempt.ReviewNote = strings.TrimSpace(in.Note)
	attempt.ReviewedBy = reviewerUserID

	if err := s.repo.UpdateAttempt(ctx, attempt); err != nil {
		return Attempt{}, fmt.Errorf("finalize assessment attempt: %w", err)
	}

	if attempt.Passed {
		employee, err := s.employees.Get(ctx, organizationID, attempt.EmployeeID)
		if err != nil {
			return Attempt{}, fmt.Errorf("load employee for certificate: %w", err)
		}

		if err := s.issueCertificate(ctx, assessment, employee, attempt); err != nil {
			return Attempt{}, err
		}
	}

	return attempt, nil
}

// ListAttempts returns every attempt on an assessment (manager view),
// newest first.
func (s *Service) ListAttempts(ctx context.Context, organizationID, assessmentID string) ([]Attempt, error) {
	if _, err := s.load(ctx, organizationID, assessmentID); err != nil {
		return nil, err
	}

	items, err := s.repo.ListAttempts(ctx, organizationID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("list assessment attempts: %w", err)
	}

	return items, nil
}

// ListMyCertificates returns the caller's certificates, newest first.
func (s *Service) ListMyCertificates(
	ctx context.Context,
	organizationID, userID string,
) ([]Certificate, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListCertificatesForEmployee(ctx, organizationID, employee.ID)
	if err != nil {
		return nil, fmt.Errorf("list certificates: %w", err)
	}

	return items, nil
}

// LatestAttemptPassed reports whether the employee's most recent attempt on
// the assessment is graded and passed. Journey assessment steps complete
// through this check (PRD §5.3.6). Called cross-module by internal/assignments.
func (s *Service) LatestAttemptPassed(
	ctx context.Context,
	organizationID, assessmentID, employeeID string,
) (bool, error) {
	attempt, err := s.repo.LatestAttempt(ctx, organizationID, assessmentID, employeeID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAttemptNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("load latest assessment attempt: %w", err)
	}

	return attempt.Status == AttemptGraded && attempt.Passed, nil
}

// issueCertificate creates the certificate for a passed attempt, skipping
// the insert when the employee already holds one for the assessment.
func (s *Service) issueCertificate(
	ctx context.Context,
	assessment Assessment,
	employee employees.Employee,
	attempt Attempt,
) error {
	if _, err := s.repo.FindCertificate(ctx, assessment.OrganizationID, assessment.ID, attempt.EmployeeID); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("check existing certificate: %w", err)
	}

	certificate := Certificate{
		ID:              uuid.NewString(),
		OrganizationID:  assessment.OrganizationID,
		EmployeeID:      attempt.EmployeeID,
		EmployeeName:    employeeDisplayName(employee),
		AssessmentID:    assessment.ID,
		AssessmentTitle: assessment.Title,
		Score:           attempt.Score,
		Serial:          newSerial(),
		IssuedAt:        time.Now().UTC(),
	}

	if err := s.repo.CreateCertificate(ctx, certificate); err != nil {
		return fmt.Errorf("issue certificate: %w", err)
	}

	return nil
}

func (s *Service) load(ctx context.Context, organizationID, assessmentID string) (Assessment, error) {
	if organizationID == "" || assessmentID == "" {
		return Assessment{}, ErrInvalidInput
	}

	assessment, err := s.repo.GetAssessment(ctx, organizationID, assessmentID)
	if err != nil {
		return Assessment{}, fmt.Errorf("get assessment: %w", err)
	}

	return assessment, nil
}

func (s *Service) loadPublished(
	ctx context.Context,
	organizationID, assessmentID string,
) (Assessment, error) {
	assessment, err := s.load(ctx, organizationID, assessmentID)
	if err != nil {
		return Assessment{}, err
	}

	if assessment.Status != StatusPublished {
		return Assessment{}, ErrNotPublished
	}

	return assessment, nil
}

func (s *Service) resolveEmployee(
	ctx context.Context,
	organizationID, userID string,
) (employees.Employee, error) {
	if userID == "" {
		return employees.Employee{}, ErrInvalidInput
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		return employees.Employee{}, fmt.Errorf("resolve employee: %w", err)
	}

	return employee, nil
}

func validateSettings(passingScore float64, maxAttempts int) error {
	if passingScore < 0 || passingScore > percentScale || maxAttempts < 0 {
		return fmt.Errorf("%w: passingScore must be 0-100 and maxAttempts non-negative", ErrInvalidInput)
	}

	return nil
}

func attemptsRemaining(maxAttempts, used int) int {
	if maxAttempts == 0 {
		return -1
	}

	return max(maxAttempts-used, 0)
}

func employeeDisplayName(employee employees.Employee) string {
	name := strings.TrimSpace(employee.FirstName + " " + employee.LastName)
	if name == "" {
		return employee.ID
	}

	return name
}

// newSerial returns a human-shareable certificate serial, e.g. LP-9F2K7Q1A.
func newSerial() string {
	return "LP-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
}
