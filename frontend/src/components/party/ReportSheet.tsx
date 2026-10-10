import { useState } from 'react'
import { ApiError } from '@/api/client'
import type { PartyCard, PartyReportReason } from '@/api/types'
import { Button } from '@/components/ui/Button'
import { Sheet } from '@/components/ui/Sheet'
import { useReportCard } from '@/features/useParty'
import { toast } from '@/features/toast/store'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

type ReportSheetProps = {
  card: PartyCard | null
  onClose: () => void
}

const REASONS: PartyReportReason[] = ['spam', 'abuse', 'fake']

/** Окно жалобы: причина из трёх, отправка на POST /party/{id}/report. */
export function ReportSheet({ card, onClose }: ReportSheetProps) {
  return (
    <Sheet open={card !== null} onClose={onClose} title={t.party.reportTitle} width={440}>
      {/* форма монтируется заново для каждой анкеты — выбранная причина не «залипает» */}
      {card && <ReportForm key={card.user_id} card={card} onClose={onClose} />}
    </Sheet>
  )
}

function ReportForm({ card, onClose }: { card: PartyCard; onClose: () => void }) {
  const [reason, setReason] = useState<PartyReportReason | null>(null)
  const report = useReportCard()

  const submit = () => {
    if (!reason) return
    report.mutate(
      { userId: card.user_id, reason },
      {
        onSuccess: () => {
          toast.success(t.party.reportSent)
          onClose()
        },
        onError: (err) => {
          toast.error(err instanceof ApiError ? err.message : t.states.unknown)
          // повторная жалоба — это не ошибка, окно можно закрыть
          if (err instanceof ApiError && err.status === 409) onClose()
        },
      },
    )
  }

  return (
    <div>
      <p className="mb-4 text-[14px] text-muted">{t.party.reportHint}</p>
      <div role="radiogroup" aria-label={t.party.reportTitle} className="space-y-2">
        {REASONS.map((r) => (
          <button
            key={r}
            type="button"
            role="radio"
            aria-checked={reason === r}
            onClick={() => setReason(r)}
            className={cn(
              'flex w-full items-center gap-3 rounded-[12px] border px-4 py-3 text-left text-[15px] transition-colors',
              reason === r
                ? 'border-accent-2 bg-accent-2/15'
                : 'border-line bg-bg hover:bg-surface-2',
            )}
          >
            <span
              aria-hidden
              className={cn(
                'flex size-5 shrink-0 items-center justify-center rounded-full border',
                reason === r ? 'border-accent-2' : 'border-muted/50',
              )}
            >
              {reason === r && <span className="size-2.5 rounded-full bg-accent-2" />}
            </span>
            {t.party.reportReasons[r]}
          </button>
        ))}
      </div>
      <Button
        variant="accent"
        fullWidth
        className="mt-5"
        disabled={!reason}
        loading={report.isPending}
        onClick={submit}
      >
        {t.party.reportSend}
      </Button>
    </div>
  )
}
