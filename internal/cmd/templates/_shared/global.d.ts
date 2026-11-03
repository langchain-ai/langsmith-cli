export {};

declare global {
  /** Metadata supplied as render()'s third argument by the host. */
  type RenderMetadata = {
    mode: 'dark' | 'light';
    /** Empty when the workspace is unresolved. */
    workspaceId: string;
    host: string;
    /** The app's route: "/" when the link has none, undefined where deep links aren't supported. */
    route?: string;
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
      /**
       * Sets the app's route in the page URL. Replaces by default; { replace: false } adds a Back step.
       * "" or "/" clears it; a route over 2,048 UTF-8 bytes is ignored.
       * Deep links work only where metadata.route is a string and this method exists.
       */
      setRoute?: (route: string, options?: { replace?: boolean }) => void;
      feedback: {
        create: (args: Record<string, unknown>) => Promise<unknown>;
      };
    };
  }
}
