package main

import (
	"context"
	"net/http"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var relationshipTypes = []string{"depends_on", "reinforces", "uses", "inspires"}

// populatedLinks fetches links matching filter and expands both card refs,
// mirroring mongoose populate('sourceCardId', 'title type').
func populatedLinks(ctx context.Context, filter bson.M) ([]PopulatedLink, error) {
	cur, err := links().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	raw := []Link{}
	if err := cur.All(ctx, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []PopulatedLink{}, nil
	}

	idSet := map[primitive.ObjectID]bool{}
	for _, l := range raw {
		idSet[l.SourceCardID] = true
		idSet[l.TargetCardID] = true
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	ccur, err := cards().Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"title": 1, "type": 1}))
	if err != nil {
		return nil, err
	}
	refs := []CardRef{}
	if err := ccur.All(ctx, &refs); err != nil {
		return nil, err
	}
	refMap := map[primitive.ObjectID]*CardRef{}
	for i := range refs {
		refMap[refs[i].ID] = &refs[i]
	}

	out := make([]PopulatedLink, 0, len(raw))
	for _, l := range raw {
		out = append(out, PopulatedLink{
			ID:               l.ID,
			SourceCard:       refMap[l.SourceCardID],
			TargetCard:       refMap[l.TargetCardID],
			RelationshipType: l.RelationshipType,
			CreatedAt:        l.CreatedAt,
			UpdatedAt:        l.UpdatedAt,
		})
	}
	return out, nil
}

func handleListLinks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	filter := bson.M{}
	if cardID := r.URL.Query().Get("cardId"); cardID != "" {
		id, err := primitive.ObjectIDFromHex(cardID)
		if err != nil {
			writeJSON(w, 200, map[string]any{"links": []PopulatedLink{}})
			return
		}
		filter["$or"] = bson.A{bson.M{"sourceCardId": id}, bson.M{"targetCardId": id}}
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter["relationshipType"] = t
	}
	list, err := populatedLinks(ctx, filter)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"links": list})
}

func handleCreateLink(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	var in struct {
		SourceCardID     string `json:"sourceCardId"`
		TargetCardID     string `json:"targetCardId"`
		RelationshipType string `json:"relationshipType"`
	}
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	if in.SourceCardID == "" || in.TargetCardID == "" || in.RelationshipType == "" {
		writeErr(w, 400, "sourceCardId, targetCardId, and relationshipType are required")
		return
	}
	if !slices.Contains(relationshipTypes, in.RelationshipType) {
		writeErr(w, 400, "relationshipType must be one of: depends_on, reinforces, uses, inspires")
		return
	}
	if in.SourceCardID == in.TargetCardID {
		writeErr(w, 400, "cannot link a card to itself")
		return
	}
	srcID, err1 := primitive.ObjectIDFromHex(in.SourceCardID)
	tgtID, err2 := primitive.ObjectIDFromHex(in.TargetCardID)
	if err1 != nil || err2 != nil {
		writeErr(w, 404, "source or target card not found")
		return
	}
	n, err := cards().CountDocuments(ctx, bson.M{"_id": bson.M{"$in": bson.A{srcID, tgtID}}})
	if err != nil || n != 2 {
		writeErr(w, 404, "source or target card not found")
		return
	}
	if err := links().FindOne(ctx, bson.M{"sourceCardId": srcID, "targetCardId": tgtID}).Err(); err == nil {
		writeErr(w, 400, "link already exists")
		return
	}
	now := time.Now().UTC()
	link := Link{
		ID:               primitive.NewObjectID(),
		SourceCardID:     srcID,
		TargetCardID:     tgtID,
		RelationshipType: in.RelationshipType,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := links().InsertOne(ctx, link); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	populated, err := populatedLinks(ctx, bson.M{"_id": link.ID})
	if err != nil || len(populated) == 0 {
		writeErr(w, 500, "created but failed to load link")
		return
	}
	writeJSON(w, 201, populated[0])
}

func handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeErr(w, 404, "Link not found")
		return
	}
	res, err := links().DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if res.DeletedCount == 0 {
		writeErr(w, 404, "Link not found")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true})
}
