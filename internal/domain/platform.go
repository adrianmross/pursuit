package domain

import (
	"github.com/adrianmross/appkit/actor"
	"github.com/adrianmross/appkit/apperr"
	"github.com/adrianmross/appkit/paging"
	"github.com/adrianmross/appkit/session"
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

	Token = session.Token
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
