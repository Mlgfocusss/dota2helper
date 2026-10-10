import { useEffect } from 'react'
import { useSearchParams } from 'react-router'
import { toast } from '@/features/toast/store'
import { clearTrackedAccountId } from '@/features/tracked'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'

/**
 * Общие реакции на вход через Steam:
 * - возврат с ?login=error|cancelled — уведомление и чистый адрес;
 * - после входа гостевой отслеживаемый профиль из localStorage больше не нужен:
 *   показываем профиль аккаунта (SPEC 4.1, п. 4).
 */
export function useAuthEffects() {
  const [params, setParams] = useSearchParams()
  const result = params.get('login')
  const { data: me } = useMe()
  const loggedIn = Boolean(me?.user)

  useEffect(() => {
    if (loggedIn) clearTrackedAccountId()
  }, [loggedIn])

  useEffect(() => {
    if (result !== 'error' && result !== 'cancelled') return
    if (result === 'error') toast.error(t.auth.loginError)
    else toast.info(t.auth.loginCancelled)
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.delete('login')
        return next
      },
      { replace: true, preventScrollReset: true },
    )
  }, [result, setParams])
}
