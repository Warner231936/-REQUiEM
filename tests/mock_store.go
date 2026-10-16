package pipeline_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type MockMemoryStore struct {
	mu       sync.Mutex
	memories map[string]models.Memory
}

func NewMockMemoryStore() *MockMemoryStore {
	return &MockMemoryStore{
		memories: make(map[string]models.Memory),
	}
}

func (m *MockMemoryStore) Create(ctx context.Context, memory models.Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories[memory.ID] = memory
	return nil
}

func (m *MockMemoryStore) Get(ctx context.Context, id string) (models.Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.memories[id]
	if !ok {
		return models.Memory{}, models.NotFoundError{Resource: "memory", ID: id}
	}
	return mem, nil
}

func (m *MockMemoryStore) Search(ctx context.Context, query models.MemoryQuery) ([]models.MemorySearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []models.MemorySearchResult
	for _, mem := range m.memories {
		if query.UserID != "" && mem.UserID != query.UserID {
			continue
		}
		if query.Type != "" && mem.Type != query.Type {
			continue
		}
		if len(query.Tags) > 0 {
			matched := false
			for _, tag := range query.Tags {
				for _, memTag := range mem.Tags {
					if memTag == tag {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, models.MemorySearchResult{
			Memory:     mem,
			Similarity: mem.CalculateEffectiveRelevance(time.Now().UTC()),
			Relevance:  mem.CalculateEffectiveRelevance(time.Now().UTC()),
		})
	}
	return results, nil
}

func (m *MockMemoryStore) Update(ctx context.Context, memory models.Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.memories[memory.ID]; !ok {
		return models.NotFoundError{Resource: "memory", ID: memory.ID}
	}
	m.memories[memory.ID] = memory
	return nil
}

func (m *MockMemoryStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.memories[id]; !ok {
		return models.NotFoundError{Resource: "memory", ID: id}
	}
	delete(m.memories, id)
	return nil
}

func (m *MockMemoryStore) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []models.Memory
	for _, mem := range m.memories {
		if mem.ConversationID == conversationID {
			results = append(results, mem)
		}
	}
	return results, nil
}

type MockMindStore struct {
	mu               sync.Mutex
	beliefs          map[string]models.Belief
	opinions         map[string]models.Opinion
	prefs            map[string]models.Preference
	values           map[string]models.Value
	goals            map[string]models.Goal
	hypotheses       map[string]models.Hypothesis
	uncertainties    map[string]models.Uncertainty
	selfObs          map[string]models.SelfObservation
	contradictions   map[string]models.Contradiction
	CreateBeliefFunc func(ctx context.Context, belief models.Belief) error
}

func NewMockMindStore() *MockMindStore {
	return &MockMindStore{
		beliefs:        make(map[string]models.Belief),
		opinions:       make(map[string]models.Opinion),
		prefs:          make(map[string]models.Preference),
		values:         make(map[string]models.Value),
		goals:          make(map[string]models.Goal),
		hypotheses:     make(map[string]models.Hypothesis),
		uncertainties:  make(map[string]models.Uncertainty),
		selfObs:        make(map[string]models.SelfObservation),
		contradictions: make(map[string]models.Contradiction),
	}
}

func (m *MockMindStore) GetBeliefs(ctx context.Context, query models.MindQuery) ([]models.Belief, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Belief
	for _, b := range m.beliefs {
		if query.MinConfidence > 0 && float64(b.Confidence) < float64(query.MinConfidence) {
			continue
		}
		if len(query.Subjects) > 0 {
			matched := false
			for _, subj := range query.Subjects {
				if b.Subject == subj {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, b)
	}
	return results, nil
}

func (m *MockMindStore) CreateBelief(ctx context.Context, belief models.Belief) error {
	if m.CreateBeliefFunc != nil {
		return m.CreateBeliefFunc(ctx, belief)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beliefs[belief.ID] = belief
	return nil
}

func (m *MockMindStore) UpdateBelief(ctx context.Context, belief models.Belief) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.beliefs[belief.ID]; !ok {
		return models.NotFoundError{Resource: "belief", ID: belief.ID}
	}
	m.beliefs[belief.ID] = belief
	return nil
}

func (m *MockMindStore) GetOpinions(ctx context.Context, query models.MindQuery) ([]models.Opinion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Opinion
	for _, o := range m.opinions {
		results = append(results, o)
	}
	return results, nil
}

func (m *MockMindStore) CreateOpinion(ctx context.Context, opinion models.Opinion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opinions[opinion.ID] = opinion
	return nil
}

func (m *MockMindStore) UpdateOpinion(ctx context.Context, opinion models.Opinion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.opinions[opinion.ID]; !ok {
		return models.NotFoundError{Resource: "opinion", ID: opinion.ID}
	}
	m.opinions[opinion.ID] = opinion
	return nil
}

func (m *MockMindStore) GetPreferences(ctx context.Context, query models.MindQuery) ([]models.Preference, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Preference
	for _, p := range m.prefs {
		results = append(results, p)
	}
	return results, nil
}

func (m *MockMindStore) CreatePreference(ctx context.Context, preference models.Preference) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prefs[preference.ID] = preference
	return nil
}

func (m *MockMindStore) UpdatePreference(ctx context.Context, preference models.Preference) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.prefs[preference.ID]; !ok {
		return models.NotFoundError{Resource: "preference", ID: preference.ID}
	}
	m.prefs[preference.ID] = preference
	return nil
}

func (m *MockMindStore) GetValues(ctx context.Context, query models.MindQuery) ([]models.Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Value
	for _, v := range m.values {
		results = append(results, v)
	}
	return results, nil
}

func (m *MockMindStore) CreateValue(ctx context.Context, value models.Value) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[value.ID] = value
	return nil
}

func (m *MockMindStore) GetGoals(ctx context.Context, query models.MindQuery) ([]models.Goal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Goal
	for _, g := range m.goals {
		results = append(results, g)
	}
	return results, nil
}

func (m *MockMindStore) CreateGoal(ctx context.Context, goal models.Goal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.goals[goal.ID] = goal
	return nil
}

func (m *MockMindStore) UpdateGoal(ctx context.Context, goal models.Goal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.goals[goal.ID]; !ok {
		return models.NotFoundError{Resource: "goal", ID: goal.ID}
	}
	m.goals[goal.ID] = goal
	return nil
}

func (m *MockMindStore) GetHypotheses(ctx context.Context, query models.MindQuery) ([]models.Hypothesis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Hypothesis
	for _, h := range m.hypotheses {
		results = append(results, h)
	}
	return results, nil
}

func (m *MockMindStore) CreateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hypotheses[hypothesis.ID] = hypothesis
	return nil
}

