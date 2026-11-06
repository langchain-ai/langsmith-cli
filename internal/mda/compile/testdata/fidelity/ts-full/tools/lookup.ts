import greeting from "../prompts/greeting.md" with { type: "text" };

export function lookup(query: string): string {
  return `${greeting} ${query}`;
}
