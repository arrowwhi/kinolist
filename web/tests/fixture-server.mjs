// Isolated contract fixtures, never included in the production bundle.
import http from 'node:http'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const dist = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../dist',
)
const genres = [
  { id: 18, name_ru: 'Драма', name_en: 'Drama' },
  { id: 878, name_ru: 'Фантастика', name_en: 'Science Fiction' },
]
const directors = [
  {
    id: 17,
    tmdb_id: 137427,
    name: 'Дени Вильнёв',
    original_name: 'Denis Villeneuve',
  },
  { id: 18, tmdb_id: 147, name: 'Бен Стиллер', original_name: 'Ben Stiller' },
  { id: 19, tmdb_id: 3998226, name: 'Селин Сон', original_name: 'Celine Song' },
  { id: 20, tmdb_id: 200, name: 'Вим Вендерс', original_name: 'Wim Wenders' },
  {
    id: 21,
    tmdb_id: 201,
    name: 'Аойф Макардл',
    original_name: 'Aoife McArdle',
  },
  { id: 22, tmdb_id: 202, name: 'Джессика Ли', original_name: 'Jessica Lee' },
  { id: 23, tmdb_id: 203, name: 'Джон Смит', original_name: 'John Smith' },
]
const makeMovie = (id, title, media_type = 'movie', season_count = null) => ({
  id,
  title,
  original_title: title,
  year: 2022,
  media_type,
  season_count,
  runtime_min: media_type === 'tv' ? undefined : 124,
  overview:
    media_type === 'tv'
      ? 'Сотрудники разделяют рабочие и личные воспоминания. Но идеальный баланс оказывается ловушкой.'
      : 'История о выборе, дружбе и неожиданных встречах.',
  rating: 8.7,
  rating_provider: 'tmdb',
  rating_vote_count: 20000,
  genres: [genres[0]],
  directors:
    media_type === 'tv'
      ? [directors[1], directors[4], directors[5], directors[6]]
      : [id === 1 ? directors[0] : directors[2]],
})
const movies = [
  makeMovie(1, 'Дюна: Часть вторая'),
  makeMovie(2, 'Прошлые жизни'),
  makeMovie(3, 'Разделение', 'tv', 2),
  makeMovie(4, 'Сериал без сезонов', 'tv'),
]
movies[0].genres = [genres[1]]
movies[0].year = 2024
movies[3].directors = []
movies[0].tmdb_id = 1
const catalog = [
  { ...movies[0], external_id: 1 },
  { ...movies[1], external_id: 2 },
  { ...movies[2], external_id: 7 },
  makeMovie(7, 'Разделение'),
  { ...movies[3], external_id: 4 },
  makeMovie(8, 'Тьма', 'tv', 3),
].map((m) => ({ ...m, external_id: m.external_id ?? m.id, provider: 'tmdb' }))
// Каталог вне коллекции: фильмография режиссёра и карточки /catalog.
const catalogTitles = [
  {
    ...makeMovie(0, 'Враг'),
    tmdb_id: 181886,
    original_title: 'Enemy',
    year: 2013,
    runtime_min: 91,
    rating: 6.9,
    rating_vote_count: 5123,
    release_date: '2013-09-08',
    upcoming: false,
    genres: [{ id: 53, name_ru: 'Триллер', name_en: 'Thriller' }],
    directors: [directors[0]],
  },
  {
    ...makeMovie(0, 'Дюна: Мессия'),
    tmdb_id: 1170608,
    original_title: 'Dune: Messiah',
    year: 2026,
    runtime_min: undefined,
    rating: undefined,
    rating_vote_count: 0,
    overview: '',
    release_date: '2026-12-18',
    upcoming: true,
    genres: [genres[1]],
    directors: [directors[0]],
  },
  { ...movies[0], id: 0, tmdb_id: 1, upcoming: false },
]
let items = [...movies, ...movies].map((movie, i) => ({
  id: i + 1,
  movie,
  status: i < 4 ? 'want_to_watch' : 'watched',
  user_rating: i < 4 ? 0 : [9, 8, 10, 0][i - 4],
  note: 'Посмотреть вечером. Без спойлеров!',
  review: i < 4 ? '' : 'Хочется обсудить финал.',
  source: 'manual',
  added_at: '2026-11-05T10:00:00Z',
  watched_at: i < 4 ? undefined : '2026-11-05T11:00:00Z',
}))
const jobs = new Map()
const send = (res, code, body) => {
  res.writeHead(code, { 'Content-Type': 'application/json' })
  res.end(body === undefined ? undefined : JSON.stringify(body))
}
http
  .createServer(async (req, res) => {
    try {
      const url = new URL(req.url, 'http://localhost'),
        p = url.pathname.replace(/\/$/, '')
      if (!p.startsWith('/api/')) {
        const filename = path.resolve(dist, p.slice(1) || 'index.html')
        if (!filename.startsWith(dist + '/')) return send(res, 403, {})
        const file = await fs
          .readFile(filename)
          .catch(() => fs.readFile(path.join(dist, 'index.html')))
        res.writeHead(200, {
          'Content-Type':
            {
              '.js': 'text/javascript',
              '.css': 'text/css',
              '.woff2': 'font/woff2',
              '.woff': 'font/woff',
            }[path.extname(filename)] || 'text/html',
          'Cache-Control': 'no-store',
        })
        res.end(
          url.searchParams.get('theme') === 'light' &&
            filename.endsWith('index.html')
            ? file.toString().replace('<html', '<html data-theme="light"')
            : file,
        )
        return
      }
      let raw = ''
      for await (const chunk of req) raw += chunk
      const body = JSON.parse(raw || '{}'),
        type = url.searchParams.get('media_type')
      if (type && !['movie', 'tv'].includes(type))
        return send(res, 400, { message: 'Недопустимый тип' })
      const typed = items.filter((x) => !type || x.movie.media_type === type)
      const fixture = new URL(
        req.headers.referer || 'http://localhost',
      ).searchParams.get('fixture')
      const selectedDirectors = (url.searchParams.get('director_ids') || '')
        .split(',')
        .filter(Boolean)
        .map(Number)
      const directed = typed.filter(
        (x) =>
          !selectedDirectors.length ||
          x.movie.directors?.some((d) => selectedDirectors.includes(d.id)),
      )
      if (p === '/api/v1/directors' || /^\/api\/v1\/directors\/\d+$/.test(p)) {
        if (fixture === 'error')
          return send(res, 503, { message: 'Не удалось загрузить режиссёров' })
        const stats = directors
          .filter((d) =>
            items.some((x) =>
              x.movie.directors?.some((person) => person.id === d.id),
            ),
          )
          .map((d) => {
            const works = typed.filter((x) =>
              x.movie.directors?.some((person) => person.id === d.id),
            )
            const rated = works.filter(
              (x) => x.status === 'watched' && x.user_rating,
            )
            return {
              ...d,
              total_count: works.length,
              want_to_watch_count: works.filter(
                (x) => x.status === 'want_to_watch',
              ).length,
              watched_count: works.filter((x) => x.status === 'watched').length,
              rated_count: rated.length,
              average_user_rating: rated.length
                ? Math.round(
                    (rated.reduce((a, x) => a + x.user_rating, 0) /
                      rated.length) *
                      10,
                  ) / 10
                : null,
            }
          })
        if (p !== '/api/v1/directors') {
          const d = stats.find((d) => d.id === Number(p.split('/').at(-1)))
          return d && fixture !== 'empty'
            ? send(res, 200, d)
            : send(res, 404, {
                code: 'director_not_found',
                message: 'Режиссёр не найден в коллекции',
              })
        }
        const q = (url.searchParams.get('q') || '').toLowerCase()
        let found =
          fixture === 'empty'
            ? []
            : stats.filter(
                (d) =>
                  d.total_count > 0 &&
                  `${d.name} ${d.original_name}`.toLowerCase().includes(q),
              )
        const sort = url.searchParams.get('sort') || 'name',
          direction = url.searchParams.get('order') === 'desc' ? -1 : 1
        found.sort((a, b) =>
          sort === 'name'
            ? direction * a.name.localeCompare(b.name, 'ru')
            : sort === 'work_count'
              ? direction * (a.total_count - b.total_count)
              : a.average_user_rating == null
                ? 1
                : b.average_user_rating == null
                  ? -1
                  : direction * (a.average_user_rating - b.average_user_rating),
        )
        const offset = Number(url.searchParams.get('offset') || 0),
          limit = Number(url.searchParams.get('limit') || 50)
        return send(res, 200, {
          items: found.slice(offset, offset + limit),
          total: found.length,
          limit,
          offset,
        })
      }
      if (p === '/api/v1/me') {
        const fixture = new URL(
          req.headers.referer || 'http://localhost',
        ).searchParams.get('fixture')
        return send(res, 200, {
          id: 1,
          first_name: fixture === 'empty' ? '' : 'Иван',
        })
      }
      if (p === '/api/v1/statistics') {
        const fixture = new URL(
          req.headers.referer || 'http://localhost',
        ).searchParams.get('fixture')
        if (fixture === 'error')
          return send(res, 503, {
            code: 'test',
            message: 'Не удалось загрузить статистику',
          })
        return send(
          res,
          200,
          fixture === 'empty'
            ? { total_count: 0, want_to_watch_count: 0, watched_count: 0 }
            : {
                total_count: items.length,
                want_to_watch_count: items.filter(
                  (x) => x.status === 'want_to_watch',
                ).length,
                watched_count: items.filter((x) => x.status === 'watched')
                  .length,
              },
        )
      }
      if (p === '/api/v1/genres')
        return send(res, 200, {
          items: url.searchParams.has('scope')
            ? genres
                .map((g) => ({
                  ...g,
                  want_to_watch_count: directed.filter(
                    (x) =>
                      x.status === 'want_to_watch' &&
                      x.movie.genres.some((y) => y.id === g.id),
                  ).length,
                  watched_count: directed.filter(
                    (x) =>
                      x.status === 'watched' &&
                      x.movie.genres.some((y) => y.id === g.id),
                  ).length,
                }))
                .filter((g) => g.want_to_watch_count + g.watched_count > 0)
            : genres,
        })
      const catalogMatch = p.match(
        /^\/api\/v1\/catalog\/(tmdb|kinopoisk)\/(movie|tv)\/(\d+)$/,
      )
      if (catalogMatch) {
        const title = catalogTitles.find(
          (m) =>
            catalogMatch[1] === 'tmdb' &&
            m.media_type === catalogMatch[2] &&
            m.tmdb_id === +catalogMatch[3],
        )
        if (!title)
          return send(res, 404, {
            code: 'not_found',
            message: 'Тайтл не найден в каталоге',
          })
        const owned = items.find((x) => x.movie.tmdb_id === title.tmdb_id)
        return send(
          res,
          200,
          owned
            ? {
                ...title,
                user_movie_id: owned.id,
                status: owned.status,
                user_rating: owned.user_rating || undefined,
              }
            : title,
        )
      }
      if (p === '/api/v1/movies' && req.method === 'POST') {
        const title = catalogTitles.find(
          (m) =>
            m.tmdb_id === body.external_id && m.media_type === body.media_type,
        )
        if (!title)
          return send(res, 404, {
            code: 'not_found',
            message: 'Тайтл не найден в каталоге',
          })
        const duplicate = items.find((x) => x.movie.tmdb_id === title.tmdb_id)
        if (duplicate)
          return send(res, 409, {
            code: 'already_in_collection',
            message: 'Уже в коллекции',
            existing_user_movie_id: duplicate.id,
          })
        const { upcoming, release_date, ...movie } = title
        const item = {
          id: Math.max(...items.map((x) => x.id)) + 1,
          movie: { ...movie, id: 100 + items.length },
          status: body.status || 'want_to_watch',
          user_rating: body.user_rating || 0,
          review: body.review || '',
          note: '',
          source: 'manual',
          added_at: new Date().toISOString(),
          watched_at:
            body.status === 'watched' ? new Date().toISOString() : undefined,
        }
        items.push(item)
        return send(res, 201, item)
      }
      if (p === '/api/v1/directors/17/profile') {
        const filmography = catalogTitles.map((m) => {
          const owned = items.find((x) => x.movie.tmdb_id === m.tmdb_id)
          return {
            tmdb_id: m.tmdb_id,
            media_type: m.media_type,
            title: m.title,
            original_title: m.original_title,
            year: m.year,
            release_date: m.release_date,
            rating: m.rating ?? null,
            vote_count: m.rating_vote_count ?? 0,
            upcoming: !!m.upcoming,
            user_movie_id: owned?.id,
            status: owned?.status,
            user_rating: owned?.user_rating || undefined,
          }
        })
        filmography.push({
          media_type: 'movie',
          movie_id: 900,
          title: 'Работа только из Кинопоиска',
          original_title: 'Работа только из Кинопоиска',
          year: 1998,
          rating: null,
          vote_count: 0,
          upcoming: false,
        })
        const count = (status) =>
          filmography.filter((w) => w.status === status).length
        return send(res, 200, {
          ...directors[0],
          known_for: ['Дюна', 'Прибытие'],
          followed: true,
          filmography_complete: true,
          work_count: filmography.length,
          in_collection_count: count('watched') + count('want_to_watch'),
          watched_count: count('watched'),
          want_to_watch_count: count('want_to_watch'),
          rated_count: 0,
          average_user_rating: null,
          biography: '',
          stale: false,
          filmography,
        })
      }
      if (p === '/api/v1/movies') {
        const query = (url.searchParams.get('q') || '').toLowerCase(),
          ids = (url.searchParams.get('genre_ids') || '')
            .split(',')
            .filter(Boolean)
            .map(Number)
        if (query === 'ошибка')
          return send(res, 503, {
            code: 'test',
            message: 'Не удалось загрузить коллекцию',
          })
        const filtered = directed.filter(
          (x) =>
            (!url.searchParams.get('status') ||
              x.status === url.searchParams.get('status')) &&
            (!ids.length || x.movie.genres.some((g) => ids.includes(g.id))),
        )
        const rated = filtered.filter(
            (x) => x.status === 'watched' && x.user_rating,
          ),
          summary = {
            movie_count: filtered.length,
            rated_count: rated.length,
            average_user_rating: rated.length
              ? rated.reduce((sum, x) => sum + x.user_rating, 0) / rated.length
              : null,
          }
        const found = filtered.filter((x) =>
            `${x.movie.title} ${x.movie.original_title}`
              .toLowerCase()
              .includes(query),
          ),
          offset = Number(url.searchParams.get('offset') || 0),
          limit = Number(url.searchParams.get('limit') || 30)
        return send(res, 200, {
          items: found.slice(offset, offset + limit),
          total: found.length,
          offset,
          limit,
          summary,
        })
      }
      const match = p.match(/^\/api\/v1\/movies\/(\d+)$/)
      if (match) {
        const item = items.find((x) => x.id === +match[1])
        if (!item)
          return send(res, 404, {
            code: 'movie_not_found',
            message: 'Запись не найдена',
          })
        if (req.method === 'DELETE') {
          items = items.filter((x) => x !== item)
          return send(res, 204)
        }
        if (req.method === 'PATCH') {
          Object.assign(item, body)
          if (body.status === 'watched')
            item.watched_at ||= new Date().toISOString()
          if (body.status === 'want_to_watch') delete item.watched_at
        }
        return send(res, 200, item)
      }
      if (p === '/api/v1/jobs') {
        const candidates = catalog.filter((m) =>
          m.title.toLowerCase().includes(body.input.toLowerCase()),
        )
        const job = {
          id: jobs.size + 1,
          kind: 'title',
          status: candidates.length ? 'confirming' : 'failed',
          query_title: body.input,
          candidates,
        }
        jobs.set(job.id, job)
        return send(res, 202, job)
      }
      const jm = p.match(/^\/api\/v1\/jobs\/(\d+)(.*)$/)
      if (jm) {
        const job = jobs.get(+jm[1])
        if (!job)
          return send(res, 404, {
            code: 'job_not_found',
            message: 'Задача не найдена',
          })
        const cm = jm[2].match(
          /^\/candidates\/(tmdb|kinopoisk)\/(movie|tv)\/(\d+)$/,
        )
        if (cm) {
          const c = job.candidates.find(
            (x) =>
              x.provider === cm[1] &&
              x.media_type === cm[2] &&
              x.external_id === +cm[3],
          )
          return c
            ? send(res, 200, {
                ...c,
                id: 0,
                directors: c.directors?.map(({ id, ...d }) => d),
              })
            : send(res, 400, { message: 'Кандидат не относится к задаче' })
        }
        if (jm[2] === '/reject') {
          job.status = 'cancelled'
          return send(res, 204)
        }
        if (jm[2] === '/confirm') {
          const c = job.candidates.find(
            (x) =>
              x.provider === body.provider &&
              x.media_type === body.media_type &&
              x.external_id === body.external_id,
          )
          if (!c)
            return send(res, 400, { message: 'Неверный тип или ID кандидата' })
          const duplicate = items.find(
            (x) =>
              x.movie.title === c.title && x.movie.media_type === c.media_type,
          )
          if (duplicate)
            return send(res, 409, {
              code: 'already_in_collection',
              message: 'Уже в коллекции',
              existing_user_movie_id: duplicate.id,
            })
          const item = {
            id: Math.max(...items.map((x) => x.id)) + 1,
            movie: c,
            status: body.status || 'want_to_watch',
            user_rating: body.user_rating || 0,
            review: body.review || '',
            note: '',
            source: 'manual',
            added_at: new Date().toISOString(),
            watched_at:
              body.status === 'watched' ? new Date().toISOString() : undefined,
          }
          items.push(item)
          job.status = 'done'
          return send(res, 201, item)
        }
        return send(res, 200, job)
      }
      send(res, 404, { code: 'not_found', message: 'Не найдено' })
    } catch (error) {
      send(res, 500, { code: 'fixture', message: String(error) })
    }
  })
  .listen(Number(process.env.UI_FIXTURE_PORT || 4173), '127.0.0.1', () =>
    console.log('UI fixtures ready (no production data)'),
  )
