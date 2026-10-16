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

type mongoMindStore struct {
	store *MongoStore
}

func (m *mongoMindStore) GetBeliefs(ctx context.Context, query models.MindQuery) ([]models.Belief, error) {
	return queryBeliefs(ctx, m.store.beliefsCollection, query)
}

func (m *mongoMindStore) CreateBelief(ctx context.Context, belief models.Belief) error {
	if belief.UserID == "" {
		belief.UserID = GetUserIDFromContext(ctx)
	}
	belief.CreatedAt = time.Now().UTC()
	belief.UpdatedAt = belief.CreatedAt
	_, err := m.store.beliefsCollection.InsertOne(ctx, belief)
	return err
}

func (m *mongoMindStore) UpdateBelief(ctx context.Context, belief models.Belief) error {
	belief.UpdatedAt = time.Now().UTC()
	_, err := m.store.beliefsCollection.UpdateOne(ctx, bson.M{"_id": belief.ID}, bson.M{"$set": belief})
	return err
}

func (m *mongoMindStore) GetOpinions(ctx context.Context, query models.MindQuery) ([]models.Opinion, error) {
	return queryOpinions(ctx, m.store.opinionsCollection, query)
}

func (m *mongoMindStore) CreateOpinion(ctx context.Context, opinion models.Opinion) error {
	opinion.CreatedAt = time.Now().UTC()
	opinion.UpdatedAt = opinion.CreatedAt
	_, err := m.store.opinionsCollection.InsertOne(ctx, opinion)
	return err
}

func (m *mongoMindStore) UpdateOpinion(ctx context.Context, opinion models.Opinion) error {
	opinion.UpdatedAt = time.Now().UTC()
	_, err := m.store.opinionsCollection.UpdateOne(ctx, bson.M{"_id": opinion.ID}, bson.M{"$set": opinion})
	return err
}

func (m *mongoMindStore) GetPreferences(ctx context.Context, query models.MindQuery) ([]models.Preference, error) {
	return queryPreferences(ctx, m.store.preferencesCollection, query)
}

func (m *mongoMindStore) CreatePreference(ctx context.Context, preference models.Preference) error {
	preference.CreatedAt = time.Now().UTC()
	preference.UpdatedAt = preference.CreatedAt
	_, err := m.store.preferencesCollection.InsertOne(ctx, preference)
	return err
}

func (m *mongoMindStore) UpdatePreference(ctx context.Context, preference models.Preference) error {
	preference.UpdatedAt = time.Now().UTC()
	_, err := m.store.preferencesCollection.UpdateOne(ctx, bson.M{"_id": preference.ID}, bson.M{"$set": preference})
	return err
}

func (m *mongoMindStore) GetValues(ctx context.Context, query models.MindQuery) ([]models.Value, error) {
	return queryValues(ctx, m.store.valuesCollection, query)
}

func (m *mongoMindStore) CreateValue(ctx context.Context, value models.Value) error {
	value.CreatedAt = time.Now().UTC()
	value.UpdatedAt = value.CreatedAt
	_, err := m.store.valuesCollection.InsertOne(ctx, value)
	return err
}

func (m *mongoMindStore) GetGoals(ctx context.Context, query models.MindQuery) ([]models.Goal, error) {
	return queryGoals(ctx, m.store.goalsCollection, query)
}

func (m *mongoMindStore) CreateGoal(ctx context.Context, goal models.Goal) error {
	goal.CreatedAt = time.Now().UTC()
	goal.UpdatedAt = goal.CreatedAt
	_, err := m.store.goalsCollection.InsertOne(ctx, goal)
	return err
}

func (m *mongoMindStore) UpdateGoal(ctx context.Context, goal models.Goal) error {
	goal.UpdatedAt = time.Now().UTC()
	_, err := m.store.goalsCollection.UpdateOne(ctx, bson.M{"_id": goal.ID}, bson.M{"$set": goal})
	return err
}

func (m *mongoMindStore) GetHypotheses(ctx context.Context, query models.MindQuery) ([]models.Hypothesis, error) {
	return queryHypotheses(ctx, m.store.hypothesesCollection, query)
}

func (m *mongoMindStore) CreateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	hypothesis.CreatedAt = time.Now().UTC()
	hypothesis.UpdatedAt = hypothesis.CreatedAt
	_, err := m.store.hypothesesCollection.InsertOne(ctx, hypothesis)
	return err
}

func (m *mongoMindStore) UpdateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	hypothesis.UpdatedAt = time.Now().UTC()
	_, err := m.store.hypothesesCollection.UpdateOne(ctx, bson.M{"_id": hypothesis.ID}, bson.M{"$set": hypothesis})
	return err
}

