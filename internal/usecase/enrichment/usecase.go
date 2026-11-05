package enrichment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

//go:generate mockgen -source=usecase.go -destination=mocks/mocks.go -package=mocks

type (
	jobRepository interface {
		Create(ctx context.Context, j enrichment.Job) (enrichment.Job, error)
		Get(ctx context.Context, id int64) (enrichment.Job, error)
		GetForUser(ctx context.Context, userID, id int64) (enrichment.Job, error)
		Update(ctx context.Context, j enrichment.Job) error
		Claim(ctx context.Context, limit int) ([]enrichment.Job, error)
		SaveSession(ctx context.Context, s enrichment.Session) error
		ClearSession(ctx context.Context, userID int64) error
	}

	movieRepository interface {
		Upsert(ctx context.Context, m movie.Movie) (movie.Movie, error)
		Add(ctx context.Context, um movie.UserMovie) (movie.UserMovie, error)
		GenresByTMDBIDs(ctx context.Context, tmdbIDs []int32) ([]movie.Genre, error)
		GenresByExternalNames(ctx context.Context, p movie.Provider, names []string) ([]movie.Genre, error)
	}

	reelFetcher interface {
		Enabled() bool
		Fetch(ctx context.Context, reelURL string) (enrichment.ReelContent, error)
	}

	titleExtractor interface {
		Enabled() bool
		Extract(ctx context.Context, content enrichment.ReelContent) (enrichment.ExtractedTitle, error)
	}

	catalog interface {
		Provider() movie.Provider
		Search(ctx context.Context, title string, year int16) ([]movie.Candidate, error)
		Details(ctx context.Context, externalID int32) (movie.Candidate, error)
	}

	txManager interface {
		BeginTransaction(ctx context.Context) (context.Context, error)
		FinishTransaction(ctx context.Context, err *error)
	}
)

// UseCase владеет всем пайплайном: ссылка/название → кандидаты → фильм в коллекции.
type UseCase struct {
	jobs      jobRepository
	movies    movieRepository
	fetcher   reelFetcher
	extractor titleExtractor
	primary   catalog
	fallback  catalog
	tx        txManager

	maxAttempts  int
	retryDelay   time.Duration
	detailsMu    sync.Mutex
	detailsCache map[string]cachedDetails
}

type Deps struct {
	Jobs      jobRepository
	Movies    movieRepository
	Fetcher   reelFetcher
	Extractor titleExtractor
	// Primary — основной каталог (TMDB), Fallback — запасной (Кинопоиск),
	// он опционален и может быть nil.
	Primary  catalog
	Fallback catalog
	Tx       txManager

	MaxAttempts int
	RetryDelay  time.Duration
}

type cachedDetails struct {
	candidate movie.Candidate
	expires   time.Time
}

func New(d Deps) *UseCase {
	if d.MaxAttempts < 1 {
		d.MaxAttempts = 3
	}
	if d.RetryDelay <= 0 {
		d.RetryDelay = 30 * time.Second
	}

	return &UseCase{
		jobs:         d.Jobs,
		detailsCache: make(map[string]cachedDetails),
		movies:       d.Movies,
		fetcher:      d.Fetcher,
		extractor:    d.Extractor,
		primary:      d.Primary,
		fallback:     d.Fallback,
		tx:           d.Tx,
		maxAttempts:  d.MaxAttempts,
		retryDelay:   d.RetryDelay,
	}
}

// Enqueue разбирает пользовательский ввод и ставит задачу в очередь.
// Возвращает управление сразу: поиск занимает секунды и делается воркером.
func (uc *UseCase) Enqueue(ctx context.Context, userID, chatID int64, input string) (enrichment.Job, error) {
	kind, value, err := enrichment.ClassifyInput(input)
	if err != nil {
		return enrichment.Job{}, err
	}

	job := enrichment.Job{
		UserID:   userID,
		Kind:     kind,
		Status:   enrichment.StatusPending,
		ChatID:   chatID,
		RunAfter: time.Now(),
	}

	switch kind {
	case enrichment.KindReel:
		if !uc.fetcher.Enabled() {
			return enrichment.Job{}, enrichment.ErrFetcherDisabled
		}
		job.SourceURL = value
	case enrichment.KindTitle:
		job.RawTitle = value
		job.QueryTitle = value
	}

	created, err := uc.jobs.Create(ctx, job)
	if err != nil {
		return enrichment.Job{}, fmt.Errorf("create enrichment job: %w", err)
	}

	return created, nil
}

