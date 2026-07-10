import { ArrowUpRightIcon, PlusIcon, XIcon } from '@langchain/untitled-ui-icons';

export interface AdhocAssertion {
  key: string;
  comment: string;
}

interface Props {
  assertions: AdhocAssertion[];
  onChange: (next: AdhocAssertion[]) => void;
}

/**
 * Free-form assertion (key + comment) drafts, mirroring LangSmith's Annotation Queue
 * "Assertions" panel. These are session-local drafts (e.g. for curating a dataset
 * example) rather than persisted feedback — the annotation-queues API has no
 * assertions endpoint to save them to.
 */
export function AssertionsSection({ assertions, onChange }: Props) {
  function addAssertion() {
    onChange([...assertions, { key: '', comment: '' }]);
  }

  function updateAssertion(index: number, next: AdhocAssertion) {
    onChange(assertions.map((a, i) => (i === index ? next : a)));
  }

  function deleteAssertion(index: number) {
    onChange(assertions.filter((_, i) => i !== index));
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="text-base font-medium text-primary">Assertions</div>
          <a
            href="https://docs.langchain.com/langsmith/assertions#use-assertions"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm text-link hover:text-link-hover"
          >
            Learn more
            <ArrowUpRightIcon className="h-3 w-3" />
          </a>
        </div>
        <button
          type="button"
          onClick={addAssertion}
          className="inline-flex items-center gap-1 rounded-md border border-secondary px-2.5 py-1.5 text-xs font-medium text-secondary hover:bg-secondary"
        >
          <PlusIcon className="h-3.5 w-3.5" />
          Add
        </button>
      </div>

      {assertions.map((assertion, index) => (
          <div key={`adhoc-${index}`} className="flex flex-col gap-2 rounded-lg border border-secondary p-3">
            <div className="flex items-center gap-2">
              <input
                value={assertion.key}
                onChange={(e) => updateAssertion(index, { ...assertion, key: e.target.value })}
                placeholder="must_<slug> or must_not_<slug>"
                className="min-w-0 flex-1 rounded-md border border-secondary bg-primary px-3 py-1.5 font-mono text-sm text-primary focus:border-brand focus:outline-none"
              />
              <button
                type="button"
                onClick={() => deleteAssertion(index)}
                className="shrink-0 rounded p-1 text-quaternary hover:bg-secondary"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </div>
            <textarea
              value={assertion.comment}
              onChange={(e) => updateAssertion(index, { ...assertion, comment: e.target.value })}
              placeholder="Describe the claim being made…"
              rows={2}
              className="resize-none rounded-md border border-secondary bg-primary px-3 py-1.5 text-sm text-primary focus:border-brand focus:outline-none"
            />
          </div>
      ))}
    </div>
  );
}
