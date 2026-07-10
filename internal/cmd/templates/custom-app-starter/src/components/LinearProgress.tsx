/** Indeterminate top-of-page progress bar, shown while the queue/run data is still loading. */
export function LinearProgress() {
  return (
    <div className="h-0.5 w-full shrink-0 overflow-hidden bg-brand-subtle">
      <div
        className="h-full w-[12.5%] rounded-full bg-brand"
        style={{ animation: 'linear-progress-indeterminate 1.2s ease-in-out infinite' }}
      />
    </div>
  );
}
