import type { IntegrationStat } from '../types';

interface Props {
  stats: IntegrationStat[];
  colorOf: (integration: string) => string;
}

// Per-integration model breakdown: one card per ls_integration, with run
// count, error rate, and a mini bar per ls_model_name.
export function IntegrationBreakdown({ stats, colorOf }: Props) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {stats.map((s) => {
        const maxModel = s.models.reduce((m, x) => Math.max(m, x.count), 0);
        const errorRate = s.count > 0 ? Math.round((s.errors / s.count) * 100) : 0;
        return (
          <div key={s.integration} className="flex flex-col gap-3 rounded-lg border border-secondary p-4">
            <div className="flex items-center gap-2">
              <span className="size-3 shrink-0 rounded-sm" style={{ backgroundColor: colorOf(s.integration) }} />
              <span className="min-w-0 flex-1 truncate font-medium text-primary">{s.integration}</span>
              <span className="shrink-0 text-xs text-tertiary">
                {s.count} run{s.count === 1 ? '' : 's'} · {errorRate}% err
              </span>
            </div>

            <ul className="flex flex-col gap-1.5">
              {s.models.map((m) => (
                <li key={m.model} className="flex items-center gap-2 text-sm">
                  <span className="w-32 shrink-0 truncate text-secondary" title={m.model}>
                    {m.model}
                  </span>
                  <span className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-secondary">
                    <span
                      className="block h-full rounded-full"
                      style={{
                        width: `${maxModel > 0 ? (m.count / maxModel) * 100 : 0}%`,
                        backgroundColor: colorOf(s.integration),
                      }}
                    />
                  </span>
                  <span className="w-8 shrink-0 text-right text-tertiary">{m.count}</span>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
