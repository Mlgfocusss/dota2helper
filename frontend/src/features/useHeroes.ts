import { useQuery } from '@tanstack/react-query'
import { getHeroes, getPatch } from '@/api/endpoints/heroes'
import { keys, STALE } from '@/api/keys'
import type { Hero } from '@/api/types'

export type HeroesData = {
  /** Все герои по алфавиту */
  list: Hero[]
  byId: Map<number, Hero>
}

// select вне компонента — TanStack Query не пересобирает Map на каждый рендер
const toHeroesData = (list: Hero[]): HeroesData => ({
  list,
  byId: new Map(list.map((hero) => [hero.id, hero])),
})

/** Справочник героев: грузится один раз за сессию. */
export function useHeroes() {
  return useQuery({
    queryKey: keys.heroes,
    queryFn: ({ signal }) => getHeroes(signal),
    staleTime: STALE.constants,
    gcTime: Infinity,
    select: toHeroesData,
  })
}

/** Герой по id; undefined, пока справочник грузится или id неизвестен. */
export function useHero(id: number | null | undefined): Hero | undefined {
  const { data } = useHeroes()
  return id == null ? undefined : data?.byId.get(id)
}

/** Текущий патч: номер и метка для интерфейса («7.41F»). */
export function usePatch() {
  return useQuery({
    queryKey: keys.patch,
    queryFn: ({ signal }) => getPatch(signal),
    staleTime: STALE.meta,
  })
}
