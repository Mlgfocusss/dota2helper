import { t } from '@/i18n/ru'

/** «Рейтинговый», «Турбо»… По id режима из OpenDota. */
export function gameModeName(id: number): string {
  return t.coach.gameModes[id] ?? t.coach.gameModeUnknown
}

type Kda = { kills: number; deaths: number; assists: number }

/** «10 / 15 / 22» */
export function formatKda({ kills, deaths, assists }: Kda): string {
  return `${kills} / ${deaths} / ${assists}`
}

/** Картинка предмета на CDN Valve по короткому имени («blink»). */
export function itemImageUrl(key: string): string {
  return `https://cdn.cloudflare.steamstatic.com/apps/dota2/images/dota_react/items/${key}.png`
}
