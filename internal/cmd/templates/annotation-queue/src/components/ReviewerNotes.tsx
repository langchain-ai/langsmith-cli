import { Trash02Icon } from '@langchain/untitled-ui-icons';
import { useEffect, useState } from 'react';
import { deleteFeedback, fetchFeedbacksForRun, submitFeedback } from '../api';
import type { FeedbackItem } from '../types';
import { ErrorBanner } from './ErrorBanner';
import { Spinner } from './Spinner';

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
    <div className="flex flex-col gap-3">
      <div className="text-xs font-medium uppercase tracking-wide text-tertiary">
        Reviewer notes
      </div>
      {error && <ErrorBanner error={error} />}
      <div className="flex flex-col gap-2">
        <textarea
          className="resize-none rounded-md border border-secondary bg-primary px-3 py-1.5 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-50"
          rows={3}
          placeholder="Leave a note for other reviewers…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          disabled={submitting}
        />
        <button
          type="button"
          className="self-end rounded-md border border-secondary px-2.5 py-1.5 text-xs font-medium text-secondary hover:bg-secondary disabled:opacity-50"
          onClick={handleAddNote}
          disabled={submitting || !draft.trim()}
        >
          {submitting ? 'Adding…' : 'Add note'}
        </button>
      </div>
      {loading ? (
        <div className="flex justify-center py-2">
          <Spinner size="sm" />
        </div>
      ) : (
        notes.length > 0 && (
          <ul className="flex flex-col gap-2">
            {notes.map((note) => (
              <li
                key={note.id}
                className="flex flex-col gap-1 rounded-md border border-secondary px-3 py-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <p className="min-w-0 flex-1 whitespace-pre-wrap text-sm text-primary">
                    {note.comment}
                  </p>
                  <button
                    type="button"
                    className="shrink-0 rounded p-1 text-quaternary hover:bg-secondary"
                    onClick={() => handleDeleteNote(note)}
                    aria-label="Delete note"
                  >
                    <Trash02Icon className="h-3.5 w-3.5" />
                  </button>
                </div>
                <span className="text-xs text-quaternary">
                  {new Date(note.created_at).toLocaleString()}
                </span>
              </li>
            ))}
          </ul>
        )
      )}
    </div>
  );
}
