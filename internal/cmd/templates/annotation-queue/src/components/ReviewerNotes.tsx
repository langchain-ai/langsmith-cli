import { Trash02Icon } from '@langchain/untitled-ui-icons';
import { useEffect, useState } from 'react';
import { Banner } from '@/components/langsmith/design-system/components/Banner';
import { Button } from '@/components/langsmith/design-system/components/Button';
import { IconButton } from '@/components/langsmith/design-system/components/IconButton';
import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { Textarea } from '@/components/langsmith/design-system/components/Textarea';
import { deleteFeedback, fetchFeedbacksForRun, submitFeedback } from '../api';
import type { FeedbackItem } from '../types';

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

interface Props {
  runId: string | undefined;
  traceId: string | undefined;
  sessionId: string | undefined;
  startTime: string | undefined;
}

// Mirrors LangSmith's RunNotesCrud: a per-run comment thread built on the
// regular feedback API under the reserved "note" key (comment-only, no score).
// THREAD items omit this panel — same as the product annotation queue UI.
const NOTE_KEY = 'note';

export function ReviewerNotes({ runId, traceId, sessionId, startTime }: Props) {
  const [notes, setNotes] = useState<FeedbackItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setDraft('');
    setError(null);
    if (!runId) {
      setNotes([]);
      return;
    }
    setLoading(true);
    fetchFeedbacksForRun(runId)
      .then((items) =>
        setNotes(
          items
            .filter((f) => f.key === NOTE_KEY && f.comment)
            .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
        )
      )
      .catch((e) => console.error('Failed to load reviewer notes', e))
      .finally(() => setLoading(false));
  }, [runId]);

  async function handleAddNote() {
    if (!runId || !draft.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      const saved = await submitFeedback({
        key: NOTE_KEY,
        run_id: runId,
        comment: draft.trim(),
        feedback_config: { type: 'freeform' },
        trace_id: traceId,
        session_id: sessionId,
        start_time: startTime,
      });
      setNotes((prev) => [saved, ...prev]);
      setDraft('');
    } catch (e) {
      console.error('Failed to add reviewer note', e);
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDeleteNote(note: FeedbackItem) {
    try {
      await deleteFeedback(note.id);
      setNotes((prev) => prev.filter((n) => n.id !== note.id));
    } catch (e) {
      console.error('Failed to delete reviewer note', e);
      setError(errorMessage(e));
    }
  }

  if (!runId) return null;

  return (
    <div className="flex flex-col gap-space-3">
      <Text variant="xs" weight="medium" color="tertiary" className="uppercase tracking-wide">
        Reviewer notes
      </Text>
      {error && (
        <Banner intent="error" title="Couldn't save that note" dismissible onDismiss={() => setError(null)}>
          {error}
        </Banner>
      )}
      <div className="flex flex-col gap-space-2">
        <Textarea
          rows={3}
          resize="none"
          placeholder="Leave a note for other reviewers…"
          value={draft}
          onChange={setDraft}
          disabled={submitting}
        />
        <Button
          size="sm"
          color="secondary"
          variant="outlined"
          className="self-end"
          onClick={handleAddNote}
          loading={submitting}
          disabled={submitting || !draft.trim()}
        >
          {submitting ? 'Adding…' : 'Add note'}
        </Button>
      </div>
      {loading ? (
        <div className="flex justify-center py-space-2">
          <Spinner size="xs" className="text-icon-tertiary" />
        </div>
      ) : (
        notes.length > 0 && (
          <ul className="flex flex-col gap-space-2">
            {notes.map((note) => (
              <li
                key={note.id}
                className="flex flex-col gap-space-1 rounded-md border border-default px-space-3 py-space-2"
              >
                <div className="flex items-start justify-between gap-space-2">
                  <Text variant="md" className="min-w-0 flex-1 whitespace-pre-wrap">
                    {note.comment}
                  </Text>
                  <IconButton
                    size="xs"
                    color="secondary"
                    variant="plain"
                    icon={Trash02Icon}
                    label="Delete note"
                    onClick={() => handleDeleteNote(note)}
                  />
                </div>
                <Text variant="sm" color="quaternary">
                  {new Date(note.created_at).toLocaleString()}
                </Text>
              </li>
            ))}
          </ul>
        )
      )}
    </div>
  );
}
