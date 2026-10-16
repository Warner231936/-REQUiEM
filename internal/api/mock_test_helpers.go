package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

var (
	errMockNotFound  = errors.New("not found")
	errMockDuplicate = errors.New("duplicate key")
)

type mockMemoryStore struct{}

func (m *mockMemoryStore) Create(ctx context.Context, mem models.Memory) error { return nil }
func (m *mockMemoryStore) Get(ctx context.Context, id string) (models.Memory, error) {
	return models.Memory{}, errMockNotFound
}
func (m *mockMemoryStore) Search(ctx context.Context, q models.MemoryQuery) ([]models.MemorySearchResult, error) {
	return nil, nil
}
func (m *mockMemoryStore) Update(ctx context.Context, mem models.Memory) error { return nil }
func (m *mockMemoryStore) Delete(ctx context.Context, id string) error         { return nil }
func (m *mockMemoryStore) GetByConversation(ctx context.Context, convID string, limit int) ([]models.Memory, error) {
	return nil, nil
}

type mockMindStore struct{}

func (m *mockMindStore) GetBeliefs(ctx context.Context, q models.MindQuery) ([]models.Belief, error) {
	return nil, nil
}
func (m *mockMindStore) CreateBelief(ctx context.Context, b models.Belief) error { return nil }
func (m *mockMindStore) UpdateBelief(ctx context.Context, b models.Belief) error { return nil }
func (m *mockMindStore) GetOpinions(ctx context.Context, q models.MindQuery) ([]models.Opinion, error) {
	return nil, nil
}
func (m *mockMindStore) CreateOpinion(ctx context.Context, o models.Opinion) error { return nil }
func (m *mockMindStore) UpdateOpinion(ctx context.Context, o models.Opinion) error { return nil }
func (m *mockMindStore) GetPreferences(ctx context.Context, q models.MindQuery) ([]models.Preference, error) {
	return nil, nil
}
func (m *mockMindStore) CreatePreference(ctx context.Context, p models.Preference) error { return nil }
func (m *mockMindStore) UpdatePreference(ctx context.Context, p models.Preference) error { return nil }
func (m *mockMindStore) GetValues(ctx context.Context, q models.MindQuery) ([]models.Value, error) {
	return nil, nil
}
func (m *mockMindStore) CreateValue(ctx context.Context, v models.Value) error { return nil }
func (m *mockMindStore) GetGoals(ctx context.Context, q models.MindQuery) ([]models.Goal, error) {
	return nil, nil
}
func (m *mockMindStore) CreateGoal(ctx context.Context, g models.Goal) error { return nil }
func (m *mockMindStore) UpdateGoal(ctx context.Context, g models.Goal) error { return nil }
func (m *mockMindStore) GetHypotheses(ctx context.Context, q models.MindQuery) ([]models.Hypothesis, error) {
	return nil, nil
}
func (m *mockMindStore) CreateHypothesis(ctx context.Context, h models.Hypothesis) error { return nil }
func (m *mockMindStore) UpdateHypothesis(ctx context.Context, h models.Hypothesis) error { return nil }
func (m *mockMindStore) GetUncertainties(ctx context.Context, q models.MindQuery) ([]models.Uncertainty, error) {
	return nil, nil
}
func (m *mockMindStore) CreateUncertainty(ctx context.Context, u models.Uncertainty) error {
	return nil
}
func (m *mockMindStore) UpdateUncertainty(ctx context.Context, u models.Uncertainty) error {
	return nil
}
func (m *mockMindStore) GetSelfObservations(ctx context.Context, q models.MindQuery) ([]models.SelfObservation, error) {
	return nil, nil
}
func (m *mockMindStore) CreateSelfObservation(ctx context.Context, so models.SelfObservation) error {
	return nil
}
func (m *mockMindStore) UpdateSelfObservation(ctx context.Context, so models.SelfObservation) error {
	return nil
}
func (m *mockMindStore) GetContradictions(ctx context.Context, q models.MindQuery) ([]models.Contradiction, error) {
	return nil, nil
}
func (m *mockMindStore) CreateContradiction(ctx context.Context, c models.Contradiction) error {
	return nil
}
func (m *mockMindStore) UpdateContradiction(ctx context.Context, c models.Contradiction) error {
	return nil
}

