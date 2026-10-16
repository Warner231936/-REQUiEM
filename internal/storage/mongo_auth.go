package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/requiem/cognitive/pkg/models"
)

type mongoAuthStore struct {
	store *MongoStore
}

func (s *mongoAuthStore) CreateUser(ctx context.Context, user models.User) error {
	if user.ID == "" {
		user.ID = models.NewID()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := s.store.usersCollection.InsertOne(ctx, user)
	return err
}

func (s *mongoAuthStore) GetUser(ctx context.Context, userID string) (models.User, error) {
	var user models.User
	err := s.store.usersCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *mongoAuthStore) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := s.store.usersCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *mongoAuthStore) UpdateUser(ctx context.Context, user models.User) error {
	user.UpdatedAt = time.Now().UTC()
	_, err := s.store.usersCollection.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": user},
		options.Update().SetUpsert(false),
	)
	return err
}

func (s *mongoAuthStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.store.usersCollection.DeleteOne(ctx, bson.M{"_id": userID})
	return err
}

func (s *mongoAuthStore) ListUsers(ctx context.Context) ([]models.User, error) {
	cursor, err := s.store.usersCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *mongoAuthStore) CreateVerificationToken(ctx context.Context, token models.VerificationToken) error {
	_, err := s.store.verificationTokensCollection.InsertOne(ctx, token)
	return err
}

func (s *mongoAuthStore) GetVerificationToken(ctx context.Context, token string) (models.VerificationToken, error) {
	var vt models.VerificationToken
	err := s.store.verificationTokensCollection.FindOne(ctx, bson.M{"token": token}).Decode(&vt)
	if err != nil {
		return models.VerificationToken{}, err
	}
	return vt, nil
}

func (s *mongoAuthStore) DeleteVerificationToken(ctx context.Context, token string) error {
	_, err := s.store.verificationTokensCollection.DeleteOne(ctx, bson.M{"token": token})
	return err
}

func (s *mongoAuthStore) CreatePasswordResetToken(ctx context.Context, token models.PasswordResetToken) error {
	_, err := s.store.passwordResetTokensCollection.InsertOne(ctx, token)
	return err
}

func (s *mongoAuthStore) GetPasswordResetToken(ctx context.Context, token string) (models.PasswordResetToken, error) {
	var prt models.PasswordResetToken
	err := s.store.passwordResetTokensCollection.FindOne(ctx, bson.M{"token": token}).Decode(&prt)
	if err != nil {
		return models.PasswordResetToken{}, err
	}
	return prt, nil
}

func (s *mongoAuthStore) DeletePasswordResetToken(ctx context.Context, token string) error {
	_, err := s.store.passwordResetTokensCollection.DeleteOne(ctx, bson.M{"token": token})
	return err
}

var _ AuthStore = (*mongoAuthStore)(nil)
