import { describe, expect, it } from 'vitest'
import { isTabRoot, parentPath, tabForPath } from './routes'

describe('tabForPath', () => {
  it.each([
    ['/', 'home'],
    ['/news', 'home'],
    ['/news/123', 'home'],
    ['/feedback', 'home'],
    ['/play', 'play'],
    ['/play/quiz/hero', 'play'],
    ['/coach', 'coach'],
    ['/coach/matches/7', 'coach'],
    ['/coach/deep', 'reviews'],
    ['/coach/analysis/example', 'reviews'],
    ['/reviews', 'reviews'],
    ['/tools/drafter/top', 'tools'],
    ['/profile/party-card', 'profile'],
    ['/playground', 'home'],
    ['/unknown', 'home'],
  ])('%s → %s', (path, tab) => {
    expect(tabForPath(path)).toBe(tab)
  })
})

describe('isTabRoot', () => {
  it('только корни вкладок', () => {
    expect(isTabRoot('/')).toBe(true)
    expect(isTabRoot('/coach/')).toBe(true)
    expect(isTabRoot('/tools')).toBe(true)
    expect(isTabRoot('/news')).toBe(false)
    expect(isTabRoot('/reviews')).toBe(true)
    expect(isTabRoot('/coach/deep')).toBe(false)
  })
})

describe('parentPath', () => {
  it.each([
    ['/news', '/'],
    ['/news/42', '/news'],
    ['/feedback', '/'],
    ['/coach/deep', '/reviews'],
    ['/coach/matches/7', '/coach'],
    ['/coach/analysis/example', '/reviews'],
    ['/play/quiz/position', '/play'],
    ['/play/party', '/play'],
    ['/tools/drafter/history', '/tools/drafter'],
    ['/tools/heroes/1', '/tools/heroes'],
    ['/profile/party-card', '/profile'],
    ['/dev/ui', '/'],
  ])('%s → %s', (path, parent) => {
    expect(parentPath(path)).toBe(parent)
  })
})
