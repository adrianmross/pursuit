package embed

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayBatchesAndOrders(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer k" || r.URL.Path != "/embeddings" {
			w.WriteHeader(401)
			return
		}
		var in struct {
			Model      string   `json:"model"`
			Input      []string `json:"input"`
			Dimensions int      `json:"dimensions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Model != Model || in.Dimensions != Dimensions || len(in.Input) > Batch {
			t.Errorf("request %+v", in)
		}
		// Answer out of order, with vectors that aren't unit length.
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []item
		for i := len(in.Input) - 1; i >= 0; i-- {
			data = append(data, item{i, []float32{float32(len(in.Input[i])), 0, 0}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	texts := make([]string, Batch+3)
	for i := range texts {
		texts[i] = string(make([]byte, i+1))
	}
	vs, err := (&Gateway{Key: "k", Base: srv.URL}).Embed(context.Background(), texts)
	if err != nil || len(vs) != len(texts) || calls != 2 {
		t.Fatalf("vs=%d calls=%d err=%v", len(vs), calls, err)
	}
	if vs[0][0] != 1 || vs[len(vs)-1][0] != 1 {
		t.Fatalf("not normalized or misordered: %v %v", vs[0], vs[len(vs)-1])
	}
	if _, err := (&Gateway{Key: "nope", Base: srv.URL}).Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("401 should fail")
	}
}

func TestEncodeDot(t *testing.T) {
	v := Normalize([]float32{3, 4})
	got, err := Decode(Encode(v))
	if err != nil || math.Abs(float64(Dot(got, v))-1) > 1e-6 {
		t.Fatalf("round trip %v %v", got, err)
	}
	if Dot(Normalize([]float32{1, 0}), Normalize([]float32{0, 1})) != 0 {
		t.Fatal("orthogonal")
	}
}
