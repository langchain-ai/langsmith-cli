export {};

declare global {
  /** render()'s 3rd arg, an open/extensible dict. v1 has only `mode`
   * ("dark"|"light"); the sandbox sets `html.dark` from it. */
  type RenderMetadata = { mode: 'dark' | 'light' };

  /** render()'s 1st arg for a "thread" context app: the thread this app is
   * embedded in, plus the tracing project it belongs to. Everything else the
   * app fetches itself via window.langsmith.call. When running
   * `langsmith apps dev` without --thread-id/--project-id these are empty. */
  type ThreadData = { threadId: string; projectId: string };

  interface Window {
    /** Injected by the host page (LangSmith, or `langsmith apps dev` locally) — see AGENTS.md. */
    langsmith: {
      call: (
        operation: string,
        args?: { params?: Record<string, string | string[]>; body?: unknown }
      ) => Promise<unknown>;
      setData: (patch: Record<string, unknown>) => void;
      feedback: {
        create: (args: Record<string, unknown>) => Promise<unknown>;
      };
    };
  }
}
