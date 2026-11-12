import { CopyIconButton } from '@langchain/macaw-components/CopyButton';
import { cn } from '@langchain/macaw-components/utils/cn';

export function CodeBlock({
  code,
  language,
  className,
}: {
  code: string;
  language?: string | null;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'group/code relative min-w-0 rounded-md border border-subtle bg-surface-level-2',
        className
      )}
    >
      <div className="flex h-7 items-center justify-between border-b border-muted pl-space-3 pr-space-1">
        <span className="font-mono text-xxs uppercase tracking-wide text-tertiary">
          {language || 'text'}
        </span>
        <CopyIconButton copy={code} size="xs" variant="plain" color="secondary" label="Copy code" />
      </div>
      <pre className="m-0 max-h-[480px] overflow-auto px-space-3 py-space-2 font-mono text-xs leading-relaxed text-primary">
        <code>{code}</code>
      </pre>
    </div>
  );
}
