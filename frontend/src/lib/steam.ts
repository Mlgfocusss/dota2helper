const STEAM_ID64_BASE = 76561197960265728n

/** SteamID64 по account_id из Dota (число больше 2^53, поэтому BigInt). */
export function steamId64FromAccountId(accountId: number): string {
  return (STEAM_ID64_BASE + BigInt(accountId)).toString()
}

export function steamProfileUrl(accountId: number): string {
  return `https://steamcommunity.com/profiles/${steamId64FromAccountId(accountId)}`
}
