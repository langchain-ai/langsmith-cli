export {};

declare global {
  /** Metadata supplied as render()'s third argument by the host. */
  type RenderMetadata = {
    mode: 'dark' | 'light';
    /** Empty when the workspace is unresolved. */
    workspaceId: string;
    host: string;
  };

  interface Window {
    /** Injected by the host page (LangSmith, or `langsmith apps dev` locally). See AGENTS.md. */
    langsmith: {
      call: (
        operation: string,
        args?: { params?: Record<string, string | string[]>; body?: unknown }
      ) => Promise<unknown>;
      setData: (patch: Record<string, unknown>) => void;
      download: (options: {
        filename: string;
        content: string | Blob | ArrayBuffer | ArrayBufferView;
        mimeType?: string;
      }) => Promise<{ status: 'started' }>;
      openUrl: (url: string, options?: { newTab?: boolean }) => void;
      feedback: {
        create: (args: Record<string, unknown>) => Promise<unknown>;
      };
    };
  }
}
