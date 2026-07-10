import { Trash02Icon } from '@langchain/untitled-ui-icons';
import { useEffect, useState } from 'react';
import { deleteFeedback, fetchFeedbacks, submitFeedback } from '../api';
import type { FeedbackItem } from '../types';
import { Spinner } from './Spinner';

interface Props {
  runId: string | undefined;
}

// Mirrors LangSmith's RunNotesCrud: a per-run comment thread built on the
// regular feedback API under the reserved "note" key (comment-only, no score).
const NOTE_KEY = 'note';

export function ReviewerNotes({ runId }: Props) {
  const [notes, setNotes] = useState<FeedbackItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setDraft('');
    if (!runId) {
      setNotes([]);
      return;
    }
    setLoading(true);
    fetchFeedbacks(runId)
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
    try {
      const saved = await submitFeedback({
        key: NOTE_KEY,
        run_id: runId,
        comment: draft.trim(),
      });
      setNotes((prev) => [saved, ...prev]);
      setDraft('');
    } catch (e) {
      console.error('Failed to add reviewer note', e);
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
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="text-base font-medium text-primary">Reviewer Notes</div>

      <div className="flex flex-col gap-2">
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              handleAddNote();
            }
          }}
          placeholder="Leave a note for other reviewers…"
          rows={2}
          disabled={!runId || submitting}
          className="resize-none rounded-md border border-secondary bg-primary px-3 py-1.5 text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-50"
        />
        <button
          type="button"
          onClick={handleAddNote}
          disabled={!runId || submitting || !draft.trim()}
          className="self-end rounded-md border border-secondary px-2.5 py-1.5 text-xs font-medium text-secondary hover:bg-secondary disabled:opacity-50"
        >
          {submitting ? 'Adding…' : 'Add note'}
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-4">
          <Spinner size="sm" />
        </div>
      ) : (
        notes.length > 0 && (
          <div className="flex flex-col gap-2">
            {notes.map((note) => (
              <div
                key={note.id}
                className="flex flex-col gap-1 rounded-lg border border-secondary p-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="whitespace-pre-line break-words text-sm text-primary">
                    {note.comment}
                  </span>
                  <button
                    type="button"
                    onClick={() => handleDeleteNote(note)}
                    className="shrink-0 rounded p-1 text-quaternary hover:bg-secondary"
                  >
                    <Trash02Icon className="h-3.5 w-3.5" />
                  </button>
                </div>
                <span className="text-xs text-quaternary">
                  {new Date(note.created_at).toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        )
      )}
    </div>
  );
}