func (m *MockMindStore) UpdateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hypotheses[hypothesis.ID]; !ok {
		return models.NotFoundError{Resource: "hypothesis", ID: hypothesis.ID}
	}
	m.hypotheses[hypothesis.ID] = hypothesis
	return nil
}

func (m *MockMindStore) GetUncertainties(ctx context.Context, query models.MindQuery) ([]models.Uncertainty, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Uncertainty
	for _, u := range m.uncertainties {
		results = append(results, u)
	}
	return results, nil
}

func (m *MockMindStore) CreateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uncertainties[uncertainty.ID] = uncertainty
	return nil
}

func (m *MockMindStore) UpdateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.uncertainties[uncertainty.ID]; !ok {
		return models.NotFoundError{Resource: "uncertainty", ID: uncertainty.ID}
	}
	m.uncertainties[uncertainty.ID] = uncertainty
	return nil
}

func (m *MockMindStore) GetSelfObservations(ctx context.Context, query models.MindQuery) ([]models.SelfObservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.SelfObservation
	for _, so := range m.selfObs {
		results = append(results, so)
	}
	return results, nil
}

func (m *MockMindStore) CreateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selfObs[observation.ID] = observation
	return nil
}

func (m *MockMindStore) UpdateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.selfObs[observation.ID]; !ok {
		return models.NotFoundError{Resource: "self_observation", ID: observation.ID}
	}
	m.selfObs[observation.ID] = observation
	return nil
}

func (m *MockMindStore) GetContradictions(ctx context.Context, query models.MindQuery) ([]models.Contradiction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Contradiction
	for _, c := range m.contradictions {
		results = append(results, c)
	}
	return results, nil
}

