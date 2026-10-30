import { defineDeepAgent } from "managed-deepagents";

export const agent = defineDeepAgent({ name: "with-local", model: "openai:gpt-5" });
