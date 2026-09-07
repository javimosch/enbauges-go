package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// APIKey represents a key that grants access to bulk data exports.
type APIKey struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Key       string             `bson:"key" json:"key"`           // the actual secret (ebg_...)
	Name      string             `bson:"name" json:"name"`         // human label
	Email     string             `bson:"email,omitempty" json:"email,omitempty"`
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	RevokedAt *time.Time         `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"`
	LastUsedAt *time.Time        `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
}

func apiKeys() *mongo.Collection { return db.Collection("api_keys") }

func apiKeyIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "active", Value: 1}}},
	}
}

// generateKey produces a random key with an "ebg_" prefix (enbauges).
func generateKey() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return "ebg_" + hex.EncodeToString(b)
}

// POST /api/api-keys — admin creates a new API key.
func handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeErr(w, 401, "unauthorized")
		return
	}
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	if in.Name == "" {
		writeErr(w, 400, "name is required")
		return
	}
	k := APIKey{
		ID:        primitive.NewObjectID(),
		Key:       generateKey(),
		Name:      in.Name,
		Email:     in.Email,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := apiKeys().InsertOne(r.Context(), k); err != nil {
		writeErr(w, 500, "failed to create key")
		return
	}
	writeJSON(w, 201, k)
}

// GET /api/api-keys — admin lists all keys (secret included for management).
func handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeErr(w, 401, "unauthorized")
		return
	}
	cur, err := apiKeys().Find(r.Context(),
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		writeJSON(w, 200, map[string]any{"keys": []any{}})
		return
	}
	var list []APIKey
	_ = cur.All(r.Context(), &list)
	if list == nil {
		list = []APIKey{}
	}
	writeJSON(w, 200, map[string]any{"keys": list, "total": len(list)})
}

// DELETE /api/api-keys/{id} — admin revokes a key.
func handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeErr(w, 401, "unauthorized")
		return
	}
	idStr := r.PathValue("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		writeErr(w, 400, "invalid key id")
		return
	}
	now := time.Now().UTC()
	res, err := apiKeys().UpdateByID(r.Context(), id, bson.M{
		"$set": bson.M{"active": false, "revokedAt": now},
	})
	if err != nil || res.MatchedCount == 0 {
		writeErr(w, 404, "key not found")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true})
}

// validateAPIKey checks a key string against the DB and updates lastUsedAt.
func validateAPIKey(r *http.Request, key string) bool {
	ctx := r.Context()
	var k APIKey
	err := apiKeys().FindOne(ctx, bson.M{"key": key, "active": true}).Decode(&k)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	_, _ = apiKeys().UpdateByID(ctx, k.ID, bson.M{"$set": bson.M{"lastUsedAt": now}})
	return true
}
