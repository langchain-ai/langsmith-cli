export {};

declare global {
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
