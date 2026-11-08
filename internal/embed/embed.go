// Package embed turns text into vectors through Vercel AI Gateway's
// OpenAI-compatible /embeddings endpoint, and compares them.
package embed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	Model      = "openai/text-embedding-3-small"
	Dimensions = 512
	// Batch is the most inputs sent in one request.
	Batch = 64
)

// Embedder turns texts into unit-length vectors, one per text, in order.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Gateway is AI Gateway with an API key.
type Gateway struct {
	Key  string
	HTTP *http.Client
	// Base is the API root; tests point it at an httptest server.
	Base string
}

func (g *Gateway) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += Batch {
		end := min(start+Batch, len(texts))
		vs, err := g.batch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vs...)
	}
	return out, nil
}

func (g *Gateway) batch(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": Model, "input": texts, "dimensions": Dimensions})
	base := g.Base
	if base == "" {
		base = "https://ai-gateway.vercel.sh/v1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.Key)
	req.Header.Set("Content-Type", "application/json")
	hc := g.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings: %d %s", res.StatusCode, bytes.TrimSpace(raw[:min(len(raw), 300)]))
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(out.Data), len(texts))
	}
	vs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vs) {
			return nil, fmt.Errorf("embeddings: index %d out of range", d.Index)
		}
		vs[d.Index] = Normalize(d.Embedding)
	}
	return vs, nil
}

// Normalize scales v to unit length, so a dot product is the cosine.
func Normalize(v []float32) []float32 {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return v
	}
	s := float32(1 / math.Sqrt(n))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * s
	}
	return out
}

// Dot is the cosine of two unit vectors.
func Dot(a, b []float32) float32 {
	var s float32
	for i := range min(len(a), len(b)) {
		s += a[i] * b[i]
	}
	return s
}

// Encode is a vector as base64 of little-endian float32s, for a TEXT column.
func Encode(v []float32) string {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return base64.StdEncoding.EncodeToString(b)
}

func Decode(s string) ([]float32, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b)%4 != 0 {
		return nil, fmt.Errorf("bad vector")
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v, nil
}
