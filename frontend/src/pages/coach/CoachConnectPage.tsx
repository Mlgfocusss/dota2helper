import { Search } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '@/api/client'
import type { ResolvedProfile } from '@/api/types'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Field, Input } from '@/components/ui/Field'
import { Page, PageHeader } from '@/components/ui/Page'
import { toast } from '@/features/toast/store'
import { useResolveProfile, useSelectProfile } from '@/features/useCoach'
import { t } from '@/i18n/ru'

// Поиск профиля по ссылке (SPEC 5.2): найти игрока и сделать его отслеживаемым.
export default function CoachConnectPage() {
  const navigate = useNavigate()
  const [input, setInput] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [found, setFound] = useState<ResolvedProfile | null>(null)
  const resolve = useResolveProfile()
  const select = useSelectProfile()

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (resolve.isPending) return
    if (!input.trim()) {
      setError(t.coach.connect.emptyInput)
      return
    }
    setError(null)
    setFound(null)
    resolve.mutate(input.trim(), {
      onSuccess: setFound,
      onError: (err) => setError(err instanceof ApiError ? err.message : t.coach.connect.notFound),
    })
  }

  const confirm = () => {
    if (!found) return
    select.mutate(found.account_id, {
      onSuccess: () => {
        toast.success(t.coach.connect.saved)
        navigate('/coach')
      },
    })
  }

  return (
    <Page title={t.pages.coachConnect.title}>
      <PageHeader title={t.coach.connect.title} subtitle={t.coach.connect.subtitle} />
      <Card padding="lg">
        <form onSubmit={onSubmit} noValidate className="space-y-4">
          <Field
            label={t.coach.connect.label}
            htmlFor="profile-input"
            hint={t.coach.connect.hint}
            error={error ?? undefined}
          >
            <Input
              id="profile-input"
              value={input}
              maxLength={300}
              autoComplete="off"
              autoFocus
              spellCheck={false}
              aria-invalid={error ? true : undefined}
              placeholder={t.coach.connect.placeholder}
              onChange={(e) => {
                setInput(e.target.value)
                setError(null)
              }}
            />
          </Field>
          <Button type="submit" size="lg" fullWidth loading={resolve.isPending}>
            {!resolve.isPending && <Search size={18} aria-hidden />}
            {t.coach.connect.find}
          </Button>
        </form>
      </Card>

      {found && (
        <Card padding="none" className="mt-4 flex items-center gap-4 p-5">
          <Avatar src={found.avatar} name={found.persona_name} size={64} radius={14} />
          <div className="min-w-0 flex-1">
            <div className="truncate text-[18px] font-semibold">
              {found.persona_name || t.profile.noName}
            </div>
            <div className="font-mono text-[13px] text-muted">ID {found.account_id}</div>
          </div>
          <Button loading={select.isPending} onClick={confirm}>
            {t.coach.connect.itsMe}
          </Button>
        </Card>
      )}
    </Page>
  )
}
