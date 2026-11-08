package tmdb

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// paramLanguage — язык ответа TMDB, общий для всех запросов клиента.
const paramLanguage = "language"

// SearchPeople ищет людей по имени. Режиссёры идут первыми, но остальные
// не отбрасываются: у многих режиссёров в TMDB основной департамент «Acting».
func (c *Client) SearchPeople(ctx context.Context, query string) ([]movie.Person, error) {
	return c.SearchPeopleByDepartment(ctx, query, movie.DirectingDepartment)
}

// SearchPeopleByDepartment ищет людей по имени и ставит первыми тех, чей
// known_for_department равен department; порядок TMDB внутри групп сохраняется.
func (c *Client) SearchPeopleByDepartment(ctx context.Context, query, department string) ([]movie.Person, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, movie.ErrEmptyQuery
	}
	params := url.Values{"query": {query}, paramLanguage: {c.language}, "include_adult": {"false"}}
	var resp personSearchResponse
	if err := c.get(ctx, "/search/person", params, &resp); err != nil {
		return nil, err
	}
	people := make([]movie.Person, 0, len(resp.Results))
	for _, r := range resp.Results {
		if p := r.toPerson(); p.TMDBID > 0 && p.Name != "" {
			people = append(people, p)
		}
	}
	sort.SliceStable(people, func(i, j int) bool {
		return people[i].KnownForDepartment == department && people[j].KnownForDepartment != department
	})
	return people, nil
}

// PersonDetails загружает человека, его режиссёрские работы, актёрские роли
// и переводы биографии одним запросом — общий для обоих разделов (и общий
// ключ кэша).
func (c *Client) PersonDetails(ctx context.Context, tmdbID int32) (movie.PersonDetails, error) {
	if tmdbID <= 0 {
		return movie.PersonDetails{}, movie.ErrInvalidPersonID
	}
	params := url.Values{paramLanguage: {c.language}, "append_to_response": {"combined_credits,translations"}}
	var resp personDetailsResponse
	if err := c.get(ctx, "/person/"+strconv.Itoa(int(tmdbID)), params, &resp); err != nil {
		if errors.Is(err, movie.ErrNotFound) {
			return movie.PersonDetails{}, movie.ErrPersonNotFound
		}
		return movie.PersonDetails{}, err
	}
	if resp.ID == 0 {
		return movie.PersonDetails{}, movie.ErrPersonNotFound
	}
	return resp.toDetails(time.Now()), nil
}

type personSearchResponse struct {
	Results []personResult `json:"results"`
}

type personResult struct {
	ID                 int32   `json:"id"`
	Name               string  `json:"name"`
	OriginalName       string  `json:"original_name"`
	KnownForDepartment string  `json:"known_for_department"`
	ProfilePath        string  `json:"profile_path"`
	Popularity         float32 `json:"popularity"`
	KnownFor           []struct {
		MediaType string `json:"media_type"`
		Title     string `json:"title"`
		Name      string `json:"name"`
	} `json:"known_for"`
}

func (r personResult) toPerson() movie.Person {
	name := strings.TrimSpace(r.Name)
	original := strings.TrimSpace(r.OriginalName)
	if original == "" {
		original = name
	}
	source := movie.NameSourceTMDB
	if movie.HasCyrillic(name) {
		source = movie.NameSourceTMDBTranslation
	}
	known := make([]string, 0, len(r.KnownFor))
	for _, k := range r.KnownFor {
		title := k.Title
		if k.MediaType == string(movie.MediaTV) || title == "" {
			title = k.Name
		}
		if title = strings.TrimSpace(title); title != "" {
			known = append(known, title)
		}
	}
	return movie.Person{
		TMDBID: r.ID, Name: name, OriginalName: original, NameSource: source,
		ProfilePath: r.ProfilePath, KnownForDepartment: r.KnownForDepartment,
		KnownFor: known, Popularity: r.Popularity,
	}
}