func (m *MockMindStore) CreateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contradictions[contradiction.ID] = contradiction
	return nil
}

func (m *MockMindStore) UpdateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.contradictions[contradiction.ID]; !ok {
		return models.NotFoundError{Resource: "contradiction", ID: contradiction.ID}
	}
	m.contradictions[contradiction.ID] = contradiction
	return nil
}

type MockConversationStore struct {
	mu            sync.Mutex
	messages      map[string][]models.ConversationMessage
	conversations map[string]struct{}
}

func NewMockConversationStore() *MockConversationStore {
	return &MockConversationStore{
		messages:      make(map[string][]models.ConversationMessage),
		conversations: make(map[string]struct{}),
	}
}

func (m *MockConversationStore) CreateConversation(ctx context.Context, message models.ConversationMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversations[message.ConversationID] = struct{}{}
	m.messages[message.ConversationID] = []models.ConversationMessage{message}
	return nil
}

func (m *MockConversationStore) AddMessage(ctx context.Context, conversationID string, message models.ConversationMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[conversationID] = append(m.messages[conversationID], message)
	return nil
}

func (m *MockConversationStore) GetMessages(ctx context.Context, conversationID string, limit int) ([]models.ConversationMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := m.messages[conversationID]
	if limit > 0 && limit < len(msgs) {
		return msgs[:limit], nil
	}
	return msgs, nil
}

func (m *MockConversationStore) GetMessagesPaginated(ctx context.Context, conversationID string, limit, offset int) ([]models.ConversationMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := m.messages[conversationID]
	if offset >= len(msgs) {
		return []models.ConversationMessage{}, nil
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	return msgs[offset:end], nil
}

func (m *MockConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]storage.ConversationSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []storage.ConversationSummary
	for id := range m.conversations {
		result = append(result, storage.ConversationSummary{ConversationID: id})
	}
	return result, nil
}

func (m *MockConversationStore) DeleteAll(ctx context.Context, conversationID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := int64(len(m.messages[conversationID]))
	delete(m.messages, conversationID)
	delete(m.conversations, conversationID)
	return count, nil
}

type MockEventStore struct{}

func (m *MockEventStore) RecordEvent(ctx context.Context, event models.PipelineEvent) error {
	return nil
}

func (m *MockEventStore) GetEvents(ctx context.Context, pipelineRunID string) ([]models.PipelineEvent, error) {
	return []models.PipelineEvent{}, nil
}

type MockAuditStore struct {
	mu      sync.Mutex
	history []storage.AuditRecord
}

func NewMockAuditStore() *MockAuditStore {
	return &MockAuditStore{
		history: []storage.AuditRecord{},
	}
}

func (m *MockAuditStore) RecordHistory(ctx context.Context, record storage.AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, record)
	return nil
}

