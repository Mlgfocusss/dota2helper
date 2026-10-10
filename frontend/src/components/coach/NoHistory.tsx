import { useQueryClient } from '@tanstack/react-query'
import { RotateCw } from 'lucide-react'
import { keys } from '@/api/keys'
import type { CoachSummary } from '@/api/types'
import { Accordion } from '@/components/ui/Accordion'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { toast } from '@/features/toast/store'
import { useRefreshCoach } from '@/features/useCoach'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { parseRankTier } from '@/lib/ranks'

/** Пустое состояние «Не видим историю матчей»: инструкция и «Проверить снова». */
export function NoHistory({ summary }: { summary: CoachSummary }) {
  const queryClient = useQueryClient()
  const refresh = useRefreshCoach(summary.player.account_id)
  const { persona_name, avatar, rank_tier, leaderboard_rank } = summary.player
  const rank = rank_tier ? parseRankTier(rank_tier, leaderboard_rank).label : null

  const check = () =>
    refresh.mutate(undefined, {
      onSuccess: () => {
        const fresh = queryClient.getQueryData<CoachSummary>(
          keys.coachSummary(summary.player.account_id),
        )
        if (fresh?.has_history) toast.success(t.coach.noHistory.updated)
        else toast.info(t.coach.noHistory.stillEmpty)
      },
    })

  return (
    <div className="mx-auto max-w-[520px]">
      <div className="flex flex-col items-center text-center">
        <Avatar src={avatar} name={persona_name} size={72} radius={18} />
        <h2 className="mt-3 text-[20px] font-semibold">{persona_name || t.profile.noName}</h2>
        <p className="text-[14px] text-muted">{rank ?? t.coach.summary.refining}</p>
      </div>

      <h1 className="text-section-title mt-8 text-center">{t.coach.noHistory.title}</h1>
      <p className="mt-2 text-center text-muted">{t.coach.noHistory.text}</p>

      <Card padding="none" className="mt-5 p-5 text-center">
        <p className="text-[13px] text-muted">{t.coach.noHistory.settingPath}</p>
        <p className="mt-1 text-[17px] font-semibold">{t.coach.noHistory.settingName}</p>
        <p className="mt-1 text-[13px] text-muted">{t.coach.noHistory.settingState}</p>
      </Card>

      <Card padding="none" className="mt-3 px-5">
        <Accordion icon="plus" title={t.coach.noHistory.alreadyOn}>
          {t.coach.noHistory.alreadyOnText}
        </Accordion>
      </Card>

      <Button size="lg" fullWidth className="mt-5" disabled={refresh.isPending} onClick={check}>
        <RotateCw size={18} className={cn(refresh.isPending && 'animate-spin')} aria-hidden />
        {t.coach.noHistory.retry}
      </Button>
    </div>
  )
}
