package main

import (
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Card mirrors src/models/Card.js. Voters is bson-only: the Node toJSON
// strips it from responses, so json:"-" here.
type Card struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Type        string             `bson:"type" json:"type"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	Tags        []string           `bson:"tags" json:"tags"`
	URL         string             `bson:"url,omitempty" json:"url,omitempty"`
	Contact     string             `bson:"contact,omitempty" json:"contact,omitempty"`
	Address     string             `bson:"address,omitempty" json:"address,omitempty"`
	Lat         *float64           `bson:"lat,omitempty" json:"lat,omitempty"`
	Lng         *float64           `bson:"lng,omitempty" json:"lng,omitempty"`
	Images      []string           `bson:"images,omitempty" json:"images,omitempty"`
	Slug        string             `bson:"slug,omitempty" json:"slug,omitempty"`
	Votes       int                `bson:"votes" json:"votes"`
	Voters      []string           `bson:"voters" json:"-"`
	IsExample   bool               `bson:"isExample" json:"isExample"`
	Featured    bool               `bson:"featured" json:"featured"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// CardRef is the populated {_id, title, type} shape mongoose returns for
// link endpoints.
type CardRef struct {
	ID    primitive.ObjectID `bson:"_id" json:"_id"`
	Title string             `bson:"title" json:"title"`
	Type  string             `bson:"type" json:"type"`
}

type Link struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	SourceCardID     primitive.ObjectID `bson:"sourceCardId" json:"-"`
	TargetCardID     primitive.ObjectID `bson:"targetCardId" json:"-"`
	RelationshipType string             `bson:"relationshipType" json:"relationshipType"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// PopulatedLink is Link with card refs expanded, as the canvas UI expects.
type PopulatedLink struct {
	ID               primitive.ObjectID `json:"_id"`
	SourceCard       *CardRef           `json:"sourceCardId"`
	TargetCard       *CardRef           `json:"targetCardId"`
	RelationshipType string             `json:"relationshipType"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
}

type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	CardID    primitive.ObjectID `bson:"cardId" json:"cardId"`
	Content   string             `bson:"content" json:"content"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Event mirrors src/models/OrgEvent.js (collection "orgevents"). The org/user
// refs stay optional so documents created by the Node app read fine.
type Event struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	OrgID        *primitive.ObjectID `bson:"orgId,omitempty" json:"orgId,omitempty"`
	ContactEmail string              `bson:"contactEmail,omitempty" json:"contactEmail,omitempty"`
	Title        string              `bson:"title" json:"title"`
	Description  string              `bson:"description,omitempty" json:"description,omitempty"`
	StartAt      time.Time           `bson:"startAt" json:"startAt"`
	EndAt        time.Time           `bson:"endAt" json:"endAt"`
	Location     string              `bson:"location,omitempty" json:"location,omitempty"`
	Category     string              `bson:"category,omitempty" json:"category,omitempty"`
	Status       string              `bson:"status" json:"status"`
	CreatedAt    time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt" json:"updatedAt"`
}

func cards() *mongo.Collection    { return db.Collection("cards") }
func links() *mongo.Collection    { return db.Collection("links") }
func comments() *mongo.Collection { return db.Collection("comments") }
func events() *mongo.Collection   { return db.Collection("orgevents") }

func cardIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "tags", Value: 1}}},
		{Keys: bson.D{{Key: "votes", Value: -1}}},
		{Keys: bson.D{{Key: "isExample", Value: 1}}},
		{Keys: bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true)},
	}
}

func linkIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "sourceCardId", Value: 1}, {Key: "targetCardId", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "relationshipType", Value: 1}}},
	}
}

func commentIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "cardId", Value: 1}, {Key: "createdAt", Value: 1}}},
	}
}

func eventIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "startAt", Value: 1}}},
	}
}

// --- shared HTTP helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}
