// Отслеживаемый профиль гостя (SPEC 4.2) хранится в localStorage браузера.
// localStorage может быть недоступен (приватный режим, запрет сайтов) — тогда
// просто работаем без него.

const KEY = 'tracked_account_id'

export function getTrackedAccountId(): number | null {
  try {
    const n = Number(localStorage.getItem(KEY))
    return Number.isSafeInteger(n) && n > 0 ? n : null
  } catch {
    return null
  }
}

export function setTrackedAccountId(accountId: number) {
  try {
    localStorage.setItem(KEY, String(accountId))
  } catch {
    // нет доступа к хранилищу — профиль не запомнится между визитами
  }
}

export function clearTrackedAccountId() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // нечего очищать
  }
}
