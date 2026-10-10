import { CircleCheck } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Field, Input, Textarea } from '@/components/ui/Field'
import { Page, PageHeader } from '@/components/ui/Page'
import { useSendFeedback } from '@/features/useContent'
import { t } from '@/i18n/ru'

const MAX_TEXT = 2000
const MAX_CONTACT = 200

// Отзыв (SPEC 5.1): текст до 2000 символов и необязательный контакт.
export default function FeedbackPage() {
  const [text, setText] = useState('')
  const [contact, setContact] = useState('')
  const [sent, setSent] = useState(false)
  const send = useSendFeedback()

  const trimmed = text.trim()
  const canSend = trimmed.length >= 2 && text.length <= MAX_TEXT && !send.isPending

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!canSend) return
    send.mutate(
      { text: trimmed, contact: contact.trim() || undefined },
      {
        onSuccess: () => {
          setText('')
          setContact('')
          setSent(true)
        },
      },
    )
  }

  if (sent) {
    return (
      <Page title={t.feedback.title}>
        <Card padding="none" className="mt-4">
          <EmptyState
            icon={CircleCheck}
            title={t.feedback.thanksTitle}
            description={t.feedback.thanksText}
            action={
              <div className="flex flex-wrap justify-center gap-3">
                <ButtonLink to="/" variant="outline">
                  {t.common.toHome}
                </ButtonLink>
                <Button variant="surface" onClick={() => setSent(false)}>
                  {t.feedback.again}
                </Button>
              </div>
            }
          />
        </Card>
      </Page>
    )
  }

  return (
    <Page title={t.feedback.title}>
      <PageHeader title={t.feedback.title} subtitle={t.feedback.subtitle} />
      <Card padding="lg">
        <form onSubmit={onSubmit} noValidate className="space-y-5">
          <Field
            label={t.feedback.textLabel}
            htmlFor="feedback-text"
            counter={{ value: text.length, max: MAX_TEXT }}
          >
            <Textarea
              id="feedback-text"
              value={text}
              maxLength={MAX_TEXT}
              placeholder={t.feedback.textPlaceholder}
              onChange={(e) => setText(e.target.value)}
              required
            />
          </Field>
          <Field
            label={t.feedback.contactLabel}
            htmlFor="feedback-contact"
            hint={t.feedback.contactHint}
          >
            <Input
              id="feedback-contact"
              value={contact}
              maxLength={MAX_CONTACT}
              autoComplete="off"
              onChange={(e) => setContact(e.target.value)}
            />
          </Field>
          <Button type="submit" size="lg" fullWidth disabled={!canSend} loading={send.isPending}>
            {t.feedback.submit}
          </Button>
        </form>
      </Card>
    </Page>
  )
}
