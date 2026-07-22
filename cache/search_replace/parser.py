import difflib
import re
from pathlib import Path
from typing import Iterator, Sequence, TypeAlias

from .errors import MissingFilenameError, ParseError
from .types import DEFAULT_FENCE, EditBlock, Fence, ParseResult


def wrap_fence(name: str) -> Fence:
    return f"<{name}>", f"</{name}>"


all_fences = [
    ("`" * 3, "`" * 3),
    ("`" * 4, "`" * 4),  # LLMs ignore and revert to triple-backtick, causing #2879
    wrap_fence("source"),
    wrap_fence("code"),
    wrap_fence("pre"),
    wrap_fence("codeblock"),
    wrap_fence("sourcecode"),
]


HEAD = r"^<{5,9} SEARCH>?\s*$"
DIVIDER = r"^={5,9}\s*$"
UPDATED = r"^>{5,9} REPLACE\s*$"

HEAD_ERR = "<<<<<<< SEARCH"
DIVIDER_ERR = "======="
UPDATED_ERR = ">>>>>>> REPLACE"

separators = "|".join([HEAD, DIVIDER, UPDATED])
split_re = re.compile(r"^((?:" + separators + r")[ ]*\n)", re.MULTILINE | re.DOTALL)

missing_filename_err = (
    "Bad/missing filename. The filename must be alone on the line before the opening fence"
    " {fence[0]}"
)

# Always be willing to treat triple-backticks as a fence when searching for filenames.
triple_backticks = "`" * 3

ParsedEditBlock: TypeAlias = tuple[str, str, str]


def strip_filename(filename: str, fence: Fence) -> str | None:
    filename = filename.strip()

    if filename == "...":
        return None

    start_fence = fence[0]
    if filename.startswith(start_fence):
        candidate = filename[len(start_fence) :]
        if candidate and ("." in candidate or "/" in candidate):
            return candidate
        return None

    if filename.startswith(triple_backticks):
        candidate = filename[len(triple_backticks) :]
        if candidate and ("." in candidate or "/" in candidate):
            return candidate
        return None

    filename = filename.rstrip(":")
    filename = filename.lstrip("#")
    filename = filename.strip()
    filename = filename.strip("`")
    filename = filename.strip("*")

    # https://github.com/Aider-AI/aider/issues/1158
    # filename = filename.replace("\\_", "_")
    return filename


def find_filename(
    lines: list[str], fence: Fence, valid_fnames: Sequence[str] | None
) -> str | None:
    """
    Deepseek Coder v2 has been doing this:

     ```python
    word_count.py
    ```
    ```python
    <<<<<<< SEARCH
    ...

    This is a more flexible search back for filenames.
    """
    if valid_fnames is None:
        valid_fnames = []

    # Go back through the 3 preceding lines.
    lines.reverse()
    lines = lines[:3]

    filenames: list[str] = []
    for line in lines:
        filename = strip_filename(line, fence)
        if filename:
            filenames.append(filename)

        # Only continue as long as we keep seeing fences.
        if not line.startswith(fence[0]) and not line.startswith(triple_backticks):
            break

    if not filenames:
        return None

    # Check for exact match first.
    for fname in filenames:
        if fname in valid_fnames:
            return fname

    # Check for partial match (basename match).
    for fname in filenames:
        for valid_name in valid_fnames:
            if fname == Path(valid_name).name:
                return valid_name

    # Perform fuzzy matching with valid_fnames.
    for fname in filenames:
        close_matches = difflib.get_close_matches(fname, valid_fnames, n=1, cutoff=0.8)
        if len(close_matches) == 1:
            return close_matches[0]

    # If no fuzzy match, look for a file w/extension.
    for fname in filenames:
        if "." in fname:
            return fname

    if filenames:
        return filenames[0]

    return None


def find_original_update_blocks(
    content: str,
    fence: Fence = DEFAULT_FENCE,
    valid_fnames: Sequence[str] | None = None,
) -> Iterator[ParsedEditBlock]:
    lines = content.splitlines(keepends=True)
    i = 0
    current_filename: str | None = None

    head_pattern = re.compile(HEAD)
    divider_pattern = re.compile(DIVIDER)
    updated_pattern = re.compile(UPDATED)

    while i < len(lines):
        line = lines[i]

        if head_pattern.match(line.strip()):
            try:
                # If next line after HEAD exists and is DIVIDER, it's a new file.
                if i + 1 < len(lines) and divider_pattern.match(lines[i + 1].strip()):
                    filename = find_filename(lines[max(0, i - 3) : i], fence, None)
                else:
                    filename = find_filename(
                        lines[max(0, i - 3) : i], fence, valid_fnames
                    )

                if not filename:
                    if current_filename:
                        filename = current_filename
                    else:
                        raise MissingFilenameError(
                            missing_filename_err.format(fence=fence)
                        )

                current_filename = filename

                original_text: list[str] = []
                i += 1
                while i < len(lines) and not divider_pattern.match(lines[i].strip()):
                    original_text.append(lines[i])
                    i += 1

                if i >= len(lines) or not divider_pattern.match(lines[i].strip()):
                    raise ParseError(f"Expected `{DIVIDER_ERR}`")

                updated_text: list[str] = []
                i += 1
                while i < len(lines) and not (
                    updated_pattern.match(lines[i].strip())
                    or divider_pattern.match(lines[i].strip())
                ):
                    updated_text.append(lines[i])
                    i += 1

                if i >= len(lines) or not (
                    updated_pattern.match(lines[i].strip())
                    or divider_pattern.match(lines[i].strip())
                ):
                    raise ParseError(f"Expected `{UPDATED_ERR}` or `{DIVIDER_ERR}`")

                yield filename, "".join(original_text), "".join(updated_text)
            except ValueError as exc:
                processed = "".join(lines[: i + 1])
                err = exc.args[0]
                raise ParseError(f"{processed}\n^^^ {err}") from exc

        i += 1


def parse_edit_blocks(
    content: str,
    fence: Fence = DEFAULT_FENCE,
    valid_fnames: Sequence[str] | None = None,
) -> ParseResult:
    edits = [
        EditBlock(path=block[0], original=block[1], updated=block[2])
        for block in find_original_update_blocks(
            content, fence=fence, valid_fnames=valid_fnames
        )
    ]
    return ParseResult(edits=edits)