type personDetailsResponse struct {
	personResult
	AlsoKnownAs     []string `json:"also_known_as"`
	Biography       string   `json:"biography"`
	Birthday        string   `json:"birthday"`
	Deathday        string   `json:"deathday"`
	PlaceOfBirth    string   `json:"place_of_birth"`
	CombinedCredits *struct {
		Cast []personCredit `json:"cast"`
		Crew []personCredit `json:"crew"`
	} `json:"combined_credits"`
	Translations *struct {
		Translations []struct {
			Language string `json:"iso_639_1"`
			Data     struct {
				Name      string `json:"name"`
				Biography string `json:"biography"`
			} `json:"data"`
		} `json:"translations"`
	} `json:"translations"`
}

type personCredit struct {
	ID            int32           `json:"id"`
	MediaType     movie.MediaType `json:"media_type"`
	Job           string          `json:"job"`
	Title         string          `json:"title"`
	OriginalTitle string          `json:"original_title"`
	Name          string          `json:"name"`
	OriginalName  string          `json:"original_name"`
	ReleaseDate   string          `json:"release_date"`
	FirstAirDate  string          `json:"first_air_date"`
	PosterPath    string          `json:"poster_path"`
	VoteAverage   float64         `json:"vote_average"`
	VoteCount     int32           `json:"vote_count"`
	EpisodeCount  int32           `json:"episode_count"`
	Video         bool            `json:"video"`
	Character     string          `json:"character"`
	GenreIDs      []int32         `json:"genre_ids"`
}

func (d personDetailsResponse) toDetails(now time.Time) movie.PersonDetails {
	p := d.toPerson()
	// В профиле name — латиница из TMDB; оригинальным считаем именно её.
	p.OriginalName = strings.TrimSpace(d.Name)
	p.Name, p.NameSource = russianName(d)
	p.BirthDate = parseDate(d.Birthday)
	p.DeathDate = parseDate(d.Deathday)
	p.PlaceOfBirth = strings.TrimSpace(d.PlaceOfBirth)
	p.Biography, p.BiographyLang = biography(d)

	var works, roles []movie.DirectorWork
	if d.CombinedCredits != nil {
		works = directorWorks(d.CombinedCredits.Crew)
		roles = actorRoles(d.CombinedCredits.Cast)
	}
	p.KnownFor = knownFor(works, now)
	return movie.PersonDetails{Person: p, Works: works, Roles: roles}
}

// russianName: перевод TMDB, затем кириллица из also_known_as, иначе латиница.
// Кинопоиск, если доступен, уточняет имя уже в usecase.
func russianName(d personDetailsResponse) (string, movie.NameSource) {
	if d.Translations != nil {
		for _, t := range d.Translations.Translations {
			if name := strings.TrimSpace(t.Data.Name); t.Language == "ru" && movie.HasCyrillic(name) {
				return name, movie.NameSourceTMDBTranslation
			}
		}
	}
	if name := strings.TrimSpace(d.Name); movie.HasCyrillic(name) {
		return name, movie.NameSourceTMDBTranslation
	}
	for _, alias := range d.AlsoKnownAs {
		if alias = strings.TrimSpace(alias); movie.HasCyrillic(alias) {
			return alias, movie.NameSourceTMDBTranslation
		}
	}
	return strings.TrimSpace(d.Name), movie.NameSourceTMDB
}

func biography(d personDetailsResponse) (string, string) {
	byLang := map[string]string{}
	if d.Translations != nil {
		for _, t := range d.Translations.Translations {
			if text := strings.TrimSpace(t.Data.Biography); text != "" && byLang[t.Language] == "" {
				byLang[t.Language] = text
			}
		}
	}
	// Основная biography приходит на языке запроса (ru-RU) или пустой.
	if text := strings.TrimSpace(d.Biography); text != "" && byLang["ru"] == "" && movie.HasCyrillic(text) {
		byLang["ru"] = text
	}
	for _, lang := range []string{"ru", "en"} {
		if text := byLang[lang]; text != "" {
			return text, lang
		}
	}
	return "", ""
}

// directorWorks оставляет только режиссёрские работы: фильмы без видео-
// релизов и сериалы, схлопнутые до одной записи на шоу.
func directorWorks(crew []personCredit) []movie.DirectorWork {
	return collectWorks(crew, func(c personCredit) bool { return c.Job == "Director" })
}

