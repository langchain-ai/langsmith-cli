import { useEffect, useState } from 'react';

// A minimal thread viewer, wired to the { threadId, projectId } this app is
// rendered with. It loads the thread's chronological messages via
// POST /v1/trajectory (format: "messages") and lists them. This is a starting
// point — rip it out and build whatever thread-scoped UI you actually want; see
// AGENTS.md for the full LangSmith API surface.

interface StandardMessage {
  role: string;
  content: string | Array<string | Record<string, unknown>>;
}

// Cap paging so a very long thread can't loop forever.
const MAX_PAGES = 20;

async function fetchThreadMessages(
  threadId: string,
  projectId: string
): Promise<StandardMessage[]> {
  const messages: StandardMessage[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < MAX_PAGES; page++) {
    const body: Record<string, string> = {
      project_id: projectId,
      thread_id: threadId,
      format: 'messages',
    };
    if (cursor) body.cursor = cursor;
    const resp = (await window.langsmith.call('POST /v1/trajectory', { body })) as {
      messages?: StandardMessage[];
      next_cursor?: string | null;
    };
    messages.push(...(resp.messages ?? []));
    if (!resp.next_cursor) break;
    cursor = resp.next_cursor;
  }
  return messages;
}

function messageText(content: StandardMessage['content']): string {
  if (typeof content === 'string') return content;
  return content
    .map((part) => (typeof part === 'string' ? part : JSON.stringify(part)))
    .join('\n');
}

export function App({ data }: { data: ThreadData; metadata?: RenderMetadata }) {
  const { threadId, projectId } = data;
  const [messages, setMessages] = useState<StandardMessage[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!threadId || !projectId) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchThreadMessages(threadId, projectId)
      .then((msgs) => {
        if (!cancelled) setMessages(msgs);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [threadId, projectId]);

  // No context: happens under `langsmith apps dev` without --thread-id/--project-id.
  if (!threadId || !projectId) {
    return (
      <Frame>
        <h1 className="text-lg font-semibold text-primary">Thread app</h1>
        <p className="mt-2 text-sm leading-relaxed text-tertiary">
          This app renders inside a project's thread view and receives{' '}
          <code className="text-secondary">{'{ threadId, projectId }'}</code>. To preview it
          locally with real data, run:
        </p>
        <pre className="mt-3 overflow-x-auto rounded-lg border border-secondary bg-secondary p-3 text-xs leading-relaxed text-secondary">
          {'langsmith apps dev \\\n  --thread-id <thread-id> \\\n  --project-id <project-id>'}
        </pre>
      </Frame>
    );
  }

  return (
    <Frame wide>
      <div className="mb-4">
        <h1 className="text-lg font-semibold text-primary">Thread</h1>
        <p className="mt-1 font-mono text-xs text-quaternary">{threadId}</p>
      </div>

      {loading && <p className="text-sm text-tertiary">Loading messages…</p>}

      {error && (
        <div className="rounded-lg border border-error bg-error p-3 text-sm text-error-primary">
          {error}
        </div>
      )}

      {messages && messages.length === 0 && !loading && (
        <p className="text-sm text-tertiary">No messages in this thread.</p>
      )}

      {messages && messages.length > 0 && (
        <ul className="flex flex-col gap-3">
          {messages.map((m, i) => (
            <li
              key={i}
              className="rounded-lg border border-secondary bg-elevated p-3 shadow-sm"
            >
              <div className="text-xs font-medium uppercase tracking-wide text-quaternary">
                {m.role}
              </div>
              <pre className="mt-1.5 whitespace-pre-wrap break-words text-sm leading-relaxed text-secondary">
                {messageText(m.content)}
              </pre>
            </li>
          ))}
        </ul>
      )}
    </Frame>
  );
}

function Frame({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <div className="min-h-screen bg-surface-level-1 p-6">
      <div className={`mx-auto w-full ${wide ? 'max-w-3xl' : 'max-w-md'}`}>{children}</div>
    </div>
  );
}