// Claim забирает готовые к обработке задачи для воркера.
func (uc *UseCase) Claim(ctx context.Context, limit int) ([]enrichment.Job, error) {
	jobs, err := uc.jobs.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("claim enrichment jobs: %w", err)
	}
	return jobs, nil
}

// Process выполняет одну задачу и возвращает то, что нужно сообщить пользователю.
func (uc *UseCase) Process(ctx context.Context, job enrichment.Job) (enrichment.Outcome, error) {
	match, err := uc.resolve(ctx, &job)
	if err != nil {
		return uc.fail(ctx, job, err)
	}

	if match.Kind == movie.MatchNone {
		return uc.fail(ctx, job, movie.ErrNoCandidates)
	}

	job.Status = enrichment.StatusConfirming
	job.Candidates = match.Candidates
	job.LastError = ""

	if err := uc.jobs.Update(ctx, job); err != nil {
		return enrichment.Outcome{}, fmt.Errorf("save confirming job: %w", err)
	}

	if job.ChatID != 0 {
		session := enrichment.Session{
			UserID: job.UserID,
			State:  enrichment.SessionAwaitingPick,
			JobID:  job.ID,
			ChatID: job.ChatID,
		}
		if err := uc.jobs.SaveSession(ctx, session); err != nil {
			return enrichment.Outcome{}, fmt.Errorf("save bot session: %w", err)
		}
	}

	return enrichment.Outcome{Job: job, Kind: match.Kind, Candidates: match.Candidates}, nil
}

// resolve доводит задачу от исходного ввода до списка кандидатов.
func (uc *UseCase) resolve(ctx context.Context, job *enrichment.Job) (movie.Match, error) {
	if job.Kind == enrichment.KindReel && job.QueryTitle == "" {
		if !uc.fetcher.Enabled() {
			return movie.Match{}, enrichment.ErrFetcherDisabled
		}
		if !uc.extractor.Enabled() {
			return movie.Match{}, enrichment.ErrExtractorDisabled
		}

		content, err := uc.fetcher.Fetch(ctx, job.SourceURL)
		if err != nil {
			return movie.Match{}, fmt.Errorf("fetch reel: %w", err)
		}

		extracted, err := uc.extractor.Extract(ctx, content)
		if err != nil {
			return movie.Match{}, fmt.Errorf("extract title: %w", err)
		}
		if !extracted.Recognized() {
			return movie.Match{}, enrichment.ErrTitleNotRecognized
		}

		job.RawTitle = content.Caption
		job.QueryTitle = extracted.Title
		job.QueryYear = extracted.Year
	}

	if job.QueryTitle == "" {
		return movie.Match{}, enrichment.ErrTitleNotRecognized
	}

	candidates, err := uc.search(ctx, job.QueryTitle, job.QueryYear)
	if err != nil {
		return movie.Match{}, err
	}

	return movie.SelectMatch(job.QueryTitle, job.QueryYear, candidates), nil
}

// search идёт в основной каталог, а при пустом результате — в запасной.
func (uc *UseCase) search(ctx context.Context, title string, year int16) ([]movie.Candidate, error) {
	candidates, primaryErr := uc.primary.Search(ctx, title, year)
	if primaryErr == nil && len(candidates) > 0 {
		return candidates, nil
	}

	if uc.fallback == nil {
		if primaryErr != nil {
			return nil, fmt.Errorf("search %s: %w", uc.primary.Provider(), primaryErr)
		}
		return nil, nil
	}

	fallbackCandidates, fallbackErr := uc.fallback.Search(ctx, title, year)
	if fallbackErr != nil {
		if primaryErr != nil {
			return nil, fmt.Errorf("search %s: %w", uc.primary.Provider(), primaryErr)
		}
		// Основной источник отработал и честно вернул пусто — это не ошибка.
		return nil, nil
	}

	return fallbackCandidates, nil
}

