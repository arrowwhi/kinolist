// Всё, что объясняет пользователю добавление по ссылке на рилс: сервис
// сам находит название в описании, если автор выделил его явно.
// Макеты — Figma, «Добавление из рилса», экраны 22–25.

import { ReelIcon } from '../../entities/movie/ReelIcon'
import { shortUrl } from '../../shared/lib/format'

// Примеры повторяют правила разбора на сервере (enrichment.CaptionTitles):
// кавычки, маркер «Фильм:» и год в скобках.
const PATTERNS: [string, string, string][] = [
  ['«', 'Дюна', '»'],
  ['Фильм: ', 'Дюна', ''],
  ['', 'Дюна', ' (2021)'],
]

/** Как автор рилса может подписать название, чтобы мы его узнали. */
export function CaptionPatterns() {
  return (
    <ul className="caption-patterns" aria-label="Примеры подписей">
      {PATTERNS.map(([before, title, after]) => (
        <li className="caption-pattern" key={before + after}>
          {before}
          <span className="caption-pattern__title">{title}</span>
          {after}
        </li>
      ))}
    </ul>
  )
}

/** Карточка-подсказка на экране добавления. */
export function ReelHint() {
  return (
    <section className="reel-hint" aria-label="Добавление по ссылке на рилс">
      <div className="reel-hint__head">
        <ReelIcon />
        <div>
          <h2 className="reel-hint__title">Нашёл кино в рилсе?</h2>
          <p className="muted reel-hint__text">
            Вставь ссылку — название найдём в описании
          </p>
        </div>
      </div>
      <p className="muted small">Быстрее всего узнаём так:</p>
      <CaptionPatterns />
    </section>
  )
}

/** Откуда взялся запрос: название, найденное в описании рилса. */
export function ReelSource({ title, year }: { title: string; year?: number }) {
  return (
    <div className="reel-source">
      <ReelIcon />
      <div>
        <p className="muted small">Нашли в описании рилса</p>
        <p className="reel-source__query">
          «<span className="reel-source__title">{title}</span>»
          {year ? ` · ${year}` : ''}
        </p>
      </div>
    </div>
  )
}

/** Компактная ссылка на рилс, который сейчас разбираем. */
export function ReelLink({ url }: { url: string }) {
  return (
    <span className="reel-link" title={url}>
      <ReelIcon size="sm" />
      <span className="reel-link__url">{shortUrl(url)}</span>
    </span>
  )
}
