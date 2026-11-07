import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { SectionLink } from '../../features/navigation/SectionLink'
import { GlobalStatistics } from '../../features/collection/GlobalStatistics'
import { useResource } from '../../shared/hooks/useResource'
import { Button } from '../../shared/ui/Button'
import { greeting } from './model'
export function HomePage({
  navigate,
  revision,
}: {
  navigate: Navigate
  revision: number
}) {
  const { data: user } = useResource((signal) => api.me(signal), 'me')
  return (
    <Shell
      title={greeting(user?.first_name)}
      subtitle={
        <>
          Все твои фильмы и сериалы —<br />в одном месте.
        </>
      }
      navigate={navigate}
      themeToggle
      floating={
        <Button className="button--pill" onClick={() => navigate('/add')}>
          ＋ Добавить
        </Button>
      }
    >
      <section className="home-sections" aria-label="Разделы коллекции">
        <h2 className="section-label">Твоя коллекция</h2>
        <SectionLink
          title="На потом"
          description="То, что хочется посмотреть."
          action="Выбрать историю"
          to="/collection/want_to_watch"
          navigate={navigate}
        />
        <SectionLink
          title="Просмотрено"
          description="Твои оценки, отзывы и впечатления."
          action="Вспомнить и оценить"
          to="/collection/watched"
          navigate={navigate}
        />
        <SectionLink
          title="Жанры"
          description="Фильмы и сериалы под настроение."
          action="Выбрать жанр"
          to="/genres"
          navigate={navigate}
        />
        <SectionLink
          title="Режиссёры"
          description="Авторы историй в твоей коллекции."
          action="Выбрать режиссёра"
          to="/directors"
          navigate={navigate}
        />
      </section>
      <GlobalStatistics revision={revision} />
    </Shell>
  )
}
