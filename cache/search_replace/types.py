from dataclasses import dataclass
from typing import TypeAlias

Fence: TypeAlias = tuple[str, str]
DEFAULT_FENCE: Fence = ("`" * 3, "`" * 3)


@dataclass(frozen=True, slots=True)
class EditBlock:
    path: str
    original: str
    updated: str


@dataclass(frozen=True, slots=True)
class ParseResult:
    edits: list[EditBlock]


@dataclass(frozen=True, slots=True)
class ApplyResult:
    updated_edits: list[EditBlock]
