package service

import (
	"context"

	"github.com/pursuit/pursuit/internal/domain"
)

// ProfileView is the applicant profile the browser extension autofills from.
// Answers are standing answers, with PURSUIT_PROFILE filling any key not set in the database.
type ProfileView struct {
	Answers map[string]string `json:"answers"`
}

func (s *Service) ApplicantProfile(ctx context.Context) (ProfileView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return ProfileView{}, err
	}
	answers, err := s.standingMap(ctx, a.UserID)
	if err != nil {
		return ProfileView{}, err
	}
	if answers == nil {
		answers = map[string]string{}
	}
	return ProfileView{Answers: answers}, nil
}

// CaptureResult is a save from the extension. Created is false when the posting URL was already tracked.
type CaptureResult struct {
	Application domain.Application `json:"application"`
	Created     bool               `json:"created"`
}

// CapturePosting saves a page the user is viewing as a to_apply application.
// The same posting URL returns the existing application instead of a second row.
func (s *Service) CapturePosting(ctx context.Context, in ApplicationInput) (CaptureResult, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return CaptureResult{}, err
	}
	raw, err := domain.CanonicalPostingURL(in.PostingURL)
	if err != nil {
		return CaptureResult{}, err
	}
	if raw == "" {
		return CaptureResult{}, domain.Validation("posting_url is required").WithHint("save the job from its posting page")
	}
	existing, err := s.DB.GetApplicationByPostingURL(ctx, a.UserID, raw)
	if err == nil {
		return CaptureResult{Application: existing, Created: false}, nil
	}
	if !domain.IsCode(err, "not_found") {
		return CaptureResult{}, err
	}
	in.PostingURL = raw
	in.Status = domain.StatusToApply
	app, err := s.CreateApplication(ctx, in)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{Application: app, Created: true}, nil
}

// ApplicationByPostingURL is the extension's "is this page already tracked?" lookup.
func (s *Service) ApplicationByPostingURL(ctx context.Context, raw string) (domain.Application, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Application{}, err
	}
	u, err := domain.CanonicalPostingURL(raw)
	if err != nil {
		return domain.Application{}, err
	}
	if u == "" {
		return domain.Application{}, domain.Validation("url is required").WithHint("pass the posting page URL")
	}
	return s.DB.GetApplicationByPostingURL(ctx, a.UserID, u)
}