// Confirm сохраняет выбранного кандидата в коллекцию пользователя.
func (uc *UseCase) Confirm(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32) (movie.UserMovie, error) {
	return uc.ConfirmWithOptions(ctx, userID, jobID, provider, externalID, movie.ConfirmOptions{})
}

func (uc *UseCase) ConfirmWithOptions(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32, options movie.ConfirmOptions) (movie.UserMovie, error) {
	if err := options.Validate(); err != nil {
		return movie.UserMovie{}, err
	}
	if options.Status == "" {
		options.Status = movie.StatusWantToWatch
	}
	job, err := uc.jobs.GetForUser(ctx, userID, jobID)
	if err != nil {
		return movie.UserMovie{}, fmt.Errorf("get job: %w", err)
	}
	if job.Status != enrichment.StatusConfirming {
		return movie.UserMovie{}, enrichment.ErrJobNotConfirmable
	}

	chosen, ok := findCandidate(job.Candidates, provider, externalID)
	if !ok {
		return movie.UserMovie{}, enrichment.ErrCandidateNotInJob
	}

	// Resolve external metadata before opening the database transaction. Failure to
	// enrich must not prevent saving a candidate already found in the catalogue.
	detailed, detailErr := uc.details(ctx, chosen)
	if ctx.Err() != nil {
		return movie.UserMovie{}, ctx.Err()
	}
	if detailErr == nil {
		chosen = detailed
	}
	added, duplicate, err := uc.persist(ctx, userID, job, chosen, options)
	if err != nil {
		return movie.UserMovie{}, err
	}
	// Дубль — законный исход, а не сбой: задача закрыта и закоммичена,
	// вызывающему остаётся только сообщить пользователю.
	if duplicate {
		return movie.UserMovie{}, &movie.AlreadyInCollectionError{UserMovieID: added.ID}
	}

	return added, nil
}

// persist выполняет всю запись в одной транзакции. Признак дубля возвращается
// отдельно от ошибки: если отдать его через err, defer откатит транзакцию и
// задача останется висеть в confirming навсегда.
func (uc *UseCase) persist(
	ctx context.Context,
	userID int64,
	job enrichment.Job,
	chosen movie.Candidate,
	options movie.ConfirmOptions,
) (result movie.UserMovie, duplicate bool, err error) {
	ctx, err = uc.tx.BeginTransaction(ctx)
	if err != nil {
		return movie.UserMovie{}, false, err
	}
	defer uc.tx.FinishTransaction(ctx, &err)

	genres, err := uc.resolveGenres(ctx, chosen)
	if err != nil {
		return movie.UserMovie{}, false, err
	}

	stored, err := uc.movies.Upsert(ctx, toMovie(chosen, genres))
	if err != nil {
		return movie.UserMovie{}, false, fmt.Errorf("upsert movie: %w", err)
	}

	source := movie.SourceManual
	if job.Kind == enrichment.KindReel {
		source = movie.SourceReel
	}

	added, addErr := uc.movies.Add(ctx, movie.UserMovie{
		UserID:     userID,
		Movie:      stored,
		Status:     options.Status,
		UserRating: options.UserRating, Review: options.Review, Note: options.Note,
		Source:    source,
		SourceURL: job.SourceURL,
	})
	switch {
	case errors.Is(addErr, movie.ErrAlreadyInCollection):
		duplicate = true
		var existing *movie.AlreadyInCollectionError
		if errors.As(addErr, &existing) {
			result.ID = existing.UserMovieID
		}
	case addErr != nil:
		return movie.UserMovie{}, false, fmt.Errorf("add movie to collection: %w", addErr)
	default:
		result = added
	}

	if err = uc.closeJob(ctx, job, enrichment.StatusDone); err != nil {
		return movie.UserMovie{}, false, err
	}

	return result, duplicate, nil
}

