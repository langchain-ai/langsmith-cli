import type { Run } from '../types';

// Custom metadata read helpers. Field names vary across integrations, so the
// entity accessors fall back through the known aliases (see AGENTS.md).
function str(run: Run, key: string): string | undefined {
  const v = run.extra?.metadata?.[key];
  return typeof v === 'string' && v ? v : undefined;
}

function num(run: Run, key: string): number | undefined {
  const v = run.extra?.metadata?.[key];
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined;
}

export const UNKNOWN = 'unknown';

export function integrationOf(run: Run): string {
  return str(run, 'ls_integration') ?? UNKNOWN;
}

export function modelOf(run: Run): string {
  return str(run, 'ls_model_name') ?? str(run, 'model') ?? UNKNOWN;
}

export function providerOf(run: Run): string {
  return str(run, 'ls_provider') ?? UNKNOWN;
}

export function toolNameOf(run: Run): string {
  return str(run, 'ls_tool_name') ?? str(run, 'tool_name') ?? str(run, 'toolName') ?? run.name ?? UNKNOWN;
}

export function userOf(run: Run): string {
  return str(run, 'user_name') ?? str(run, 'user_email') ?? str(run, 'local_username') ?? UNKNOWN;
}

export function repoOf(run: Run): string {
  return str(run, 'repository_name') ?? UNKNOWN;
}

export function branchOf(run: Run): string {
  return str(run, 'git_branch') ?? UNKNOWN;
}

export function threadOf(run: Run): string | undefined {
  return str(run, 'thread_id');
}

export function turnOf(run: Run): number | undefined {
  return num(run, 'turn_number');
}

export function subagentTypeOf(run: Run): string {
  return str(run, 'ls_subagent_type') ?? UNKNOWN;
}

export function stopReasonOf(run: Run): string {
  return str(run, 'stop_reason') ?? 'none';
}

export function integrationVersionOf(run: Run): string {
  const runtime = str(run, 'ls_agent_runtime');
  const version = str(run, 'ls_integration_version') ?? str(run, 'ls_agent_runtime_version');
  if (!version) return UNKNOWN;
  return runtime ? `${runtime} ${version}` : version;
}

export function sdkVersionOf(run: Run): string {
  const rt = run.extra?.runtime;
  const sdk = typeof rt?.sdk === 'string' ? rt.sdk : 'sdk';
  const v = typeof rt?.sdk_version === 'string' ? rt.sdk_version : undefined;
  return v ? `${sdk} ${v}` : UNKNOWN;
}

export function cacheRead(run: Run): number {
  return run.prompt_token_details?.cache_read ?? 0;
}

export function cacheCreation(run: Run): number {
  return run.prompt_token_details?.cache_creation ?? 0;
}

export function durationMs(run: Run): number | null {
  if (!run.start_time || !run.end_time) return null;
  const s = Date.parse(run.start_time);
  const e = Date.parse(run.end_time);
  return Number.isFinite(s) && Number.isFinite(e) && e >= s ? e - s : null;
}
