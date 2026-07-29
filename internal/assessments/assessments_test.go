package assessments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/assessments"
	"launchpad/internal/employees"
)

const (
	testOrgID       = "org-1"
	testOtherOrgID  = "org-2"
	testUserID      = "user-1"
	testManagerID   = "user-manager"
	testEmployeeID  = "emp-1"
	testTitle       = "Security basics"
	createdByUserID = "user-admin"
)

// memoryRepo is an in-memory, tenant-scoped assessments.Repository.
type memoryRepo struct {
	assessments  map[string]assessments.Assessment
	attempts     map[string]assessments.Attempt
	certificates map[string]assessments.Certificate
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		assessments:  map[string]assessments.Assessment{},
		attempts:     map[string]assessments.Attempt{},
		certificates: map[string]assessments.Certificate{},
	}
}

func (m *memoryRepo) EnsureIndexes(context.Context) error { return nil }

func (m *memoryRepo) CreateAssessment(_ context.Context, assessment assessments.Assessment) error {
	m.assessments[assessment.ID] = assessment

	return nil
}

func (m *memoryRepo) GetAssessment(
	_ context.Context,
	organizationID, assessmentID string,
) (assessments.Assessment, error) {
	assessment, ok := m.assessments[assessmentID]
	if !ok || assessment.OrganizationID != organizationID {
		return assessments.Assessment{}, assessments.ErrNotFound
	}

	return assessment, nil
}

func (m *memoryRepo) ListAssessments(
	_ context.Context,
	organizationID string,
) ([]assessments.Assessment, error) {
	items := make([]assessments.Assessment, 0)

	for _, assessment := range m.assessments {
		if assessment.OrganizationID == organizationID {
			items = append(items, assessment)
		}
	}

	return items, nil
}

func (m *memoryRepo) UpdateAssessment(_ context.Context, assessment assessments.Assessment) error {
	existing, ok := m.assessments[assessment.ID]
	if !ok || existing.OrganizationID != assessment.OrganizationID {
		return assessments.ErrNotFound
	}

	m.assessments[assessment.ID] = assessment

	return nil
}

func (m *memoryRepo) CreateAttempt(_ context.Context, attempt assessments.Attempt) error {
	m.attempts[attempt.ID] = attempt

	return nil
}

func (m *memoryRepo) GetAttempt(
	_ context.Context,
	organizationID, assessmentID, attemptID string,
) (assessments.Attempt, error) {
	attempt, ok := m.attempts[attemptID]
	if !ok || attempt.OrganizationID != organizationID || attempt.AssessmentID != assessmentID {
		return assessments.Attempt{}, assessments.ErrAttemptNotFound
	}

	return attempt, nil
}

func (m *memoryRepo) CountAttempts(
	_ context.Context,
	organizationID, assessmentID, employeeID string,
) (int64, error) {
	var count int64

	for _, attempt := range m.attempts {
		if attempt.OrganizationID == organizationID &&
			attempt.AssessmentID == assessmentID &&
			attempt.EmployeeID == employeeID {
			count++
		}
	}

	return count, nil
}

func (m *memoryRepo) ListAttempts(
	_ context.Context,
	organizationID, assessmentID string,
) ([]assessments.Attempt, error) {
	items := make([]assessments.Attempt, 0)

	for _, attempt := range m.attempts {
		if attempt.OrganizationID == organizationID && attempt.AssessmentID == assessmentID {
			items = append(items, attempt)
		}
	}

	return items, nil
}

func (m *memoryRepo) LatestAttempt(
	_ context.Context,
	organizationID, assessmentID, employeeID string,
) (assessments.Attempt, error) {
	var latest assessments.Attempt

	found := false

	for _, attempt := range m.attempts {
		if attempt.OrganizationID == organizationID &&
			attempt.AssessmentID == assessmentID &&
			attempt.EmployeeID == employeeID &&
			(!found || attempt.SubmittedAt.After(latest.SubmittedAt)) {
			latest = attempt
			found = true
		}
	}

	if !found {
		return assessments.Attempt{}, assessments.ErrAttemptNotFound
	}

	return latest, nil
}

func (m *memoryRepo) UpdateAttempt(_ context.Context, attempt assessments.Attempt) error {
	existing, ok := m.attempts[attempt.ID]
	if !ok || existing.OrganizationID != attempt.OrganizationID {
		return assessments.ErrAttemptNotFound
	}

	m.attempts[attempt.ID] = attempt

	return nil
}

