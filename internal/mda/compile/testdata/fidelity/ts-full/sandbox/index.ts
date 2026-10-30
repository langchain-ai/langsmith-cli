import { defineSandbox } from "managed-deepagents";

export const sandbox = defineSandbox({ dockerImage: "node:22", fsCapacityBytes: 2147483648 });