// Жанры TMDB, роли в которых не считаются актёрскими: ток-шоу, новости,
// реалити.
var nonActingGenres = map[int32]bool{10767: true, 10763: true, 10764: true}

// actorRoles оставляет актёрские роли: без ток-шоу, новостей, реалити и
// появлений «в роли себя». Эпизоды сериала и несколько персонажей одной
// работы схлопываются в одну строку.
func actorRoles(cast []personCredit) []movie.ActorWork {
	return collectWorks(cast, func(c personCredit) bool {
		for _, g := range c.GenreIDs {
			if nonActingGenres[g] {
				return false
			}
		}
		return !playsSelf(c.Character)
	})
}

// playsSelf распознаёт «Self», «Himself - Host», «Herself (archive footage)».
func playsSelf(character string) bool {
	character = strings.ToLower(strings.TrimSpace(character))
	if strings.Contains(character, "самого себя") || strings.Contains(character, "саму себя") {
		return true
	}
	first, _, _ := strings.Cut(strings.TrimLeft(character, "( "), " ")
	first = strings.TrimRight(first, ",-–—:;()")
	switch first {
	case "self", "himself", "herself", "themselves", "themself":
		return true
	}
	return false
}

// collectWorks — общие для режиссёров и актёров правила: без видеорелизов,
// без безымянных и заброшенных проектов, одна строка на работу.
func collectWorks(credits []personCredit, accept func(personCredit) bool) []movie.DirectorWork {
	out := make([]movie.DirectorWork, 0)
	index := map[string]int{}
	for _, c := range credits {
		if c.Video || c.ID <= 0 || !c.MediaType.Valid() || !accept(c) {
			continue
		}
		title, original, date := c.Title, c.OriginalTitle, c.ReleaseDate
		if c.MediaType == movie.MediaTV {
			title, original, date = c.Name, c.OriginalName, c.FirstAirDate
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = strings.TrimSpace(original)
		}
		// Безымянные и никем не оценённые работы без даты — обычно мусор
		// вроде заявленных, но заброшенных проектов.
		if title == "" || (c.MediaType == movie.MediaMovie && date == "" && c.VoteCount == 0) {
			continue
		}
		key := string(c.MediaType) + ":" + strconv.Itoa(int(c.ID))
		if i, ok := index[key]; ok {
			if c.EpisodeCount > 0 && (out[i].EpisodeCount == nil || *out[i].EpisodeCount < c.EpisodeCount) {
				episodes := c.EpisodeCount
				out[i].EpisodeCount = &episodes
			}
			out[i].Character = movie.JoinCharacters(out[i].Character, c.Character)
			continue
		}
		w := movie.DirectorWork{
			MediaType: c.MediaType, TMDBID: c.ID, Title: title, OriginalTitle: strings.TrimSpace(original),
			ReleaseDate: parseDate(date), Year: parseYear(date), PosterPath: c.PosterPath, VoteCount: c.VoteCount,
			Character: movie.JoinCharacters(c.Character),
		}
		if c.VoteCount >= movie.MinVotesForRating && c.VoteAverage > 0 {
			rating := float64(int(c.VoteAverage*10+0.5)) / 10
			w.Rating = &rating
		}
		if c.MediaType == movie.MediaTV && c.EpisodeCount > 0 {
			episodes := c.EpisodeCount
			w.EpisodeCount = &episodes
		}
		index[key] = len(out)
		out = append(out, w)
	}
	return out
}

func knownFor(works []movie.DirectorWork, now time.Time) []string {
	rated := make([]movie.DirectorWork, 0, len(works))
	for _, w := range works {
		if w.Rating != nil && !w.Upcoming(now) {
			rated = append(rated, w)
		}
	}
	sort.SliceStable(rated, func(i, j int) bool { return *rated[i].Rating > *rated[j].Rating })
	out := make([]string, 0, 3)
	for i := 0; i < len(rated) && i < 3; i++ {
		out = append(out, rated[i].Title)
	}
	return out
}

func parseDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil
	}
	return &t
}