func (m *memoryRepo) CreateCertificate(_ context.Context, certificate assessments.Certificate) error {
	m.certificates[certificate.ID] = certificate

	return nil
}

func (m *memoryRepo) FindCertificate(
	_ context.Context,
	organizationID, assessmentID, employeeID string,
) (assessments.Certificate, error) {
	for _, certificate := range m.certificates {
		if certificate.OrganizationID == organizationID &&
			certificate.AssessmentID == assessmentID &&
			certificate.EmployeeID == employeeID {
			return certificate, nil
		}
	}

	return assessments.Certificate{}, assessments.ErrNotFound
}

func (m *memoryRepo) ListCertificatesForEmployee(
	_ context.Context,
	organizationID, employeeID string,
) ([]assessments.Certificate, error) {
	items := make([]assessments.Certificate, 0)

	for _, certificate := range m.certificates {
		if certificate.OrganizationID == organizationID && certificate.EmployeeID == employeeID {
			items = append(items, certificate)
		}
	}

	return items, nil
}

// fakeEmployeeReader resolves the fixed test employee.
type fakeEmployeeReader struct{}

func (fakeEmployeeReader) Get(
	_ context.Context,
	organizationID, employeeID string,
) (employees.Employee, error) {
	if employeeID != testEmployeeID {
		return employees.Employee{}, errors.New("employee not found")
	}

	return employees.Employee{
		ID:             employeeID,
		OrganizationID: organizationID,
		UserID:         testUserID,
		FirstName:      "Ada",
		LastName:       "Lovelace",
	}, nil
}

func (fakeEmployeeReader) GetByUserID(
	_ context.Context,
	organizationID, userID string,
) (employees.Employee, error) {
	if userID != testUserID {
		return employees.Employee{}, errors.New("employee not found")
	}

	return employees.Employee{
		ID:             testEmployeeID,
		OrganizationID: organizationID,
		UserID:         userID,
		FirstName:      "Ada",
		LastName:       "Lovelace",
	}, nil
}

func newService() (*assessments.Service, *memoryRepo) {
	repo := newMemoryRepo()

	return assessments.NewService(repo, fakeEmployeeReader{}), repo
}

// allTypeQuestions covers every question type, one point each.
func allTypeQuestions() []assessments.Question {
	return []assessments.Question{
		{
			ID:             "single",
			Type:           assessments.QuestionTypeSingleChoice,
			Text:           "Pick one",
			Options:        []string{"a", "b", "c"},
			CorrectOptions: []int{1},
		},
		{
			ID:             "multi",
			Type:           assessments.QuestionTypeMultipleChoice,
			Text:           "Pick two",
			Options:        []string{"a", "b", "c"},
			CorrectOptions: []int{0, 2},
		},
		{
			ID:             "tf",
			Type:           assessments.QuestionTypeTrueFalse,
			Text:           "True?",
			CorrectOptions: []int{0},
		},
		{
			ID:              "short",
			Type:            assessments.QuestionTypeShortAnswer,
			Text:            "Name it",
			AcceptedAnswers: []string{"Paris"},
		},
	}
}

func createPublished(
	t *testing.T,
	svc *assessments.Service,
	questions []assessments.Question,
	passingScore float64,
	maxAttempts int,
) assessments.Assessment {
	t.Helper()

	ctx := context.Background()

	assessment, err := svc.Create(ctx, testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title:        testTitle,
		Questions:    questions,
		PassingScore: passingScore,
		MaxAttempts:  maxAttempts,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	published, err := svc.Publish(ctx, testOrgID, assessment.ID)
	if err != nil {
		t.Fatalf("publish assessment: %v", err)
	}

	return published
}

func TestCreateValidatesQuestions(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if _, err := svc.Create(ctx, testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title:        testTitle,
		Questions:    nil,
		PassingScore: 70,
	}); !errors.Is(err, assessments.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty questions, got %v", err)
	}

	if _, err := svc.Create(ctx, testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title: testTitle,
		Questions: []assessments.Question{{
			ID:      "bad",
			Type:    assessments.QuestionTypeSingleChoice,
			Text:    "Pick",
			Options: []string{"only"},
		}},
		PassingScore: 70,
	}); !errors.Is(err, assessments.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for single-option choice, got %v", err)
	}

	if _, err := svc.Create(ctx, testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title:        testTitle,
		Questions:    allTypeQuestions(),
		PassingScore: 101,
	}); !errors.Is(err, assessments.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for passingScore > 100, got %v", err)
	}
}

