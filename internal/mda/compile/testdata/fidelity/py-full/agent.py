from managed_deepagents import define_deep_agent

agent = define_deep_agent(
    name="Support Bot",
    model="openai:gpt-5",
    subagents=[
        {"name": "researcher", "description": "Researches questions", "async": True},
    ],
)
