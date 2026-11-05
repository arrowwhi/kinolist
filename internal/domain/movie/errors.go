package movie

import "errors"

var (
	ErrDirectorNotFound      = errors.New("director not found")
	ErrInvalidDirectorFilter = errors.New("invalid director filter")
	ErrInvalidMediaType      = errors.New("invalid media type")
	ErrReviewTooLong         = errors.New("review exceeds 5000 characters")
	ErrNotFound              = errors.New("movie not found")
	ErrAlreadyInCollection   = errors.New("movie already in collection")
	ErrNoCandidates          = errors.New("no matching movies found")
	ErrInvalidStatus         = errors.New("invalid movie status")
	ErrInvalidRating         = errors.New("user rating must be between 1 and 10")
	ErrEmptyQuery            = errors.New("search query is empty")
	ErrProviderUnavailable   = errors.New("movie catalog provider unavailable")
)
