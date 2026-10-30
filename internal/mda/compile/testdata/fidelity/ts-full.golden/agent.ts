import { defineDeepAgent } from "managed-deepagents";
import { lookup } from "./tools/lookup";

export const agent = defineDeepAgent({
  name: "Support Bot",
  model: "anthropic:claude-sonnet-4-5",
  tools: [lookup],
  subagents: [
    { name: "researcher", description: "Researches questions", async: true },
  ],
});
