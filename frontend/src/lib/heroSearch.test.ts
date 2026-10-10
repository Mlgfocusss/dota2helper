import { describe, expect, it } from 'vitest'
import type { Hero } from '@/api/types'
import { filterHeroes, moveHighlight, splitMatch } from './heroSearch'

const hero = (id: number, name: string, short: string, attr: Hero['primary_attr']): Hero => ({
  id,
  name: `npc_dota_hero_${short}`,
  localized_name: name,
  short_name: short,
  primary_attr: attr,
  roles: [],
  img: '',
  icon: '',
})

const heroes = [
  hero(1, 'Anti-Mage', 'antimage', 'agi'),
  hero(2, 'Axe', 'axe', 'str'),
  hero(3, 'Nature’s Prophet', 'furion', 'int'),
  hero(4, 'Shadow Fiend', 'nevermore', 'agi'),
]

describe('filterHeroes', () => {
  it('ищет по названию и по внутреннему имени, без учёта регистра и знаков', () => {
    expect(filterHeroes(heroes, 'anti mage', null).map((h) => h.id)).toEqual([1])
    expect(filterHeroes(heroes, 'NEVERMORE', null).map((h) => h.id)).toEqual([4])
    expect(filterHeroes(heroes, 'natures', null).map((h) => h.id)).toEqual([3])
  })

  it('фильтрует по атрибуту и сохраняет порядок', () => {
    expect(filterHeroes(heroes, '', 'agi').map((h) => h.id)).toEqual([1, 4])
    expect(filterHeroes(heroes, 'a', 'str').map((h) => h.id)).toEqual([2])
    expect(filterHeroes(heroes, '', null)).toHaveLength(4)
  })
})

describe('moveHighlight', () => {
  it('двигает по сетке и не выходит за границы', () => {
    expect(moveHighlight(0, 'ArrowRight', 10, 4)).toBe(1)
    expect(moveHighlight(5, 'ArrowDown', 10, 4)).toBe(9)
    expect(moveHighlight(6, 'ArrowDown', 10, 4)).toBeNull()
    expect(moveHighlight(0, 'ArrowLeft', 10, 4)).toBeNull()
    expect(moveHighlight(2, 'ArrowUp', 10, 4)).toBeNull()
    expect(moveHighlight(0, 'a', 10, 4)).toBeNull()
    expect(moveHighlight(0, 'ArrowRight', 0, 4)).toBeNull()
  })
})

describe('splitMatch', () => {
  it('выделяет совпадение без учёта регистра', () => {
    expect(splitMatch('Anti-Mage', 'mag')).toEqual(['Anti-', 'Mag', 'e'])
    expect(splitMatch('Axe', 'AX')).toEqual(['', 'Ax', 'e'])
  })

  it('без совпадения или с пустым запросом возвращает название целиком', () => {
    expect(splitMatch('Nature’s Prophet', 'natures')).toEqual(['Nature’s Prophet', '', ''])
    expect(splitMatch('Axe', '  ')).toEqual(['Axe', '', ''])
  })
})