func (m *mongoMindStore) GetUncertainties(ctx context.Context, query models.MindQuery) ([]models.Uncertainty, error) {
	return queryUncertainties(ctx, m.store.uncertaintiesCollection, query)
}

func (m *mongoMindStore) CreateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	uncertainty.CreatedAt = time.Now().UTC()
	uncertainty.UpdatedAt = uncertainty.CreatedAt
	_, err := m.store.uncertaintiesCollection.InsertOne(ctx, uncertainty)
	return err
}

func (m *mongoMindStore) UpdateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	uncertainty.UpdatedAt = time.Now().UTC()
	_, err := m.store.uncertaintiesCollection.UpdateOne(ctx, bson.M{"_id": uncertainty.ID}, bson.M{"$set": uncertainty})
	return err
}

func (m *mongoMindStore) GetSelfObservations(ctx context.Context, query models.MindQuery) ([]models.SelfObservation, error) {
	return querySelfObservations(ctx, m.store.selfObservationsCollection, query)
}

func (m *mongoMindStore) CreateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	observation.CreatedAt = time.Now().UTC()
	observation.UpdatedAt = observation.CreatedAt
	_, err := m.store.selfObservationsCollection.InsertOne(ctx, observation)
	return err
}

func (m *mongoMindStore) UpdateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	observation.UpdatedAt = time.Now().UTC()
	_, err := m.store.selfObservationsCollection.UpdateOne(ctx, bson.M{"_id": observation.ID}, bson.M{"$set": observation})
	return err
}

func (m *mongoMindStore) GetContradictions(ctx context.Context, query models.MindQuery) ([]models.Contradiction, error) {
	return queryContradictions(ctx, m.store.contradictionsCollection, query)
}

func (m *mongoMindStore) CreateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	contradiction.CreatedAt = time.Now().UTC()
	contradiction.UpdatedAt = contradiction.CreatedAt
	_, err := m.store.contradictionsCollection.InsertOne(ctx, contradiction)
	return err
}

func (m *mongoMindStore) UpdateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	contradiction.UpdatedAt = time.Now().UTC()
	_, err := m.store.contradictionsCollection.UpdateOne(ctx, bson.M{"_id": contradiction.ID}, bson.M{"$set": contradiction})
	return err
}

func buildMindFilter(ctx context.Context, query models.MindQuery) bson.M {
	if query.UserID == "" {
		query.UserID = GetUserIDFromContext(ctx)
	}
	filter := bson.M{}

	if query.UserID != "" {
		filter["user_id"] = query.UserID
	}
	if len(query.IDs) > 0 {
		filter["_id"] = bson.M{"$in": query.IDs}
	}
	if query.Status != "" {
		filter["status"] = query.Status
	}
	if len(query.Subjects) > 0 {
		filter["subject"] = bson.M{"$in": query.Subjects}
	}
	if query.MinConfidence > 0 {
		filter["confidence"] = bson.M{"$gte": query.MinConfidence}
	}
	if query.SourceType != "" {
		filter["source_type"] = query.SourceType
	}

	return filter
}

func getMindFindOptions(query models.MindQuery) *options.FindOptions {
	limit := int64(50)
	if query.Limit > 0 {
		limit = int64(query.Limit)
	}

	return options.Find().SetLimit(limit).SetSort(bson.D{{Key: "confidence", Value: -1}})
}

func queryBeliefs(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Belief, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query beliefs: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Belief
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode beliefs: %w", err)
	}
	return results, nil
}

func queryOpinions(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Opinion, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query opinions: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Opinion
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode opinions: %w", err)
	}
	return results, nil
}

func queryPreferences(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Preference, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query preferences: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Preference
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode preferences: %w", err)
	}
	return results, nil
}

func queryValues(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Value, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query values: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Value
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode values: %w", err)
	}
	return results, nil
}

func queryGoals(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Goal, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Goal
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode goals: %w", err)
	}
	return results, nil
}

func queryHypotheses(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Hypothesis, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query hypotheses: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Hypothesis
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode hypotheses: %w", err)
	}
	return results, nil
}

func queryUncertainties(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Uncertainty, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query uncertainties: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Uncertainty
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode uncertainties: %w", err)
	}
	return results, nil
}

func querySelfObservations(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.SelfObservation, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query self_observations: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.SelfObservation
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode self_observations: %w", err)
	}
	return results, nil
}

func queryContradictions(ctx context.Context, coll *mongo.Collection, query models.MindQuery) ([]models.Contradiction, error) {
	cursor, err := coll.Find(ctx, buildMindFilter(ctx, query), getMindFindOptions(query))
	if err != nil {
		return nil, fmt.Errorf("query contradictions: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.Contradiction
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode contradictions: %w", err)
	}
	return results, nil
}
