from managed_deepagents import define_schedule

schedule = define_schedule(
    cron="0 9 * * 1",
    input={"topic": "news"},
    thread={"mode": "persistent", "id": "weekly"},
)