func TestTrueFalseDefaultsOptions(t *testing.T) {
	svc, _ := newService()

	assessment, err := svc.Create(context.Background(), testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title: testTitle,
		Questions: []assessments.Question{{
			ID:             "tf",
			Type:           assessments.QuestionTypeTrueFalse,
			Text:           "True?",
			CorrectOptions: []int{1},
		}},
		PassingScore: 50,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	got := assessment.Questions[0].Options
	if len(got) != 2 || got[0] != "True" || got[1] != "False" {
		t.Fatalf("expected default True/False options, got %v", got)
	}
}

func TestGradingPerQuestionType(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 75, 0)
	ctx := context.Background()

	attempt, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{
			{QuestionID: "single", Options: []int{1}},
			{QuestionID: "multi", Options: []int{2, 0}}, // order must not matter
			{QuestionID: "tf", Options: []int{0}},
			{QuestionID: "short", Text: "  PARIS "}, // case/space-insensitive
		},
	})
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}

	if attempt.Score != 100 || !attempt.Passed || attempt.Status != assessments.AttemptGraded {
		t.Fatalf("expected 100%% graded pass, got score=%v passed=%v status=%s",
			attempt.Score, attempt.Passed, attempt.Status)
	}
}

func TestGradingPartialCreditByPoints(t *testing.T) {
	svc, _ := newService()

	questions := []assessments.Question{
		{
			ID:             "big",
			Type:           assessments.QuestionTypeSingleChoice,
			Text:           "Worth three",
			Options:        []string{"a", "b"},
			CorrectOptions: []int{0},
			Points:         3,
		},
		{
			ID:             "small",
			Type:           assessments.QuestionTypeSingleChoice,
			Text:           "Worth one",
			Options:        []string{"a", "b"},
			CorrectOptions: []int{0},
			Points:         1,
		},
	}

	assessment := createPublished(t, svc, questions, 50, 0)

	// Miss the 3-point question, hit the 1-point one: 25%.
	attempt, err := svc.SubmitAttempt(context.Background(), testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{
			{QuestionID: "big", Options: []int{1}},
			{QuestionID: "small", Options: []int{0}},
		},
	})
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}

	if attempt.Score != 25 || attempt.Passed {
		t.Fatalf("expected 25%% fail, got score=%v passed=%v", attempt.Score, attempt.Passed)
	}

	certificates, err := svc.ListMyCertificates(context.Background(), testOrgID, testUserID)
	if err != nil {
		t.Fatalf("list certificates: %v", err)
	}

	if len(certificates) != 0 {
		t.Fatalf("expected no certificate for a failed attempt, got %d", len(certificates))
	}
}

func TestMultipleChoiceRequiresExactSet(t *testing.T) {
	svc, _ := newService()

	questions := []assessments.Question{{
		ID:             "multi",
		Type:           assessments.QuestionTypeMultipleChoice,
		Text:           "Pick two",
		Options:        []string{"a", "b", "c"},
		CorrectOptions: []int{0, 2},
	}}

	assessment := createPublished(t, svc, questions, 50, 0)

	// A superset of the correct options must not score.
	attempt, err := svc.SubmitAttempt(context.Background(), testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{{QuestionID: "multi", Options: []int{0, 1, 2}}},
	})
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}

	if attempt.Score != 0 || attempt.Passed {
		t.Fatalf("expected 0%% for a superset answer, got score=%v passed=%v", attempt.Score, attempt.Passed)
	}
}

