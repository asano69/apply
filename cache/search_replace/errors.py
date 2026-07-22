from dataclasses import dataclass

from .types import EditBlock


class SearchReplaceError(ValueError):
    pass


class ParseError(SearchReplaceError):
    pass


class MissingFilenameError(ParseError):
    pass


class PathEscapeError(SearchReplaceError):
    pass


@dataclass(slots=True)
class ApplyError(SearchReplaceError):
    message: str
    failed: list[EditBlock]
    passed: list[EditBlock]
    updated_edits: list[EditBlock]

    def __str__(self) -> str:
        return self.message