type mockConversationStore struct{}

func (m *mockConversationStore) CreateConversation(ctx context.Context, msg models.ConversationMessage) error {
	return nil
}
func (m *mockConversationStore) AddMessage(ctx context.Context, convID string, msg models.ConversationMessage) error {
	return nil
}
func (m *mockConversationStore) GetMessages(ctx context.Context, convID string, limit int) ([]models.ConversationMessage, error) {
	return nil, nil
}
func (m *mockConversationStore) GetMessagesPaginated(ctx context.Context, convID string, limit, offset int) ([]models.ConversationMessage, error) {
	return nil, nil
}
func (m *mockConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]storage.ConversationSummary, error) {
	return nil, nil
}
func (m *mockConversationStore) DeleteAll(ctx context.Context, convID string) (int64, error) {
	return 0, nil
}

type mockEventStore struct{}

func (m *mockEventStore) RecordEvent(ctx context.Context, event models.PipelineEvent) error {
	return nil
}
func (m *mockEventStore) GetEvents(ctx context.Context, runID string) ([]models.PipelineEvent, error) {
	return nil, nil
}

type mockAuditStore struct{}

func (m *mockAuditStore) RecordHistory(ctx context.Context, record storage.AuditRecord) error {
	return nil
}
func (m *mockAuditStore) GetHistory(ctx context.Context, entityType, entityID string) ([]storage.AuditRecord, error) {
	return nil, nil
}
func (m *mockAuditStore) GetProvenance(ctx context.Context, entityID string) ([]storage.ProvenanceStep, error) {
	return nil, nil
}

type mockPipelineRunStore struct{}

func (m *mockPipelineRunStore) RecordRun(ctx context.Context, run models.PipelineRun) error {
	return nil
}
func (m *mockPipelineRunStore) GetRun(ctx context.Context, id string) (models.PipelineRun, error) {
	return models.PipelineRun{}, errMockNotFound
}

type mockAuthStore struct {
	mu    sync.Mutex
	users map[string]models.User
}

func newMockAuthStore() *mockAuthStore {
	return &mockAuthStore{
		users: make(map[string]models.User),
	}
}

func (s *mockAuthStore) CreateUser(ctx context.Context, user models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[user.Email]; exists {
		return errMockDuplicate
	}
	s.users[user.Email] = user
	return nil
}

func (s *mockAuthStore) GetUser(ctx context.Context, userID string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return models.User{}, errMockNotFound
}

func (s *mockAuthStore) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[email]; ok {
		return u, nil
	}
	return models.User{}, errMockNotFound
}

func (s *mockAuthStore) ListUsers(ctx context.Context) ([]models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	users := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users, nil
}

func (s *mockAuthStore) UpdateUser(ctx context.Context, user models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[user.Email]; ok {
		u.IsVerified = user.IsVerified
		u.PasswordHash = user.PasswordHash
		u.Name = user.Name
		u.IsAdmin = user.IsAdmin
		u.UpdatedAt = time.Now().UTC()
		s.users[user.Email] = u
		return nil
	}
	return errMockNotFound
}

func (s *mockAuthStore) DeleteUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for email, u := range s.users {
		if u.ID == userID {
			delete(s.users, email)
			return nil
		}
	}
	return errMockNotFound
}

func (s *mockAuthStore) CreateVerificationToken(ctx context.Context, token models.VerificationToken) error {
	return nil
}
func (s *mockAuthStore) GetVerificationToken(ctx context.Context, token string) (models.VerificationToken, error) {
	return models.VerificationToken{}, errMockNotFound
}
func (s *mockAuthStore) DeleteVerificationToken(ctx context.Context, token string) error { return nil }
func (s *mockAuthStore) CreatePasswordResetToken(ctx context.Context, token models.PasswordResetToken) error {
	return nil
}
func (s *mockAuthStore) GetPasswordResetToken(ctx context.Context, token string) (models.PasswordResetToken, error) {
	return models.PasswordResetToken{}, errMockNotFound
}
func (s *mockAuthStore) DeletePasswordResetToken(ctx context.Context, token string) error { return nil }