// Reject отменяет задачу: пользователь сказал, что это не тот фильм.
func (uc *UseCase) Reject(ctx context.Context, userID, jobID int64) error {
	job, err := uc.jobs.GetForUser(ctx, userID, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	return uc.closeJob(ctx, job, enrichment.StatusCancelled)
}

func (uc *UseCase) JobForUser(ctx context.Context, userID, jobID int64) (enrichment.Job, error) {
	job, err := uc.jobs.GetForUser(ctx, userID, jobID)
	if err != nil {
		return enrichment.Job{}, fmt.Errorf("get job: %w", err)
	}
	return job, nil
}

func (uc *UseCase) closeJob(ctx context.Context, job enrichment.Job, status enrichment.Status) error {
	job.Status = status
	if err := uc.jobs.Update(ctx, job); err != nil {
		return fmt.Errorf("close job: %w", err)
	}
	if err := uc.jobs.ClearSession(ctx, job.UserID); err != nil {
		return fmt.Errorf("clear bot session: %w", err)
	}
	return nil
}

// fail переводит задачу в failed либо переставляет на повтор, если попытки
// ещё остались и ошибка похожа на временную.
func (uc *UseCase) fail(ctx context.Context, job enrichment.Job, cause error) (enrichment.Outcome, error) {
	job.Attempts++
	job.LastError = cause.Error()

	if retryable(cause) && int(job.Attempts) < uc.maxAttempts {
		job.Status = enrichment.StatusPending
		job.RunAfter = time.Now().Add(uc.retryDelay * time.Duration(job.Attempts))
		if err := uc.jobs.Update(ctx, job); err != nil {
			return enrichment.Outcome{}, fmt.Errorf("reschedule job: %w", err)
		}
		// Пользователя не тревожим: попытка ещё будет.
		return enrichment.Outcome{}, nil
	}

	job.Status = enrichment.StatusFailed
	if err := uc.jobs.Update(ctx, job); err != nil {
		return enrichment.Outcome{}, fmt.Errorf("fail job: %w", err)
	}
	if err := uc.jobs.ClearSession(ctx, job.UserID); err != nil {
		return enrichment.Outcome{}, fmt.Errorf("clear bot session: %w", err)
	}

	return enrichment.Outcome{Job: job, Kind: movie.MatchNone, Err: cause}, nil
}

// retryable отделяет временные сбои внешних сервисов от окончательных
// исходов: нераспознанное название повтором не исправится.
func retryable(err error) bool {
	switch {
	case errors.Is(err, enrichment.ErrTitleNotRecognized),
		errors.Is(err, enrichment.ErrInvalidReelURL),
		errors.Is(err, enrichment.ErrExtractorDisabled),
		errors.Is(err, enrichment.ErrFetcherDisabled),
		errors.Is(err, movie.ErrNoCandidates):
		return false
	default:
		return true
	}
}

func (uc *UseCase) resolveGenres(ctx context.Context, c movie.Candidate) ([]movie.Genre, error) {
	if len(c.GenreTMDBIDs) > 0 {
		genres, err := uc.movies.GenresByTMDBIDs(ctx, c.GenreTMDBIDs)
		if err != nil {
			return nil, fmt.Errorf("resolve genres by tmdb ids: %w", err)
		}
		return genres, nil
	}
	if len(c.GenreNames) > 0 {
		genres, err := uc.movies.GenresByExternalNames(ctx, c.Provider, c.GenreNames)
		if err != nil {
			return nil, fmt.Errorf("resolve genres by names: %w", err)
		}
		return genres, nil
	}
	return nil, nil
}

func toMovie(c movie.Candidate, genres []movie.Genre) movie.Movie {
	m := movie.Movie{
		Title:         c.Title,
		OriginalTitle: c.OriginalTitle,
		Year:          c.Year,
		Overview:      c.Overview,
		PosterPath:    c.PosterPath,
		Rating:        c.Rating,
		RuntimeMin:    c.RuntimeMin, RatingProvider: c.RatingProvider, RatingVoteCount: c.RatingVoteCount,
		Genres: genres,
	}
	if c.Provider == movie.ProviderKinopoisk {
		m.KinopoiskID = c.ExternalID
	} else {
		m.TMDBID = c.ExternalID
	}
	return m
}

func findCandidate(candidates []movie.Candidate, p movie.Provider, id int32) (movie.Candidate, bool) {
	for _, c := range candidates {
		if c.Provider == p && c.ExternalID == id {
			return c, true
		}
	}
	return movie.Candidate{}, false
}

// CandidateDetails is scoped to the authenticated user's job, not a public
// catalogue proxy. Never fetch a candidate not offered by that job.
func (uc *UseCase) CandidateDetails(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32) (movie.Movie, error) {
	job, err := uc.jobs.GetForUser(ctx, userID, jobID)
	if err != nil {
		return movie.Movie{}, err
	}
	if job.Status != enrichment.StatusConfirming {
		return movie.Movie{}, enrichment.ErrJobNotConfirmable
	}
	chosen, ok := findCandidate(job.Candidates, provider, externalID)
	if !ok {
		return movie.Movie{}, enrichment.ErrCandidateNotInJob
	}
	detailed, err := uc.details(ctx, chosen)
	if err != nil {
		return movie.Movie{}, err
	}
	genres, err := uc.resolveGenres(ctx, detailed)
	if err != nil {
		return movie.Movie{}, err
	}
	return toMovie(detailed, genres), nil
}

func (uc *UseCase) details(ctx context.Context, chosen movie.Candidate) (movie.Candidate, error) {
	key := fmt.Sprintf("%s:%d", chosen.Provider, chosen.ExternalID)
	uc.detailsMu.Lock()
	cached, ok := uc.detailsCache[key]
	uc.detailsMu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.candidate, nil
	}
	var provider catalog
	if uc.primary != nil && uc.primary.Provider() == chosen.Provider {
		provider = uc.primary
	}
	if provider == nil && uc.fallback != nil && uc.fallback.Provider() == chosen.Provider {
		provider = uc.fallback
	}
	if provider == nil {
		return movie.Candidate{}, movie.ErrProviderUnavailable
	}
	d, err := provider.Details(ctx, chosen.ExternalID)
	if err != nil {
		return movie.Candidate{}, err
	}
	if d.ExternalID != chosen.ExternalID || d.Provider != chosen.Provider {
		return movie.Candidate{}, movie.ErrNotFound
	}
	if d.Title == "" {
		d.Title = chosen.Title
	}
	if d.Overview == "" {
		d.Overview = chosen.Overview
	}
	if d.PosterPath == "" {
		d.PosterPath = chosen.PosterPath
	}
	if d.OriginalTitle == "" {
		d.OriginalTitle = chosen.OriginalTitle
	}
	if d.Year == 0 {
		d.Year = chosen.Year
	}
	if d.Rating == 0 {
		d.Rating = chosen.Rating
		d.RatingProvider = chosen.RatingProvider
		d.RatingVoteCount = chosen.RatingVoteCount
	}
	if len(d.GenreTMDBIDs) == 0 && len(d.GenreNames) == 0 {
		d.GenreTMDBIDs = chosen.GenreTMDBIDs
		d.GenreNames = chosen.GenreNames
	}
	if d.RatingProvider == "" && d.Rating != 0 {
		d.RatingProvider = d.Provider
	}
	uc.detailsMu.Lock()
	if len(uc.detailsCache) >= 256 {
		uc.detailsCache = make(map[string]cachedDetails)
	}
	uc.detailsCache[key] = cachedDetails{candidate: d, expires: time.Now().Add(15 * time.Minute)}
	uc.detailsMu.Unlock()
	return d, nil
}
