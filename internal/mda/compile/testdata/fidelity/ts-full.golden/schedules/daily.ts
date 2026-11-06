import { defineSchedule } from "managed-deepagents";

export const schedule = defineSchedule({
  cron: "0 9 * * *",
  prompt: "Summarize yesterday's tickets.",
});
