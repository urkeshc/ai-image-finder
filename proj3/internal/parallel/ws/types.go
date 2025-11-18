package ws

import "proj3-redesigned/internal/meta"

// Task represents a unit of work: a photo to be scored against a query.
type Task struct {
	Photo meta.PhotoMetadata
	Query string
}