type mockStore struct {
	memStore   *mockMemoryStore
	mindStore  *mockMindStore
	convStore  *mockConversationStore
	eventStore *mockEventStore
	auditStore *mockAuditStore
	runStore   *mockPipelineRunStore
	authStore  *mockAuthStore
}

func newMockStore() *mockStore {
	return &mockStore{
		memStore:   &mockMemoryStore{},
		mindStore:  &mockMindStore{},
		convStore:  &mockConversationStore{},
		eventStore: &mockEventStore{},
		auditStore: &mockAuditStore{},
		runStore:   &mockPipelineRunStore{},
		authStore:  newMockAuthStore(),
	}
}

func (s *mockStore) Memory() storage.MemoryStore                                   { return s.memStore }
func (s *mockStore) Mind() storage.MindStore                                       { return s.mindStore }
func (s *mockStore) Conversation() storage.ConversationStore                       { return s.convStore }
func (s *mockStore) Event() storage.EventStore                                     { return s.eventStore }
func (s *mockStore) Audit() storage.AuditStore                                     { return s.auditStore }
func (s *mockStore) PipelineRun() storage.PipelineRunStore                         { return s.runStore }
func (s *mockStore) Auth() storage.AuthStore                                       { return s.authStore }
func (s *mockStore) Evidence() storage.EvidenceStore                               { return &mockEvidenceStore{} }
func (s *mockStore) Claim() storage.ClaimStore                                     { return &mockClaimStore{} }
func (s *mockStore) Verification() storage.VerificationStore                        { return &mockVerificationStore{} }
func (s *mockStore) StartTransaction(ctx context.Context) (context.Context, error) { return ctx, nil }
func (s *mockStore) CommitTransaction(ctx context.Context) error                   { return nil }
func (s *mockStore) AbortTransaction(ctx context.Context) error                    { return nil }
func (s *mockStore) WithRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockEvidenceStore struct{}

func (m *mockEvidenceStore) CreateRecord(ctx context.Context, record models.EvidenceRecord) error {
	return nil
}

func (m *mockEvidenceStore) GetByClaim(ctx context.Context, claimID string) ([]models.EvidenceRecord, error) {
	return nil, nil
}

func (m *mockEvidenceStore) GetByID(ctx context.Context, id string) (models.EvidenceRecord, error) {
	return models.EvidenceRecord{}, nil
}

func (m *mockEvidenceStore) UpdateValidity(ctx context.Context, id string, validity models.ValidityStatus) error {
	return nil
}

func (m *mockEvidenceStore) Delete(ctx context.Context, id string) error {
	return nil
}

type mockClaimStore struct{}

func (m *mockClaimStore) CreateClaim(ctx context.Context, claim models.Claim) error {
	return nil
}

func (m *mockClaimStore) GetClaim(ctx context.Context, id string) (models.Claim, error) {
	return models.Claim{}, nil
}

func (m *mockClaimStore) UpdateClaim(ctx context.Context, claim models.Claim) error {
	return nil
}

func (m *mockClaimStore) GetBySubject(ctx context.Context, subject string, limit int) ([]models.Claim, error) {
	return nil, nil
}

func (m *mockClaimStore) UpdateStatus(ctx context.Context, id string, status models.ClaimStatus, confidence models.Confidence) error {
	return nil
}

type mockVerificationStore struct{}

func (m *mockVerificationStore) RecordPlan(ctx context.Context, plan models.VerificationPlan) error {
	return nil
}

func (m *mockVerificationStore) RecordResult(ctx context.Context, result models.VerificationResult) error {
	return nil
}

func (m *mockVerificationStore) GetPlan(ctx context.Context, claimID string) (models.VerificationPlan, error) {
	return models.VerificationPlan{}, nil
}

func (m *mockVerificationStore) GetResult(ctx context.Context, claimID string) (models.VerificationResult, error) {
	return models.VerificationResult{}, nil
}
