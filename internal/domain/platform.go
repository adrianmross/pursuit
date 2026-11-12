package domain

import (
	"github.com/pursuit/pursuit/internal/platform/actor"
	"github.com/pursuit/pursuit/internal/platform/apperr"
	"github.com/pursuit/pursuit/internal/platform/paging"
)

// The app-agnostic types live in internal/platform. These aliases keep the
// domain.X names working for existing callers.

type (
	Actor    = actor.Actor
	ActorRef = actor.ActorRef

	Error = apperr.Error

	PageMeta    = paging.PageMeta
	Page[T any] = paging.Page[T]
	Cursor      = paging.Cursor
)

var (
	WithActor = actor.WithActor
	ActorFrom = actor.ActorFrom

	Validation          = apperr.Validation
	NotFound            = apperr.NotFound
	Conflict            = apperr.Conflict
	VersionConflict     = apperr.VersionConflict
	Forbidden           = apperr.Forbidden
	RateLimited         = apperr.RateLimited
	FeatureUnavailable  = apperr.FeatureUnavailable
	IdempotencyMismatch = apperr.IdempotencyMismatch
	IsCode              = apperr.IsCode
	As                  = apperr.As

	EncodeCursor    = paging.EncodeCursor
	DecodeCursor    = paging.DecodeCursor
	CheckCursorSort = paging.CheckCursorSort
	ValidateFields  = paging.ValidateFields
)
