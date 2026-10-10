import { Copy, ExternalLink, Send } from 'lucide-react'
import type { PartyCard } from '@/api/types'
import { SteamIcon } from '@/components/icons/SteamIcon'
import { Avatar } from '@/components/ui/Avatar'
import { ButtonAnchor, IconButton } from '@/components/ui/Button'
import { Sheet } from '@/components/ui/Sheet'
import { toast } from '@/features/toast/store'
import { t } from '@/i18n/ru'
import { telegramUrl } from '@/lib/party'
import { steamProfileUrl } from '@/lib/steam'

type ContactSheetProps = {
  card: PartyCard | null
  onClose: () => void
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t.party.copied)
  } catch {
    toast.error(t.party.copyFailed)
  }
}

/** Контакты автора анкеты. Показываются только вошедшим — весь поиск закрыт для гостей. */
export function ContactSheet({ card, onClose }: ContactSheetProps) {
  const name = card?.persona_name || t.profile.noName
  return (
    <Sheet open={card !== null} onClose={onClose} title={t.party.contactTitle(name)} width={460}>
      {card && (
        <div className="space-y-3">
          <div className="flex items-center gap-3 pb-1">
            <Avatar src={card.avatar} name={name} size={48} radius={14} />
            <p className="min-w-0 text-[14px] text-muted">{t.party.contactHint}</p>
          </div>

          <ButtonAnchor
            href={steamProfileUrl(card.account_id)}
            target="_blank"
            rel="noreferrer"
            variant="surface"
            fullWidth
            className="justify-start"
          >
            <SteamIcon size={20} />
            <span className="flex-1 text-left">{t.party.steamProfile}</span>
            <ExternalLink size={16} className="text-muted" aria-hidden />
          </ButtonAnchor>

          {card.discord && card.discord_tag && (
            <div className="flex items-center gap-3 rounded-[14px] bg-surface-2 py-1.5 pl-5 pr-2">
              <span className="caps shrink-0">{t.party.discordTag}</span>
              <span className="min-w-0 flex-1 select-all truncate font-mono text-[15px]">
                {card.discord_tag}
              </span>
              <IconButton
                aria-label={`${t.party.copy}: ${card.discord_tag}`}
                variant="ghost"
                onClick={() => void copyText(card.discord_tag ?? '')}
              >
                <Copy size={18} />
              </IconButton>
            </div>
          )}

          {card.telegram && (
            <ButtonAnchor
              href={telegramUrl(card.telegram)}
              target="_blank"
              rel="noreferrer"
              variant="surface"
              fullWidth
              className="justify-start"
            >
              <Send size={20} aria-hidden />
              <span className="flex-1 truncate text-left">
                {t.party.telegram} · @{card.telegram}
              </span>
              <ExternalLink size={16} className="text-muted" aria-hidden />
            </ButtonAnchor>
          )}
        </div>
      )}
    </Sheet>
  )
}
