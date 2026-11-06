import { defineMemory, memoryLayer } from "managed-deepagents";

export const memory = defineMemory({ agent: memoryLayer() });
