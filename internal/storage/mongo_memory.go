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

type mongoMemoryStore struct {
	store *MongoStore
}

func (m *mongoMemoryStore) Create(ctx context.Context, memory models.Memory) error {
	memory.CreatedAt = time.Now().UTC()
	memory.UpdatedAt = memory.CreatedAt

	_, err := m.store.memoryCollection.InsertOne(ctx, memory)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	return nil
}

func (m *mongoMemoryStore) Get(ctx context.Context, id string) (models.Memory, error) {
	var memory models.Memory
	err := m.store.memoryCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&memory)
	if err == mongo.ErrNoDocuments {
		return memory, models.NotFoundError{Resource: "memory", ID: id}
	}
	if err != nil {
		return memory, fmt.Errorf("get memory: %w", err)
	}
	return memory, nil
}

func (m *mongoMemoryStore) Search(ctx context.Context, query models.MemoryQuery) ([]models.MemorySearchResult, error) {
	filter := bson.M{}

	if query.Type != "" {
		filter["type"] = query.Type
	}
	if query.ConversationID != "" {
		filter["conversation_id"] = query.ConversationID
	}
	if len(query.Tags) > 0 {
		filter["tags"] = bson.M{"$in": query.Tags}
	}
	if query.MinImportance > 0 {
		filter["importance"] = bson.M{"$gte": query.MinImportance}
	}
	if query.SearchText != "" {
		filter["$text"] = bson.M{"$search": query.SearchText}
	}

	limit := int64(20)
	if query.Limit > 0 {
		limit = int64(query.Limit)
	}

	opts := options.Find().SetLimit(limit).SetSort(bson.D{
		{Key: "importance", Value: -1},
		{Key: "created_at", Value: -1},
	})

	cursor, err := m.store.memoryCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.MemorySearchResult
	now := time.Now().UTC()

	for cursor.Next(ctx) {
		var mem models.Memory
		if err := cursor.Decode(&mem); err != nil {
			continue
		}
		relevance := mem.CalculateEffectiveRelevance(now)
		results = append(results, models.MemorySearchResult{
			Memory:     mem,
			Similarity: relevance,
			Relevance:  relevance,
		})
	}

	return results, cursor.Err()
}

func (m *mongoMemoryStore) Update(ctx context.Context, memory models.Memory) error {
	memory.UpdatedAt = time.Now().UTC()

	filter := bson.M{"_id": memory.ID}
	update := bson.M{"$set": memory}

	result, err := m.store.memoryCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFoundError{Resource: "memory", ID: memory.ID}
	}
	return nil
}

func (m *mongoMemoryStore) Delete(ctx context.Context, id string) error {
	result, err := m.store.memoryCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.NotFoundError{Resource: "memory", ID: id}
	}
	return nil
}

func (m *mongoMemoryStore) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Memory, error) {
	if limit <= 0 {
		limit = 50
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := m.store.memoryCollection.Find(ctx, bson.M{"conversation_id": conversationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("get memories by conversation: %w", err)
	}
	defer cursor.Close(ctx)

	var memories []models.Memory
	if err := cursor.All(ctx, &memories); err != nil {
		return nil, fmt.Errorf("decode memories: %w", err)
	}
	return memories, nil
}
