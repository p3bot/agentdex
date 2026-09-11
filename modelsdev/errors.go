package modelsdev

import "errors"

var (
	// ErrUnavailable is a cache miss or failed fetch with no usable catalog
	// (CacheOnly with no cache, Cached/Latest/TTL with no stale fallback).
	// Distinct from ErrModelsSchema: the endpoint was unusable, not malformed.
	ErrUnavailable = errors.New("models.dev catalog unavailable")

	// ErrModelsSchema signals that models.dev data does not match the expected
	// shape: empty top-level maps (gross drift, every fetch) or a requested
	// provider with a malformed model (per-model, in Provider and Models).
	// models.dev is unversioned community JSON, so validation is the only drift
	// signal; this error makes drift loud rather than silent blanks.
	// Model-resolution failures are the consuming layer's concern, not this package's.
	ErrModelsSchema = errors.New("models.dev schema unrecognised")
)
