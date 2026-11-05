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
})
const movies = [
  makeMovie(1, 'Дюна: Часть вторая'),
  makeMovie(2, 'Прошлые жизни'),
  makeMovie(3, 'Разделение', 'tv', 2),
  makeMovie(4, 'Сериал без сезонов', 'tv'),
]
movies[0].genres = [genres[1]]
movies[0].year = 2024
const catalog = [
  { ...movies[0], external_id: 1 },
  { ...movies[1], external_id: 2 },
  { ...movies[2], external_id: 7 },
  makeMovie(7, 'Разделение'),
  { ...movies[3], external_id: 4 },
  makeMovie(8, 'Тьма', 'tv', 3),
].map((m) => ({ ...m, external_id: m.external_id ?? m.id, provider: 'tmdb' }))
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
      if (p === '/api/v1/genres')
        return send(res, 200, {
          items: url.searchParams.has('scope')
            ? genres
                .map((g) => ({
                  ...g,
                  want_to_watch_count: typed.filter(
                    (x) =>
                      x.status === 'want_to_watch' &&
                      x.movie.genres.some((y) => y.id === g.id),
                  ).length,
                  watched_count: typed.filter(
                    (x) =>
                      x.status === 'watched' &&
                      x.movie.genres.some((y) => y.id === g.id),
                  ).length,
                }))
                .filter((g) => g.want_to_watch_count + g.watched_count > 0)
            : genres,
        })
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
        const filtered = typed.filter(
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
            ? send(res, 200, { ...c, id: 0 })
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
