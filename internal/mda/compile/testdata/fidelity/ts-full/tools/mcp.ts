import { defineMcp } from "managed-deepagents";

export const mcp = defineMcp({
  servers: { docs: { transport: "http", url: "https://docs.example.com/mcp" } },
});
