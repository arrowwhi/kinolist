// Isolated UI test data. This server is never part of the production bundle.
import http from 'node:http'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const dist = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../dist')
const genres = [{ id: 18, name_ru: 'Драма', name_en: 'Drama' }, { id: 878, name_ru: 'Фантастика', name_en: 'Science Fiction' }]
const movies = ['Дюна: Часть вторая', 'Прошлые жизни', 'Идеальные дни'].map((title, i) => ({ id: i + 1, title, original_title: ['Dune: Part Two', 'Past Lives', 'Perfect Days'][i], year: i ? 2023 : 2024, overview: ['Пол Атрейдес объединяется с Чани и фременами, чтобы отомстить заговорщикам, уничтожившим его семью.', 'Друзья детства встречаются спустя годы и размышляют о выборе и несбывшейся жизни.', 'Хираяма находит красоту в простых моментах повседневной жизни в Токио.'][i], runtime_min: i ? 124 : 166, rating: i ? 7.9 : 8.5, rating_provider: 'tmdb', rating_vote_count: 145000, genres: [genres[i ? 0 : 1]] }))
let items = [...movies, ...movies].map((movie, i) => ({ id: i + 1, movie, status: i < 3 ? 'want_to_watch' : 'watched', user_rating: i < 3 ? 0 : 9, note: i % 3 ? '' : 'Посмотреть на большом экране. Саша очень рекомендовал.', review: i < 3 ? '' : 'Красивое кино, к которому хочется вернуться.', source: 'manual', added_at: '2026-11-05T10:00:00Z', watched_at: i < 3 ? undefined : '2026-11-05T11:00:00Z' }))
const jobs = new Map()
const send = (res, code, body) => { res.writeHead(code, { 'Content-Type': 'application/json' }); res.end(body === undefined ? undefined : JSON.stringify(body)) }
http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, 'http://localhost'), p = url.pathname.replace(/\/$/, '')
    if (!p.startsWith('/api/')) {
      const filename = path.resolve(dist, p.slice(1) || 'index.html')
      if (!filename.startsWith(dist + '/')) return send(res, 403, {})
      const file = await fs.readFile(filename).catch(() => fs.readFile(path.join(dist, 'index.html')))
      res.writeHead(200, { 'Content-Type': ({ '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff' })[path.extname(filename)] || 'text/html' }); res.end(url.searchParams.get('theme') === 'light' && filename.endsWith('index.html') ? file.toString().replace('<html', '<html data-theme="light"') : file); return
    }
    let body = ''
    for await (const chunk of req) body += chunk
    body = JSON.parse(body || '{}')
    if (p === '/api/v1/genres') return send(res, 200, { items: url.searchParams.has('scope') ? genres.map(g => ({ ...g, want_to_watch_count: items.filter(x => x.status === 'want_to_watch' && x.movie.genres.some(y => y.id === g.id)).length, watched_count: items.filter(x => x.status === 'watched' && x.movie.genres.some(y => y.id === g.id)).length })) : genres })
    if (p === '/api/v1/movies') {
      const query = (url.searchParams.get('q') || '').toLowerCase()
      if (query === 'ошибка') return send(res, 503, { code: 'test', message: 'Не удалось загрузить коллекцию' })
      const ids = (url.searchParams.get('genre_ids') || '').split(',').filter(Boolean).map(Number)
      const filtered = items.filter(x => (!url.searchParams.get('status') || x.status === url.searchParams.get('status')) && (!ids.length || x.movie.genres.some(y => ids.includes(y.id))))
      const rated = filtered.filter(x => x.status === 'watched' && x.user_rating)
      const summary = { movie_count: filtered.length, rated_count: rated.length, average_user_rating: rated.length ? rated.reduce((s, x) => s + x.user_rating, 0) / rated.length : null }
      const found = filtered.filter(x => `${x.movie.title} ${x.movie.original_title}`.toLowerCase().includes(query))
      const offset = Number(url.searchParams.get('offset') || 0), limit = Number(url.searchParams.get('limit') || 30)
      return send(res, 200, { items: found.slice(offset, offset + limit), total: found.length, offset, limit, summary })
    }
    const match = p.match(/^\/api\/v1\/movies\/(\d+)$/)
    if (match) {
      const item = items.find(x => x.id === +match[1])
      if (!item) return send(res, 404, { code: 'movie_not_found', message: 'Фильм не найден' })
      if (req.method === 'DELETE') { items = items.filter(x => x !== item); return send(res, 204) }
      if (req.method === 'PATCH') { Object.assign(item, body); if (body.status === 'watched') item.watched_at ||= new Date().toISOString(); if (body.status === 'want_to_watch') delete item.watched_at }
      return send(res, 200, item)
    }
    if (p === '/api/v1/jobs') { const job = { id: jobs.size + 1, kind: 'title', status: 'confirming', query_title: body.input, candidates: [{ ...movies[2], provider: 'tmdb', external_id: 999 }] }; jobs.set(job.id, job); return send(res, 202, job) }
    const jm = p.match(/^\/api\/v1\/jobs\/(\d+)(.*)$/)
    if (jm) {
      const job = jobs.get(+jm[1]); if (!job) return send(res, 404, { code: 'job_not_found', message: 'Задача не найдена' })
      if (jm[2].startsWith('/candidates')) return send(res, 200, { ...movies[2], id: 0 })
      if (jm[2] === '/reject') { job.status = 'cancelled'; return send(res, 204) }
      if (jm[2] === '/confirm') { const item = { id: Math.max(...items.map(x => x.id)) + 1, movie: movies[2], status: body.status || 'want_to_watch', user_rating: body.user_rating || 0, review: body.review || '', note: '', source: 'manual', added_at: new Date().toISOString(), watched_at: body.status === 'watched' ? new Date().toISOString() : undefined }; items.push(item); job.status = 'done'; return send(res, 201, item) }
      return send(res, 200, job)
    }
    send(res, 404, { code: 'not_found', message: 'Не найдено' })
  } catch (error) { send(res, 500, { code: 'fixture', message: String(error) }) }
}).listen(4173, '127.0.0.1', () => console.log('UI fixtures: http://127.0.0.1:4173 (no production data)'))
