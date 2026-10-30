const greeting = `Hello, \`friend\`! Use \${name}.
`;

export function lookup(query: string): string {
  return `${greeting} ${query}`;
}
