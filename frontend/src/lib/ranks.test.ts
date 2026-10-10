import { describe, expect, it } from 'vitest'
import { parseRankTier } from './ranks'

describe('parseRankTier', () => {
  it('калибровка для пустого ранга', () => {
    expect(parseRankTier(null).label).toBe('Калибровка')
    expect(parseRankTier(undefined).medal).toBe(0)
    expect(parseRankTier(0).label).toBe('Калибровка')
  })

  it('медаль и звёзды', () => {
    expect(parseRankTier(51)).toEqual({ medal: 5, stars: 1, name: 'Легенда', label: 'Легенда 1' })
    expect(parseRankTier(15).label).toBe('Рекрут 5')
    expect(parseRankTier(75).label).toBe('Божество 5')
  })

  it('Титан без звёзд и с местом в лидерборде', () => {
    expect(parseRankTier(80).label).toBe('Титан')
    expect(parseRankTier(80, 123).label).toBe('Титан #123')
  })
})
