import { describe, expect, it } from 'vitest'
import { steamId64FromAccountId, steamProfileUrl } from './steam'

describe('steamId64FromAccountId', () => {
  it('точно переводит account_id в SteamID64', () => {
    expect(steamId64FromAccountId(1214690881)).toBe('76561199174956609')
    expect(steamId64FromAccountId(22202)).toBe('76561197960287930')
  })

  it('ссылка на профиль', () => {
    expect(steamProfileUrl(1214690881)).toBe(
      'https://steamcommunity.com/profiles/76561199174956609',
    )
  })
})
