package httpapi

import (
	"net/http"
	"strconv"

	"github.com/pursuit/pursuit/internal/domain"
)

// similarAnswers is smart search: saved answers, rounds and documents close
// in meaning to q. Not /v1/search: that name already means finding applications.
func (s *Server) similarAnswers(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, "q", "limit")
	if err != nil {
		writeErr(w, err)
		return
	}
	limit := 0
	if v := q["limit"]; len(v) > 0 {
		if limit, err = strconv.Atoi(v[0]); err != nil || limit < 1 {
			writeErr(w, domain.Validation("limit must be a positive number"))
			return
		}
	}
	text := ""
	if v := q["q"]; len(v) > 0 {
		text = v[0]
	}
	out, err := s.Svc.Similar(r.Context(), text, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
