package httpapi

import (
	"context"
	"io"
	"net/http"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
)

func (s *Server) listFeatures(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.ListFeatures(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": out})
}

func (s *Server) updateFeature(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in service.FeaturePatch
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.SetFeature(ctx, r.PathValue("id"), in)
	})
}

func (s *Server) getBilling(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.Billing(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) billingCheckout(w http.ResponseWriter, r *http.Request) {
	url, err := s.Svc.Checkout(r.Context(), s.Svc.BaseURL(publicBase(r)))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"url": url})
}

func (s *Server) billingPortal(w http.ResponseWriter, r *http.Request) {
	url, err := s.Svc.Portal(r.Context(), s.Svc.BaseURL(publicBase(r)))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"url": url})
}

func (s *Server) billingSync(w http.ResponseWriter, r *http.Request) {
	out, err := s.Svc.SyncMyBilling(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

// stripeWebhook is unauthenticated; the Stripe signature is the credential.
// It syncs before answering rather than in a goroutine: a Vercel function
// can be frozen once the response is written. The sync is two Stripe reads.
func (s *Server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, domain.Validation("read body: %s", err.Error()))
		return
	}
	if err := s.Svc.HandleStripeEvent(r.Context(), body, r.Header.Get("Stripe-Signature")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"received": true})
}
