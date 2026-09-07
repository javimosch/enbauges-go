package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// bkn files backend for card images. enbauges-go proxies uploads so the
// browser never sees the bkn admin token. Images are served publicly from
// bkn.vps1.intrane.fr.

const (
	bknBaseURL    = "http://bkn-enbauges:7799"
	bknPublicURL  = "https://bkn.vps1.intrane.fr"
	bknNamespace  = "enbauges"
	maxImages     = 5
	maxImageBytes = 5 << 20 // 5 MiB
)

func bknToken() string { return os.Getenv("BKN_ADMIN_TOKEN") }

// handleUploadImage receives a multipart upload, forwards it to bkn, and
// appends the returned file URL to the card's images array.
func handleUploadImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}

	// Parse multipart
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		writeErr(w, 400, "invalid multipart upload: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "missing 'file' field")
		return
	}
	defer file.Close()

	if header.Size > maxImageBytes {
		writeErr(w, 413, fmt.Sprintf("image too large: %d bytes (max %d)", header.Size, maxImageBytes))
		return
	}

	// Detect content type from the file header, not the extension
	ct := header.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "image/") {
		writeErr(w, 400, "file must be an image (got: "+ct+")")
		return
	}

	// Read the file body
	body, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		writeErr(w, 500, "failed to read upload: "+err.Error())
		return
	}
	if len(body) > maxImageBytes {
		writeErr(w, 413, "image too large")
		return
	}

	// Forward to bkn
	bknReq, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/files/%s/%s", bknBaseURL, bknNamespace, header.Filename),
		bytes.NewReader(body))
	bknReq.Header.Set("Authorization", "Bearer "+bknToken())
	bknReq.Header.Set("Content-Type", ct)

	bknResp, err := http.DefaultClient.Do(bknReq)
	if err != nil {
		writeErr(w, 502, "bkn upload failed: "+err.Error())
		return
	}
	defer bknResp.Body.Close()

	if bknResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(bknResp.Body)
		writeErr(w, bknResp.StatusCode, "bkn rejected upload: "+string(respBody))
		return
	}

	var bknResult struct {
		File struct {
			Name string `json:"name"`
		} `json:"file"`
	}
	if err := json.NewDecoder(bknResp.Body).Decode(&bknResult); err != nil {
		writeErr(w, 502, "failed to parse bkn response: "+err.Error())
		return
	}

	imageURL := fmt.Sprintf("%s/v1/files/%s/%s", bknPublicURL, bknNamespace, bknResult.File.Name)

	// Append to card's images array (cap at maxImages)
	var card Card
	err = cards().FindOneAndUpdate(ctx, bson.M{"_id": id},
		bson.M{
			"$push": bson.M{"images": bson.M{"$each": []string{imageURL}, "$slice": -maxImages}},
			"$set":  bson.M{"updatedAt": time.Now().UTC()},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&card)
	if err == mongo.ErrNoDocuments {
		writeErr(w, 404, "Card not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"card": card, "imageUrl": imageURL})
}

// handleDeleteImage removes an image URL from the card's images array.
// The file itself stays in bkn (content-addressed, may be referenced elsewhere).
func handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		writeErr(w, 400, "missing 'url' query parameter")
		return
	}

	var card Card
	err := cards().FindOneAndUpdate(ctx, bson.M{"_id": id},
		bson.M{
			"$pull":  bson.M{"images": imageURL},
			"$set":   bson.M{"updatedAt": time.Now().UTC()},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&card)
	if err == mongo.ErrNoDocuments {
		writeErr(w, 404, "Card not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, card)
}

// sniffContentType is a helper for tests; not used in production.
var _ = mime.TypeByExtension
var _ = primitive.NewObjectID
var _ = context.Background
