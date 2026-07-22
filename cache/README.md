# search-replace-py

A standalone Python library for parsing and applying SEARCH/REPLACE patch blocks, extracted from [Aider's](https://github.com/Aider-AI/aider) editblock engine.

Use it to give any LLM the ability to propose and apply precise code changes using the battle-tested editblock format.

---

## How it works

The editblock format is Aider's primary mechanism for LLM-driven code editing. The LLM is prompted to output changes as structured `SEARCH/REPLACE` blocks:

````
```python
mathweb/flask/app.py
<<<<<<< SEARCH
from flask import Flask
=======
import math
from flask import Flask
>>>>>>> REPLACE
```
````

This library provides:

1. **`render_system_prompt()`** — returns the rendered system prompt string to instruct the LLM.
2. **`get_example_messages()`** — returns a `FewShotExampleMessages` named tuple with four plain strings (two user + two assistant turns) to prepend to the conversation history.
3. **`apply_diff(llm_response, root)`** — parses the LLM's response and applies all blocks to disk in one call.

---

## What is included

- Block parsing (`<<<<<<< SEARCH`, `=======`, `>>>>>>> REPLACE`) with filename discovery and fuzzy filename resolution.
- Three replacement strategies:
  - exact match
  - leading-whitespace-tolerant match
  - dotdotdot (`...`) segmented replacement
- Typed errors (`ParseError`, `ApplyError`) for clean error handling in retry loops.

---

## Installation

```bash
pip install search-replace-py
# or with uv
uv add search-replace-py
```

---

## Quick start

```python
from pathlib import Path
from search_replace import render_system_prompt, get_example_messages, apply_diff

# 1. Build the system prompt — plain string, append your own context if needed
system_prompt = render_system_prompt()

# 2. Build the messages list; prepend few-shot examples before the real request
ex = get_example_messages()
messages = [{"role": "system", "content": system_prompt}]
messages += [
    {"role": "user",      "content": ex.first_user_message},
    {"role": "assistant", "content": ex.first_assistant_message},
    {"role": "user",      "content": ex.second_user_message},
    {"role": "assistant", "content": ex.second_assistant_message},
]
messages.append({"role": "user", "content": "Add a docstring to the greet() function in hello.py"})

# 3. Send to your LLM and get a response string
llm_response = "..."

# 4. Parse and apply in one call
apply_diff(llm_response, root=Path("."))
```

---

## Integration with Pydantic AI

[Pydantic AI](https://ai.pydantic.dev) accepts a string for `instructions` and a list of `ModelMessage` objects for `message_history`.

```python
from pathlib import Path

from pydantic_ai import Agent, ModelRequest, ModelResponse, TextPart, UserPromptPart
from search_replace import render_system_prompt, get_example_messages, apply_diff

ex = get_example_messages()

few_shot = [
    ModelRequest(parts=[UserPromptPart(content=ex.first_user_message)]),
    ModelResponse(parts=[TextPart(content=ex.first_assistant_message)]),
    ModelRequest(parts=[UserPromptPart(content=ex.second_user_message)]),
    ModelResponse(parts=[TextPart(content=ex.second_assistant_message)]),
]

agent = Agent("openai:gpt-5.2", instructions=render_system_prompt())

auth_py = Path("auth.py").read_text()
result = agent.run_sync(
    f"Refactor the login function in auth.py to use bcrypt.\n\nauth.py\n```python\n{auth_py}\n```",
    message_history=few_shot,
)

apply_diff(result.output, root=Path("."))
```

### With dry-run validation before writing

```python
from search_replace import parse_edit_blocks, apply_edits
from search_replace.errors import ApplyError

blocks = parse_edit_blocks(result.output)

# Validate all blocks match before touching disk.
# Without dry_run, blocks that match are written immediately — a later failure
# would leave files partially patched with no rollback.
try:
    apply_edits(blocks.edits, root=Path("."), dry_run=True)
except ApplyError as e:
    # feed the error back to the LLM for a retry
    print(f"Patch would not apply: {e}")
else:
    apply_edits(blocks.edits, root=Path("."))
```

---

## Public API

```python
from search_replace import (
    # Prompt
    render_system_prompt,
    get_example_messages,    # returns FewShotExampleMessages
    FewShotExampleMessages,  # NamedTuple: first/second_user/assistant_message
    render_prompt,           # render a single template string
    EditBlockFencedPrompts,  # raw class with main_system, system_reminder, example_messages

    # Parse + apply (convenience)
    apply_diff,

    # Parsing
    parse_edit_blocks,
    find_original_update_blocks,
    EditBlock,

    # Applying
    apply_edits,              # pass dry_run=True to validate without writing

    # Errors
    ParseError,
    ApplyError,
    MissingFilenameError,
)
```

---

## Tests and validation

- `tests/test_parser.py` — block parsing, filename resolution, edge cases
- `tests/test_apply.py` — replacement strategies, whitespace tolerance, new-file creation
- `tests/test_prompts.py` — `render_system_prompt` and `get_example_messages` output
- `tests/test_parity_harness.py` — byte-for-byte comparison against Aider's reference output on the real 100K-line `chat-history.md` fixture

```bash
uv run python -m pytest tests/
```

---

## Credits

The parsing engine, replacement strategies, and prompt templates in this library are derived from [Aider](https://github.com/Aider-AI/aider), created by [Paul Gauthier](https://github.com/paul-gauthier). Aider is an outstanding AI pair-programming tool — this library simply extracts and packages its editblock mechanism so it can be reused in other applications. All credit for the original design and implementation goes to him.

---

## Extraction notes

- Extracted from `aider/coders/editblock_coder.py` and the fenced editblock prompt module.
- Runtime coupling to Aider's coder/model lifecycle is fully removed.
- Error message contracts for malformed blocks and failed apply paths are preserved to maintain LLM retry-loop compatibility.
- `replace_closest_edit_distance()` remains defined but inactive, preserving the original behaviour of the early return in `replace_most_similar_chunk()`.
