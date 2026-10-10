import { MessageSquare, Trash2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import type { NewsComment } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { Avatar } from '@/components/ui/Avatar'
import { Button, IconButton } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Field, Textarea } from '@/components/ui/Field'
import { SectionHeader } from '@/components/ui/SectionHeader'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { useAddComment, useComments, useDeleteComment } from '@/features/useContent'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { formatRelativeTime } from '@/lib/format'

const MAX_TEXT = 1000

/** Обсуждение новости или патча: читать могут все, писать — вошедшие через Steam. */
export function Comments({ gid }: { gid: string }) {
  const comments = useComments(gid)
  const total = comments.data?.pages[0]?.total

  return (
    <Card padding="none" className="mt-8 p-5">
      <SectionHeader
        label={
          <>
            {t.news.comments.title}
            {total ? <span className="ml-2 tabular-nums text-dim">{total}</span> : null}
          </>
        }
      />
      <CommentForm gid={gid} />

      <div className="mt-5">
        {comments.isPending && (
          <div className="space-y-4" aria-hidden>
            {[0, 1].map((i) => (
              <div key={i} className="flex gap-3">
                <Skeleton className="size-9" radius="full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-4" />
                </div>
              </div>
            ))}
          </div>
        )}
        {comments.isError && (
          <ErrorState
            className="py-6"
            onRetry={() => comments.refetch()}
            retrying={comments.isRefetching}
          />
        )}
        {comments.data && total === 0 && (
          <EmptyState
            icon={MessageSquare}
            title={t.news.comments.empty}
            description={t.news.comments.emptyText}
          />
        )}
        {comments.data && total !== 0 && (
          <>
            <ul className="space-y-5">
              {comments.data.pages
                .flatMap((page) => page.items)
                .map((comment) => (
                  <li key={comment.id}>
                    <CommentRow gid={gid} comment={comment} />
                  </li>
                ))}
            </ul>
            {comments.hasNextPage && (
              <Button
                variant="outline"
                fullWidth
                className="mt-5"
                loading={comments.isFetchingNextPage}
                onClick={() => comments.fetchNextPage()}
              >
                {t.news.comments.more}
              </Button>
            )}
          </>
        )}
      </div>
    </Card>
  )
}

function CommentForm({ gid }: { gid: string }) {
  const { data: me } = useMe()
  const add = useAddComment(gid)
  const [text, setText] = useState('')

  const user = me?.user
  const trimmed = text.trim()
  const canSend = trimmed.length > 0 && text.length <= MAX_TEXT && !add.isPending

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!canSend) return
    add.mutate(trimmed, { onSuccess: () => setText('') })
  }

  if (!user) {
    // me ещё грузится — не мигаем призывом войти
    if (!me) return <Skeleton className="mt-4 h-12" radius={12} />
    return (
      <div className="mt-4 flex flex-col gap-3 rounded-[12px] bg-surface-2 p-4 sm:flex-row sm:items-center">
        <p className="min-w-0 flex-1 text-[15px] text-muted">{t.news.comments.loginHint}</p>
        <SteamLoginButton size="sm" />
      </div>
    )
  }
  return (
    <form onSubmit={onSubmit} noValidate className="mt-4 flex gap-3">
      <Avatar src={user.avatar} name={user.persona_name} size={36} className="mt-0.5" />
      <div className="min-w-0 flex-1 space-y-3">
        <Field
          label={t.news.comments.label}
          htmlFor={`comment-${gid}`}
          counter={{ value: text.length, max: MAX_TEXT }}
        >
          <Textarea
            id={`comment-${gid}`}
            value={text}
            maxLength={MAX_TEXT}
            placeholder={t.news.comments.placeholder}
            onChange={(e) => setText(e.target.value)}
            className="min-h-[96px]"
          />
        </Field>
        <Button type="submit" disabled={!canSend} loading={add.isPending}>
          {t.news.comments.submit}
        </Button>
      </div>
    </form>
  )
}

function CommentRow({ gid, comment }: { gid: string; comment: NewsComment }) {
  const remove = useDeleteComment(gid)

  return (
    <article className="flex gap-3">
      <Avatar src={comment.avatar} name={comment.author} size={36} className="mt-0.5" />
      <div className="min-w-0 flex-1">
        <header className="flex items-baseline gap-2">
          <span className="truncate text-[15px] font-semibold">{comment.author}</span>
          {comment.mine && <span className="text-[12px] text-dim">{t.news.comments.you}</span>}
          <time
            dateTime={comment.created_at}
            className="ml-auto shrink-0 font-mono text-[12px] text-muted"
          >
            {formatRelativeTime(comment.created_at)}
          </time>
          {comment.mine && (
            <IconButton
              aria-label={t.news.comments.delete}
              size="icon-sm"
              className="-my-2 -mr-2 shrink-0"
              disabled={remove.isPending}
              onClick={() =>
                remove.mutate(comment.id, { onSuccess: () => toast.info(t.news.comments.deleted) })
              }
            >
              <Trash2 size={16} />
            </IconButton>
          )}
        </header>
        <p className="mt-1 whitespace-pre-wrap text-[15px] leading-relaxed [overflow-wrap:anywhere]">
          {comment.text}
        </p>
      </div>
    </article>
  )
}