func TestShortAnswerMismatchRequiresReview(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 75, 0)
	ctx := context.Background()

	attempt, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{
			{QuestionID: "single", Options: []int{1}},
			{QuestionID: "multi", Options: []int{0, 2}},
			{QuestionID: "tf", Options: []int{0}},
			{QuestionID: "short", Text: "London"},
		},
	})
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}

	if attempt.Status != assessments.AttemptPendingReview || attempt.Passed {
		t.Fatalf("expected pending_review, got status=%s passed=%v", attempt.Status, attempt.Passed)
	}

	// Auto-graded questions still score: 3 of 4 points.
	if attempt.Score != 75 {
		t.Fatalf("expected auto score 75, got %v", attempt.Score)
	}

	// The journey completion check must not count a pending attempt.
	passed, err := svc.LatestAttemptPassed(ctx, testOrgID, assessment.ID, testEmployeeID)
	if err != nil {
		t.Fatalf("latest attempt passed: %v", err)
	}

	if passed {
		t.Fatal("pending_review attempt must not count as passed")
	}

	// Manager finalizes with a score above the passing threshold.
	reviewed, err := svc.ReviewAttempt(ctx, testOrgID, testManagerID, assessment.ID, attempt.ID, assessments.ReviewAttemptInput{
		Score: 100,
		Note:  "Accepted London after discussion",
	})
	if err != nil {
		t.Fatalf("review attempt: %v", err)
	}

	if reviewed.Status != assessments.AttemptGraded || !reviewed.Passed ||
		reviewed.Score != 100 || reviewed.ReviewedBy != testManagerID {
		t.Fatalf("unexpected reviewed attempt: %+v", reviewed)
	}

	certificates, err := svc.ListMyCertificates(ctx, testOrgID, testUserID)
	if err != nil {
		t.Fatalf("list certificates: %v", err)
	}

	if len(certificates) != 1 || certificates[0].AssessmentTitle != testTitle ||
		certificates[0].EmployeeName != "Ada Lovelace" || certificates[0].Serial == "" {
		t.Fatalf("expected one certificate after reviewed pass, got %+v", certificates)
	}

	// Reviewing an already-graded attempt is rejected.
	if _, err := svc.ReviewAttempt(ctx, testOrgID, testManagerID, assessment.ID, attempt.ID, assessments.ReviewAttemptInput{
		Score: 0,
	}); !errors.Is(err, assessments.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState reviewing a graded attempt, got %v", err)
	}
}

func TestMaxAttemptsEnforced(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 100, 2)
	ctx := context.Background()

	for i := range 2 {
		_, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
			Answers: []assessments.Answer{{QuestionID: "single", Options: []int{0}}},
		})
		if err != nil {
			t.Fatalf("submit attempt %d: %v", i+1, err)
		}
	}

	if _, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{{QuestionID: "single", Options: []int{0}}},
	}); !errors.Is(err, assessments.ErrAttemptsExhausted) {
		t.Fatalf("expected ErrAttemptsExhausted, got %v", err)
	}

	view, err := svc.Take(ctx, testOrgID, testUserID, assessment.ID)
	if err != nil {
		t.Fatalf("take: %v", err)
	}

	if view.AttemptsUsed != 2 || view.AttemptsRemaining != 0 {
		t.Fatalf("expected 2 used / 0 remaining, got %d / %d", view.AttemptsUsed, view.AttemptsRemaining)
	}
}

func TestUnlimitedAttemptsReportMinusOne(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 100, 0)

	view, err := svc.Take(context.Background(), testOrgID, testUserID, assessment.ID)
	if err != nil {
		t.Fatalf("take: %v", err)
	}

	if view.AttemptsRemaining != -1 {
		t.Fatalf("expected -1 for unlimited attempts, got %d", view.AttemptsRemaining)
	}
}

func TestTakeHidesAnswerKeys(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 75, 0)

	view, err := svc.Take(context.Background(), testOrgID, testUserID, assessment.ID)
	if err != nil {
		t.Fatalf("take: %v", err)
	}

	if len(view.Questions) != len(assessment.Questions) {
		t.Fatalf("expected %d questions, got %d", len(assessment.Questions), len(view.Questions))
	}

	// QuestionView has no answer-key fields at all; verify ids and options
	// survive so employees can render the assessment.
	ids := map[string]bool{}
	for _, question := range view.Questions {
		ids[question.ID] = true
	}

	for _, id := range []string{"single", "multi", "tf", "short"} {
		if !ids[id] {
			t.Fatalf("take view is missing question %s", id)
		}
	}
}

func TestRandomizeShufflesQuestionOrder(t *testing.T) {
	repo := newMemoryRepo()
	svc := assessments.NewService(repo, fakeEmployeeReader{})
	svc.SetShuffle(func(n int, swap func(i, j int)) {
		// Deterministic reversal for the test.
		for i := range n / 2 {
			swap(i, n-1-i)
		}
	})

	ctx := context.Background()

	created, err := svc.Create(ctx, testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title:        testTitle,
		Questions:    allTypeQuestions(),
		PassingScore: 75,
		Randomize:    true,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	assessment, err := svc.Publish(ctx, testOrgID, created.ID)
	if err != nil {
		t.Fatalf("publish assessment: %v", err)
	}

	view, err := svc.Take(ctx, testOrgID, testUserID, assessment.ID)
	if err != nil {
		t.Fatalf("take: %v", err)
	}

	if view.Questions[0].ID != "short" || view.Questions[3].ID != "single" {
		t.Fatalf("expected shuffled order, got %v", view.Questions)
	}
}

func TestCertificateIssuedOncePerAssessment(t *testing.T) {
	svc, _ := newService()

	questions := []assessments.Question{{
		ID:             "single",
		Type:           assessments.QuestionTypeSingleChoice,
		Text:           "Pick one",
		Options:        []string{"a", "b"},
		CorrectOptions: []int{0},
	}}

	assessment := createPublished(t, svc, questions, 50, 0)
	ctx := context.Background()

	for range 2 {
		attempt, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
			Answers: []assessments.Answer{{QuestionID: "single", Options: []int{0}}},
		})
		if err != nil {
			t.Fatalf("submit attempt: %v", err)
		}

		if !attempt.Passed {
			t.Fatal("expected pass")
		}
	}

	certificates, err := svc.ListMyCertificates(ctx, testOrgID, testUserID)
	if err != nil {
		t.Fatalf("list certificates: %v", err)
	}

	if len(certificates) != 1 {
		t.Fatalf("expected exactly one certificate across passes, got %d", len(certificates))
	}
}

