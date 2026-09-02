package enrichment

import "errors"

var (
	ErrJobNotFound        = errors.New("enrichment job not found")
	ErrTitleNotRecognized = errors.New("movie title not recognized")
	ErrReelUnavailable    = errors.New("reel content unavailable")
	ErrInvalidReelURL     = errors.New("not a valid instagram reel url")
	ErrEmptyInput         = errors.New("empty input")
	ErrJobNotConfirmable  = errors.New("job is not awaiting confirmation")
	ErrCandidateNotInJob  = errors.New("candidate does not belong to this job")
	ErrExtractorDisabled  = errors.New("title extractor is not configured")
	ErrFetcherDisabled    = errors.New("reel fetcher is not configured")
)
