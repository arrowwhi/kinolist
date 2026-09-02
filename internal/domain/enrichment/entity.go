package enrichment

import (
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// Kind — что именно прислал пользователь.
type Kind string

const (
	KindReel  Kind = "reel"
	KindTitle Kind = "title"
)

// Status — состояние задачи обогащения.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusConfirming Status = "confirming"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Job — единица работы: превратить ссылку или название в фильм в коллекции.
type Job struct {
	ID         int64
	UserID     int64
	Kind       Kind
	Status     Status
	SourceURL  string
	RawTitle   string
	QueryTitle string
	QueryYear  int16
	Candidates []movie.Candidate
	Attempts   int16
	LastError  string
	ChatID     int64
	RunAfter   time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ReelContent — то, что удалось достать по ссылке на рилс.
type ReelContent struct {
	Caption   string
	FrameURLs []string
	Author    string
}

// ExtractedTitle — результат работы распознавателя названия.
type ExtractedTitle struct {
	Title      string
	Year       int16
	Confidence float32
}

func (e ExtractedTitle) Recognized() bool { return e.Title != "" }

// SessionState — стадия диалога с ботом.
type SessionState string

const (
	SessionIdle          SessionState = "idle"
	SessionAwaitingTitle SessionState = "awaiting_title"
	SessionAwaitingPick  SessionState = "awaiting_confirmation"
)

// Session хранится в БД, а не в памяти: рестарт сервиса не должен
// ронять висящие подтверждения.
type Session struct {
	UserID    int64
	State     SessionState
	JobID     int64
	ChatID    int64
	MessageID int
	UpdatedAt time.Time
}

// Outcome — что воркер сообщает пользователю по завершении задачи.
type Outcome struct {
	Job        Job
	Kind       movie.MatchKind
	Candidates []movie.Candidate
	Err        error
}