func (m *MockAuditStore) GetHistory(ctx context.Context, entityType, entityID string) ([]storage.AuditRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []storage.AuditRecord
	for _, r := range m.history {
		if entityID != "" && r.EntityID != entityID {
			continue
		}
		if entityType != "" && r.EntityType != entityType {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (m *MockAuditStore) GetProvenance(ctx context.Context, entityID string) ([]storage.ProvenanceStep, error) {
	return []storage.ProvenanceStep{}, nil
}

type MockPipelineRunStore struct{}

func (m *MockPipelineRunStore) RecordRun(ctx context.Context, run models.PipelineRun) error {
	return nil
}

func (m *MockPipelineRunStore) GetRun(ctx context.Context, id string) (models.PipelineRun, error) {
	return models.PipelineRun{}, nil
}

type MockAuthStore struct {
	users map[string]models.User
}

func NewMockAuthStore() *MockAuthStore {
	return &MockAuthStore{
		users: make(map[string]models.User),
	}
}

func (s *MockAuthStore) CreateUser(ctx context.Context, user models.User) error {
	s.users[user.Email] = user
	return nil
}

func (s *MockAuthStore) GetUser(ctx context.Context, userID string) (models.User, error) {
	for _, u := range s.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return models.User{}, fmt.Errorf("user not found")
}

func (s *MockAuthStore) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	if u, ok := s.users[email]; ok {
		return u, nil
	}
	return models.User{}, fmt.Errorf("user not found")
}

func (s *MockAuthStore) UpdateUser(ctx context.Context, user models.User) error {
	if u, ok := s.users[user.Email]; ok {
		u.IsVerified = user.IsVerified
		u.PasswordHash = user.PasswordHash
		u.Name = user.Name
		s.users[user.Email] = u
		return nil
	}
	return fmt.Errorf("user not found")
}

func (s *MockAuthStore) DeleteUser(ctx context.Context, userID string) error {
	for email, u := range s.users {
		if u.ID == userID {
			delete(s.users, email)
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (s *MockAuthStore) CreateVerificationToken(ctx context.Context, token models.VerificationToken) error {
	return nil
}

func (s *MockAuthStore) GetVerificationToken(ctx context.Context, token string) (models.VerificationToken, error) {
	return models.VerificationToken{}, fmt.Errorf("token not found")
}

func (s *MockAuthStore) DeleteVerificationToken(ctx context.Context, token string) error {
	return nil
}

func (s *MockAuthStore) CreatePasswordResetToken(ctx context.Context, token models.PasswordResetToken) error {
	return nil
}

func (s *MockAuthStore) GetPasswordResetToken(ctx context.Context, token string) (models.PasswordResetToken, error) {
	return models.PasswordResetToken{}, fmt.Errorf("token not found")
}

func (s *MockAuthStore) DeletePasswordResetToken(ctx context.Context, token string) error {
	return nil
}

func (s *MockAuthStore) ListUsers(ctx context.Context) ([]models.User, error) {
	users := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users, nil
}

var _ storage.AuthStore = (*MockAuthStore)(nil)

type MockStore struct {
	memStore      *MockMemoryStore
	mindStore     *MockMindStore
	convStore     *MockConversationStore
	eventStore    *MockEventStore
	auditStore    *MockAuditStore
	runStore      *MockPipelineRunStore
	authStore     *MockAuthStore
	evidenceStore *MockEvidenceStore
	claimStore    *MockClaimStore
	verifStore    *MockVerificationStore
	WithRetryFunc func(ctx context.Context, fn func(ctx context.Context) error) error
}

func NewMockStore() *MockStore {
	return &MockStore{
		memStore:   NewMockMemoryStore(),
		mindStore:  NewMockMindStore(),
		convStore:  NewMockConversationStore(),
		eventStore: &MockEventStore{},
		auditStore: NewMockAuditStore(),
		runStore:   &MockPipelineRunStore{},
		authStore:  NewMockAuthStore(),
		evidenceStore: &MockEvidenceStore{records: []models.EvidenceRecord{}},
		claimStore:    &MockClaimStore{claims: []models.Claim{}},
		verifStore:    &MockVerificationStore{},
	}
}

func (s *MockStore) Memory() storage.MemoryStore {
	return s.memStore
}

func (s *MockStore) Mind() storage.MindStore {
	return s.mindStore
}

func (s *MockStore) Conversation() storage.ConversationStore {
	return s.convStore
}

func (s *MockStore) Event() storage.EventStore {
	return s.eventStore
}

func (s *MockStore) Audit() storage.AuditStore {
	return s.auditStore
}

func (s *MockStore) PipelineRun() storage.PipelineRunStore {
	return s.runStore
}

func (s *MockStore) Auth() storage.AuthStore {
	return s.authStore
}

func (s *MockStore) Evidence() storage.EvidenceStore {
	return s.evidenceStore
}

func (s *MockStore) Claim() storage.ClaimStore {
	return s.claimStore
}

func (s *MockStore) Verification() storage.VerificationStore {
	return s.verifStore
}

func (s *MockStore) StartTransaction(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (s *MockStore) CommitTransaction(ctx context.Context) error {
	return nil
}

func (s *MockStore) AbortTransaction(ctx context.Context) error {
	return nil
}

func (s *MockStore) WithRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.WithRetryFunc != nil {
		return s.WithRetryFunc(ctx, fn)
	}
	return fn(ctx)
}

type MockEvidenceStore struct {
	records []models.EvidenceRecord
}

func (m *MockEvidenceStore) CreateRecord(ctx context.Context, record models.EvidenceRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *MockEvidenceStore) GetByClaim(ctx context.Context, claimID string) ([]models.EvidenceRecord, error) {
	var result []models.EvidenceRecord
	for _, r := range m.records {
		if r.ClaimID == claimID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MockEvidenceStore) GetByID(ctx context.Context, id string) (models.EvidenceRecord, error) {
	for _, r := range m.records {
		if r.ID == id {
			return r, nil
		}
	}
	return models.EvidenceRecord{}, fmt.Errorf("evidence record not found: %s", id)
}

func (m *MockEvidenceStore) UpdateValidity(ctx context.Context, id string, validity models.ValidityStatus) error {
	for i := range m.records {
		if m.records[i].ID == id {
			m.records[i].Validity = validity
			return nil
		}
	}
	return fmt.Errorf("evidence record not found: %s", id)
}

func (m *MockEvidenceStore) Delete(ctx context.Context, id string) error {
	for i := range m.records {
		if m.records[i].ID == id {
			m.records = append(m.records[:i], m.records[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("evidence record not found: %s", id)
}

type MockClaimStore struct {
	claims []models.Claim
}

func (m *MockClaimStore) CreateClaim(ctx context.Context, claim models.Claim) error {
	m.claims = append(m.claims, claim)
	return nil
}

func (m *MockClaimStore) GetClaim(ctx context.Context, id string) (models.Claim, error) {
	for _, c := range m.claims {
		if c.ID == id {
			return c, nil
		}
	}
	return models.Claim{}, fmt.Errorf("claim not found: %s", id)
}

func (m *MockClaimStore) UpdateClaim(ctx context.Context, claim models.Claim) error {
	for i := range m.claims {
		if m.claims[i].ID == claim.ID {
			m.claims[i] = claim
			return nil
		}
	}
	return fmt.Errorf("claim not found: %s", claim.ID)
}

func (m *MockClaimStore) GetBySubject(ctx context.Context, subject string, limit int) ([]models.Claim, error) {
	var result []models.Claim
	for _, c := range m.claims {
		if c.Subject == subject {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *MockClaimStore) UpdateStatus(ctx context.Context, id string, status models.ClaimStatus, confidence models.Confidence) error {
	for i := range m.claims {
		if m.claims[i].ID == id {
			m.claims[i].Status = status
			m.claims[i].Confidence = confidence
			return nil
		}
	}
	return fmt.Errorf("claim not found: %s", id)
}

type MockVerificationStore struct {
	mu     sync.Mutex
	plans  []models.VerificationPlan
	results []models.VerificationResult
}

func (m *MockVerificationStore) RecordPlan(ctx context.Context, plan models.VerificationPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plans = append(m.plans, plan)
	return nil
}

func (m *MockVerificationStore) RecordResult(ctx context.Context, result models.VerificationResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, result)
	return nil
}

func (m *MockVerificationStore) GetPlan(ctx context.Context, claimID string) (models.VerificationPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plans {
		if p.ClaimID == claimID {
			return p, nil
		}
	}
	return models.VerificationPlan{}, fmt.Errorf("verification plan not found")
}

func (m *MockVerificationStore) GetResult(ctx context.Context, claimID string) (models.VerificationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.results {
		if r.ClaimID == claimID {
			return r, nil
		}
	}
	return models.VerificationResult{}, fmt.Errorf("verification result not found")
}

func (m *MockVerificationStore) GetPlans(ctx context.Context, claimID string) ([]models.VerificationPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.VerificationPlan
	for _, p := range m.plans {
		if p.ClaimID == claimID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *MockVerificationStore) GetResults(ctx context.Context, claimID string) ([]models.VerificationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.VerificationResult
	for _, r := range m.results {
		if r.ClaimID == claimID {
			result = append(result, r)
		}
	}
	return result, nil
}

var (
	_ storage.EvidenceStore  = (*MockEvidenceStore)(nil)
	_ storage.ClaimStore     = (*MockClaimStore)(nil)
	_ storage.VerificationStore = (*MockVerificationStore)(nil)
)
