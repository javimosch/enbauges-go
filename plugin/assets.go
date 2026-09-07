package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Assets is the sovereign object store: files on local disk, metadata in
// the "assets" collection. Keys are random and unguessable; an asset is
// born "orphan" and must be linked by the feature that references it, or
// the GC reclaims it.
type Assets struct {
	Dir string
	db  *mongo.Database
}

type AssetDoc struct {
	Key         string    `bson:"key" json:"key"`
	ThumbKey    string    `bson:"thumbKey,omitempty" json:"thumbKey,omitempty"`
	ContentType string    `bson:"contentType" json:"contentType"`
	SizeBytes   int       `bson:"sizeBytes" json:"sizeBytes"`
	Width       int       `bson:"width,omitempty" json:"width,omitempty"`
	Height      int       `bson:"height,omitempty" json:"height,omitempty"`
	Namespace   string    `bson:"namespace" json:"namespace"`
	Provider    string    `bson:"provider" json:"provider"`
	Status      string    `bson:"status" json:"status"` // orphan | linked
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
}

func NewAssets(dir string, db *mongo.Database) (*Assets, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Assets{Dir: dir, db: db}, nil
}

func (a *Assets) col() *mongo.Collection { return a.db.Collection("assets") }

func randomKey(namespace, ext string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return namespace + "/" + hex.EncodeToString(b) + ext
}

func (a *Assets) pathFor(key string) (string, error) {
	// Keys are server-generated, but stay paranoid about traversal.
	clean := filepath.Clean(filepath.FromSlash(key))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errors.New("invalid asset key")
	}
	return filepath.Join(a.Dir, clean), nil
}

// Put stores the main image and its thumbnail under a fresh key pair and
// records the metadata doc (status orphan).
func (a *Assets) Put(ctx context.Context, namespace string, data, thumb []byte, contentType, ext string, width, height int) (*AssetDoc, error) {
	key := randomKey(namespace, ext)
	thumbKey := strings.TrimSuffix(key, ext) + "_thumb" + ext

	p, err := a.pathFor(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return nil, err
	}
	if thumb != nil {
		tp, _ := a.pathFor(thumbKey)
		if err := os.WriteFile(tp, thumb, 0o644); err != nil {
			return nil, err
		}
	} else {
		thumbKey = ""
	}

	doc := &AssetDoc{
		Key: key, ThumbKey: thumbKey, ContentType: contentType,
		SizeBytes: len(data), Width: width, Height: height,
		Namespace: namespace, Provider: "local", Status: "orphan",
		CreatedAt: time.Now().UTC(),
	}
	if _, err := a.col().InsertOne(ctx, doc); err != nil {
		_ = os.Remove(p)
		return nil, err
	}
	return doc, nil
}

// Link marks an asset as referenced; linked assets survive the GC.
func (a *Assets) Link(ctx context.Context, key string) error {
	res, err := a.col().UpdateOne(ctx,
		bson.M{"key": key, "provider": "local"},
		bson.M{"$set": bson.M{"status": "linked"}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("asset not found")
	}
	return nil
}

// Get returns the metadata doc for a key (local provider only).
func (a *Assets) Get(ctx context.Context, key string) (*AssetDoc, error) {
	var doc AssetDoc
	err := a.col().FindOne(ctx, bson.M{"key": key, "provider": "local"}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Delete removes files (main + thumb) and the metadata doc.
func (a *Assets) Delete(ctx context.Context, key string) error {
	doc, err := a.Get(ctx, key)
	if err != nil {
		return nil // already gone
	}
	if p, err := a.pathFor(doc.Key); err == nil {
		_ = os.Remove(p)
	}
	if doc.ThumbKey != "" {
		if p, err := a.pathFor(doc.ThumbKey); err == nil {
			_ = os.Remove(p)
		}
	}
	_, err = a.col().DeleteOne(ctx, bson.M{"key": key, "provider": "local"})
	return err
}

// ServeHTTP handles GET /public/assets/{key...}. Keys are unique and
// content-addressed-ish, so immutable caching is safe.
func (a *Assets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/public/assets/")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	// Serve both main keys and derived thumb files (same doc).
	lookup := key
	if i := strings.LastIndex(key, "_thumb"); i > 0 {
		lookup = key[:i] + key[i+len("_thumb"):]
	}
	doc, err := a.Get(ctx, lookup)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := a.pathFor(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// StartGC reclaims orphan assets older than maxAge, hourly.
func (a *Assets) StartGC(maxAge time.Duration) {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			cutoff := time.Now().UTC().Add(-maxAge)
			cur, err := a.col().Find(ctx, bson.M{
				"provider": "local", "status": "orphan",
				"createdAt": bson.M{"$lt": cutoff},
			})
			if err == nil {
				docs := []AssetDoc{}
				_ = cur.All(ctx, &docs)
				for _, d := range docs {
					_ = a.Delete(ctx, d.Key)
				}
				if len(docs) > 0 {
					log.Printf("[assets] GC reclaimed %d orphan(s)", len(docs))
				}
			}
			cancel()
			time.Sleep(time.Hour)
		}
	}()
}
