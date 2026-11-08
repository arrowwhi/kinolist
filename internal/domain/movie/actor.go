package movie

import "strings"

// ActingDepartment — значение known_for_department у актёров в TMDB.
const ActingDepartment = "Acting"

// MaxCast — сколько актёров из начала титров хранится у работы.
const MaxCast = 10

// MinCollectionRoles — с какого числа работ коллекции актёр попадает в
// «Мои актёры» без подписки: иначе каждая работа приносила бы десяток людей.
const MinCollectionRoles = 2

// CastMember — участник актёрского состава работы. Как и у режиссёра,
// личность привязана к каталогу, а не к имени; ID равен 0, пока актёр
// не заведён в справочнике (предпросмотр, карточка каталога).
type CastMember struct {
	ID           int64
	TMDBID       int32
	KinopoiskID  int32
	Name         string
	OriginalName string
	// Character — персонаж; несколько — через « / », пусто, если неизвестен.
	Character   string
	ProfilePath string
}

// CharacterSeparator разделяет персонажей одной работы.
const CharacterSeparator = " / "

// JoinCharacters склеивает персонажей без пустых строк и повторов.
func JoinCharacters(names ...string) string {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		for _, part := range strings.Split(name, CharacterSeparator) {
			part = strings.TrimSpace(part)
			key := strings.ToLower(part)
			if part == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, part)
		}
	}
	return strings.Join(out, CharacterSeparator)
}

// Справочник актёров устроен так же, как справочник режиссёров: те же поля
// карточки, те же правила подсчёта и свежести. Отличаются таблицы и правило
// «из коллекции», поэтому типы общие.
type (
	// ActorWork — роль из фильмографии каталога; Character заполнен.
	ActorWork = DirectorWork
	// ActorOverview — карточка актёра в «Мои актёры»; счётчики по ролям.
	ActorOverview = DirectorOverview
	// ActorProfile — профиль актёра; у строк фильмографии есть Character.
	ActorProfile     = DirectorProfile
	ActorScope       = DirectorScope
	ActorScopeCounts = DirectorScopeCounts
	ActorSyncState   = DirectorSyncState
	MyActorsFilter   = MyDirectorsFilter
)
