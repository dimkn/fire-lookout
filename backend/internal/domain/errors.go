package domain

import "errors"

// ErrNotFound reports that a requested entity does not exist. Adapters wrap it so
// callers can match with errors.Is; the HTTP adapter turns it into a 404.
var ErrNotFound = errors.New("not found")