func TestLatestAttemptPassedFollowsNewestAttempt(t *testing.T) {
	svc, repo := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 100, 0)
	ctx := context.Background()

	// Fail the first attempt.
	if _, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{{QuestionID: "single", Options: []int{0}}},
	}); err != nil {
		t.Fatalf("submit failing attempt: %v", err)
	}

	// Backdate it, then submit a passing attempt so "latest" is exercised.
	for id, attempt := range repo.attempts {
		attempt.SubmittedAt = time.Now().Add(-time.Hour)
		repo.attempts[id] = attempt
	}

	if _, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{
			{QuestionID: "single", Options: []int{1}},
			{QuestionID: "multi", Options: []int{0, 2}},
			{QuestionID: "tf", Options: []int{0}},
			{QuestionID: "short", Text: "paris"},
		},
	}); err != nil {
		t.Fatalf("submit passing attempt: %v", err)
	}

	passed, err := svc.LatestAttemptPassed(ctx, testOrgID, assessment.ID, testEmployeeID)
	if err != nil {
		t.Fatalf("latest attempt passed: %v", err)
	}

	if !passed {
		t.Fatal("expected latest passing attempt to count")
	}
}

func TestTenantIsolation(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 75, 0)
	ctx := context.Background()

	if _, err := svc.Get(ctx, testOtherOrgID, assessment.ID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("expected ErrNotFound across tenants, got %v", err)
	}

	if _, err := svc.Take(ctx, testOtherOrgID, testUserID, assessment.ID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("expected ErrNotFound taking across tenants, got %v", err)
	}

	if _, err := svc.SubmitAttempt(ctx, testOtherOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{{QuestionID: "single", Options: []int{1}}},
	}); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("expected ErrNotFound submitting across tenants, got %v", err)
	}

	attempt, err := svc.SubmitAttempt(ctx, testOrgID, testUserID, assessment.ID, assessments.SubmitAttemptInput{
		Answers: []assessments.Answer{{QuestionID: "short", Text: "nope"}},
	})
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}

	if _, err := svc.ReviewAttempt(ctx, testOtherOrgID, testManagerID, assessment.ID, attempt.ID, assessments.ReviewAttemptInput{
		Score: 100,
	}); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("expected ErrNotFound reviewing across tenants, got %v", err)
	}

	certificates, err := svc.ListMyCertificates(ctx, testOtherOrgID, testUserID)
	if err != nil {
		t.Fatalf("list certificates: %v", err)
	}

	if len(certificates) != 0 {
		t.Fatalf("expected no certificates for another tenant, got %d", len(certificates))
	}
}

func TestDraftAssessmentCannotBeTaken(t *testing.T) {
	svc, _ := newService()

	assessment, err := svc.Create(context.Background(), testOrgID, createdByUserID, assessments.CreateAssessmentInput{
		Title:        testTitle,
		Questions:    allTypeQuestions(),
		PassingScore: 75,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	if _, err := svc.Take(context.Background(), testOrgID, testUserID, assessment.ID); !errors.Is(err, assessments.ErrNotPublished) {
		t.Fatalf("expected ErrNotPublished for a draft, got %v", err)
	}
}

func TestUpdateOnlyInDraft(t *testing.T) {
	svc, _ := newService()
	assessment := createPublished(t, svc, allTypeQuestions(), 75, 0)

	newTitle := "Updated"
	if _, err := svc.Update(context.Background(), testOrgID, assessment.ID, assessments.UpdateAssessmentInput{
		Title: &newTitle,
	}); !errors.Is(err, assessments.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState updating a published assessment, got %v", err)
	}
}
