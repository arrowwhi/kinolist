package enrichment

import "context"

// JobRepository — хранилище задач обогащения и состояний диалога.
type JobRepository interface {
	Create(ctx context.Context, j Job) (Job, error)
	Get(ctx context.Context, id int64) (Job, error)
	GetForUser(ctx context.Context, userID, id int64) (Job, error)
	Update(ctx context.Context, j Job) error

	// Claim атомарно забирает до limit готовых задач и переводит их в
	// processing, чтобы несколько воркеров не взяли одну и ту же.
	Claim(ctx context.Context, limit int) ([]Job, error)

	GetSession(ctx context.Context, userID int64) (Session, error)
	SaveSession(ctx context.Context, s Session) error
	ClearSession(ctx context.Context, userID int64) error
}

// ReelFetcher достаёт содержимое рилса по публичной ссылке.
type ReelFetcher interface {
	Enabled() bool
	Fetch(ctx context.Context, reelURL string) (ReelContent, error)
}

// TitleExtractor вытаскивает название фильма из подписи и кадров.
// Кадры передаются уже сейчас, хотя текущая реализация читает только
// подпись: Vision-версия подключается без изменений домена и usecase.
type TitleExtractor interface {
	Enabled() bool
	Extract(ctx context.Context, content ReelContent) (ExtractedTitle, error)
}

// Notifier сообщает пользователю результат фоновой задачи.
// Реализуется транспортом Telegram; воркер о нём ничего не знает.
type Notifier interface {
	NotifyOutcome(ctx context.Context, out Outcome) error
}
