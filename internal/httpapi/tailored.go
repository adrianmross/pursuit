package httpapi

import (
	"context"
	"net/http"

	"github.com/pursuit/pursuit/internal/domain"
)

func (s *Server) useDocument(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			DocumentID string `json:"document_id"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		if in.DocumentID == "" {
			return nil, domain.Validation("document_id is required").WithHint("pursuit documents list")
		}
		return s.Svc.UseDocument(ctx, r.PathValue("id"), in.DocumentID)
	})
}

func (s *Server) compareDocuments(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, "kind")
	if err != nil {
		writeErr(w, err)
		return
	}
	kind := "resume_pdf"
	if v := q["kind"]; len(v) > 0 {
		kind = v[0]
	}
	out, err := s.Svc.Compare(r.Context(), r.PathValue("id"), kind)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}
