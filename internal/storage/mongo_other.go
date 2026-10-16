package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/requiem/cognitive/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoConversationStore struct {
	store *MongoStore
}

func (m *mongoConversationStore) CreateConversation(ctx context.Context, message models.ConversationMessage) error {
	message.Timestamp = time.Now().UTC()
	_, err := m.store.messagesCollection.InsertOne(ctx, message)
	return err
}

func (m *mongoConversationStore) AddMessage(ctx context.Context, conversationID string, message models.ConversationMessage) error {
	message.ConversationID = conversationID
	message.Timestamp = time.Now().UTC()
	_, err := m.store.messagesCollection.InsertOne(ctx, message)
	return err
}

func (m *mongoConversationStore) GetMessages(ctx context.Context, conversationID string, limit int) ([]models.ConversationMessage, error) {
	return m.GetMessagesPaginated(ctx, conversationID, limit, 0)
}

func (m *mongoConversationStore) GetMessagesPaginated(ctx context.Context, conversationID string, limit, offset int) ([]models.ConversationMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.D{{Key: "timestamp", Value: 1}})

	cursor, err := m.store.messagesCollection.Find(ctx, bson.M{"conversation_id": conversationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []models.ConversationMessage
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return messages, nil
}

func (m *mongoConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]ConversationSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	pipeline := mongo.Pipeline{
		{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: -1}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$conversation_id"},
			{Key: "title", Value: bson.D{{Key: "$first", Value: "$title"}}},
			{Key: "message_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "last_activity", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "last_activity", Value: -1}}}},
		{{Key: "$skip", Value: int64(offset)}},
		{{Key: "$limit", Value: int64(limit)}},
	}

	cursor, err := m.store.messagesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer cursor.Close(ctx)

	var results []ConversationSummary
	for cursor.Next(ctx) {
		var doc struct {
			ID           string    `bson:"_id"`
			Title        string    `bson:"title"`
			MessageCount int64     `bson:"message_count"`
			LastActivity time.Time `bson:"last_activity"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		results = append(results, ConversationSummary{
			ConversationID: doc.ID,
			Title:          doc.Title,
			MessageCount:   doc.MessageCount,
			LastActivity:   doc.LastActivity,
		})
	}

	return results, cursor.Err()
}

func (m *mongoConversationStore) DeleteAll(ctx context.Context, conversationID string) (int64, error) {
	result, err := m.store.messagesCollection.DeleteMany(ctx, bson.M{"conversation_id": conversationID})
	if err != nil {
		return 0, fmt.Errorf("delete conversation: %w", err)
	}
	return result.DeletedCount, nil
}

type mongoEventStore struct {
	store *MongoStore
}

func (m *mongoEventStore) RecordEvent(ctx context.Context, event models.PipelineEvent) error {
	_, err := m.store.eventsCollection.InsertOne(ctx, event)
	return err
}

func (m *mongoEventStore) GetEvents(ctx context.Context, pipelineRunID string) ([]models.PipelineEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := m.store.eventsCollection.Find(ctx, bson.M{"pipeline_run_id": pipelineRunID}, opts)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}
	defer cursor.Close(ctx)

	var events []models.PipelineEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	return events, nil
}

type mongoAuditStore struct {
	store *MongoStore
}

func (m *mongoAuditStore) RecordHistory(ctx context.Context, record AuditRecord) error {
	_, err := m.store.auditCollection.InsertOne(ctx, record)
	return err
}

func (m *mongoAuditStore) GetHistory(ctx context.Context, entityType, entityID string) ([]AuditRecord, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := m.store.auditCollection.Find(ctx, bson.M{
		"entity_type": entityType,
		"entity_id":   entityID,
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer cursor.Close(ctx)

	var records []AuditRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	return records, nil
}

func (m *mongoAuditStore) GetProvenance(ctx context.Context, entityID string) ([]ProvenanceStep, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := m.store.auditCollection.Find(ctx, bson.M{"entity_id": entityID}, opts)
	if err != nil {
		return nil, fmt.Errorf("get provenance: %w", err)
	}
	defer cursor.Close(ctx)

	var steps []ProvenanceStep
	for cursor.Next(ctx) {
		var record AuditRecord
		if err := cursor.Decode(&record); err != nil {
			continue
		}
		steps = append(steps, ProvenanceStep{
			Step:      record.Action,
			EntityID:  record.EntityID,
			Action:    record.Action,
			Timestamp: record.Timestamp,
			Reason:    record.Reason,
		})
	}
	return steps, cursor.Err()
}

type mongoPipelineRunStore struct {
	store *MongoStore
}

func (m *mongoPipelineRunStore) RecordRun(ctx context.Context, run models.PipelineRun) error {
	_, err := m.store.pipelineRunsCollection.InsertOne(ctx, run)
	return err
}

func (m *mongoPipelineRunStore) GetRun(ctx context.Context, id string) (models.PipelineRun, error) {
	var run models.PipelineRun
	err := m.store.pipelineRunsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&run)
	if err != nil {
		return run, fmt.Errorf("get pipeline run: %w", err)
	}
	return run, nil
}

type mongoEvidenceStore struct {
	store *MongoStore
}

func (m *mongoEvidenceStore) CreateRecord(ctx context.Context, record models.EvidenceRecord) error {
	_, err := m.store.evidenceCollection.InsertOne(ctx, record)
	return err
}

func (m *mongoEvidenceStore) GetByClaim(ctx context.Context, claimID string) ([]models.EvidenceRecord, error) {
	cursor, err := m.store.evidenceCollection.Find(ctx, bson.M{"claim_id": claimID})
	if err != nil {
		return nil, fmt.Errorf("get evidence by claim: %w", err)
	}
	defer cursor.Close(ctx)

	var records []models.EvidenceRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("decode evidence records: %w", err)
	}
	return records, nil
}

func (m *mongoEvidenceStore) GetByID(ctx context.Context, id string) (models.EvidenceRecord, error) {
	var record models.EvidenceRecord
	err := m.store.evidenceCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&record)
	if err != nil {
		return record, fmt.Errorf("get evidence record: %w", err)
	}
	return record, nil
}

func (m *mongoEvidenceStore) UpdateValidity(ctx context.Context, id string, validity models.ValidityStatus) error {
	result, err := m.store.evidenceCollection.UpdateOne(ctx, bson.M{"_id": id}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "validity", Value: validity}}},
		{Key: "$set", Value: bson.D{{Key: "updated_at", Value: time.Now().UTC()}}},
	})
	if err != nil {
		return fmt.Errorf("update evidence validity: %w", err)
	}
	if result.ModifiedCount == 0 {
		return fmt.Errorf("evidence record not found: %s", id)
	}
	return nil
}

func (m *mongoEvidenceStore) Delete(ctx context.Context, id string) error {
	result, err := m.store.evidenceCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete evidence record: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("evidence record not found: %s", id)
	}
	return nil
}

type mongoClaimStore struct {
	store *MongoStore
}

func (m *mongoClaimStore) CreateClaim(ctx context.Context, claim models.Claim) error {
	_, err := m.store.claimsCollection.InsertOne(ctx, claim)
	return err
}

func (m *mongoClaimStore) GetClaim(ctx context.Context, id string) (models.Claim, error) {
	var claim models.Claim
	err := m.store.claimsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&claim)
	if err != nil {
		return claim, fmt.Errorf("get claim: %w", err)
	}
	return claim, nil
}

func (m *mongoClaimStore) UpdateClaim(ctx context.Context, claim models.Claim) error {
	claim.UpdatedAt = time.Now().UTC()
	result, err := m.store.claimsCollection.ReplaceOne(ctx, bson.M{"_id": claim.ID}, claim)
	if err != nil {
		return fmt.Errorf("update claim: %w", err)
	}
	if result.ModifiedCount == 0 {
		return fmt.Errorf("claim not found: %s", claim.ID)
	}
	return nil
}

func (m *mongoClaimStore) GetBySubject(ctx context.Context, subject string, limit int) ([]models.Claim, error) {
	if limit <= 0 {
		limit = 100
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := m.store.claimsCollection.Find(ctx, bson.M{"subject": subject}, opts)
	if err != nil {
		return nil, fmt.Errorf("get claims by subject: %w", err)
	}
	defer cursor.Close(ctx)

	var claims []models.Claim
	if err := cursor.All(ctx, &claims); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	return claims, nil
}

func (m *mongoClaimStore) UpdateStatus(ctx context.Context, id string, status models.ClaimStatus, confidence models.Confidence) error {
	result, err := m.store.claimsCollection.UpdateOne(ctx, bson.M{"_id": id}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "status", Value: status}, {Key: "confidence", Value: confidence}}},
		{Key: "$set", Value: bson.D{{Key: "updated_at", Value: time.Now().UTC()}}},
	})
	if err != nil {
		return fmt.Errorf("update claim status: %w", err)
	}
	if result.ModifiedCount == 0 {
		return fmt.Errorf("claim not found: %s", id)
	}
	return nil
}

type mongoVerificationStore struct {
	store *MongoStore
}

func (m *mongoVerificationStore) RecordPlan(ctx context.Context, plan models.VerificationPlan) error {
	_, err := m.store.verificationPlansCollection.InsertOne(ctx, plan)
	return err
}

func (m *mongoVerificationStore) RecordResult(ctx context.Context, result models.VerificationResult) error {
	_, err := m.store.verificationResultsCollection.InsertOne(ctx, result)
	return err
}

func (m *mongoVerificationStore) GetPlan(ctx context.Context, claimID string) (models.VerificationPlan, error) {
	var plan models.VerificationPlan
	err := m.store.verificationPlansCollection.FindOne(ctx, bson.M{"claim_id": claimID}).Decode(&plan)
	if err != nil {
		return plan, fmt.Errorf("get verification plan: %w", err)
	}
	return plan, nil
}

func (m *mongoVerificationStore) GetResult(ctx context.Context, claimID string) (models.VerificationResult, error) {
	var result models.VerificationResult
	err := m.store.verificationResultsCollection.FindOne(ctx, bson.M{"claim_id": claimID}).Decode(&result)
	if err != nil {
		return result, fmt.Errorf("get verification result: %w", err)
	}
	return result, nil
}
