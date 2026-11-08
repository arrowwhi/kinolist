import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { SectionLink } from '../../features/navigation/SectionLink'
import { GlobalStatistics } from '../../features/collection/GlobalStatistics'
import { useResource } from '../../shared/hooks/useResource'
import { Button } from '../../shared/ui/Button'
import { greeting } from './model'
// Дубль главной для нового раздела режиссёров: ведёт на /v2/directors.
export function HomePageV2({
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
          description="Любимые авторы и их фильмография целиком."
          action="Открыть режиссёров"
          to="/v2/directors"
          navigate={navigate}
        />
        <SectionLink
          title="Актёры"
          description="Следи за ролями любимых актёров."
          action="Открыть актёров"
          to="/actors"
          navigate={navigate}
        />
      </section>
      <GlobalStatistics revision={revision} />
    </Shell>
  )
}
