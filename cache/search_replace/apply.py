import re
from pathlib import Path
from typing import Sequence

from .errors import ApplyError, ParseError, PathEscapeError
from .fuzzy import find_similar_lines, replace_closest_edit_distance
from .parser import parse_edit_blocks
from .types import DEFAULT_FENCE, ApplyResult, EditBlock, Fence


def prep(content: str) -> tuple[str, list[str]]:
    if content and not content.endswith("\n"):
        content += "\n"
    lines = content.splitlines(keepends=True)
    return content, lines


def perfect_or_whitespace(
    whole_lines: list[str],
    part_lines: list[str],
    replace_lines: list[str],
) -> str | None:
    # Try for a perfect match.
    result = perfect_replace(whole_lines, part_lines, replace_lines)
    if result:
        return result

    # Try being flexible about leading whitespace.
    result = replace_part_with_missing_leading_whitespace(
        whole_lines, part_lines, replace_lines
    )
    if result:
        return result

    return None


def perfect_replace(
    whole_lines: list[str], part_lines: list[str], replace_lines: list[str]
) -> str | None:
    part_tup = tuple(part_lines)
    part_len = len(part_lines)

    for index in range(len(whole_lines) - part_len + 1):
        whole_tup = tuple(whole_lines[index : index + part_len])
        if part_tup == whole_tup:
            result = (
                whole_lines[:index] + replace_lines + whole_lines[index + part_len :]
            )
            return "".join(result)

    return None


def replace_most_similar_chunk(whole: str, part: str, replace: str) -> str | None:
    """Best efforts to find `part` lines in `whole` and replace them with `replace`."""
    whole, whole_lines = prep(whole)
    part, part_lines = prep(part)
    replace, replace_lines = prep(replace)

    result = perfect_or_whitespace(whole_lines, part_lines, replace_lines)
    if result:
        return result

    # Drop leading empty line, GPT sometimes adds them spuriously (issue #25).
    if len(part_lines) > 2 and not part_lines[0].strip():
        skip_blank_line_part_lines = part_lines[1:]
        result = perfect_or_whitespace(
            whole_lines, skip_blank_line_part_lines, replace_lines
        )
        if result:
            return result

    # Try to handle when it elides code with ...
    try:
        result = try_dotdotdots(whole, part, replace)
        if result:
            return result
    except ValueError:
        pass

    # Try fuzzy matching.
    result = replace_closest_edit_distance(whole_lines, part, part_lines, replace_lines)
    if result:
        return result

    return None


def try_dotdotdots(whole: str, part: str, replace: str) -> str | None:
    """
    See if the edit block has ... lines.
    If not, return none.

    If yes, try and do a perfect edit with the ... chunks.
    If there's a mismatch or otherwise imperfect edit, raise ValueError.

    If perfect edit succeeds, return the updated whole.
    """
    dots_re = re.compile(r"(^\s*\.\.\.\n)", re.MULTILINE | re.DOTALL)

    part_pieces = re.split(dots_re, part)
    replace_pieces = re.split(dots_re, replace)

    if len(part_pieces) != len(replace_pieces):
        raise ValueError("Unpaired ... in SEARCH/REPLACE block")

    if len(part_pieces) == 1:
        # No dots in this edit block, just return None.
        return None

    # Compare odd strings in part_pieces and replace_pieces.
    all_dots_match = all(
        part_pieces[i] == replace_pieces[i] for i in range(1, len(part_pieces), 2)
    )
    if not all_dots_match:
        raise ValueError("Unmatched ... in SEARCH/REPLACE block")

    part_pieces = [part_pieces[i] for i in range(0, len(part_pieces), 2)]
    replace_pieces = [replace_pieces[i] for i in range(0, len(replace_pieces), 2)]

    pairs = zip(part_pieces, replace_pieces)
    for part_piece, replace_piece in pairs:
        if not part_piece and not replace_piece:
            continue

        if not part_piece and replace_piece:
            if not whole.endswith("\n"):
                whole += "\n"
            whole += replace_piece
            continue

        if whole.count(part_piece) == 0:
            raise ValueError
        if whole.count(part_piece) > 1:
            raise ValueError

        whole = whole.replace(part_piece, replace_piece, 1)

    return whole


def replace_part_with_missing_leading_whitespace(
    whole_lines: list[str],
    part_lines: list[str],
    replace_lines: list[str],
) -> str | None:
    # GPT often messes up leading whitespace.
    # It usually does it uniformly across the ORIG and UPD blocks.
    # Either omitting all leading whitespace, or including only some of it.

    # Outdent everything in part_lines and replace_lines by the max fixed amount possible.
    leading: list[int] = [len(p) - len(p.lstrip()) for p in part_lines if p.strip()] + [
        len(p) - len(p.lstrip()) for p in replace_lines if p.strip()
    ]

    if leading:
        num_leading = min(leading)
        if num_leading:
            part_lines = [p[num_leading:] if p.strip() else p for p in part_lines]
            replace_lines = [p[num_leading:] if p.strip() else p for p in replace_lines]

    # Can we find an exact match not including the leading whitespace.
    num_part_lines = len(part_lines)

    for index in range(len(whole_lines) - num_part_lines + 1):
        add_leading = match_but_for_leading_whitespace(
            whole_lines[index : index + num_part_lines], part_lines
        )

        if add_leading is None:
            continue

        replace_lines = [
            add_leading + line if line.strip() else line for line in replace_lines
        ]
        whole_lines = (
            whole_lines[:index] + replace_lines + whole_lines[index + num_part_lines :]
        )
        return "".join(whole_lines)

    return None


def match_but_for_leading_whitespace(
    whole_lines: list[str], part_lines: list[str]
) -> str | None:
    num = len(whole_lines)

    # Does the non-whitespace all agree?
    if not all(whole_lines[i].lstrip() == part_lines[i].lstrip() for i in range(num)):
        return None

    # Are they all offset the same?
    add = set(
        whole_lines[i][: len(whole_lines[i]) - len(part_lines[i])]
        for i in range(num)
        if whole_lines[i].strip()
    )
    if len(add) != 1:
        return None

    return add.pop()


def strip_quoted_wrapping(
    res: str, fname: str | None = None, fence: Fence = DEFAULT_FENCE
) -> str:
    """
    Given an input string which may have extra "wrapping" around it, remove the wrapping.
    For example:

    filename.ext
    ```
    We just want this content
    Not the filename and triple quotes
    ```
    """
    if not res:
        return res

    res_lines = res.splitlines()

    if fname and res_lines[0].strip().endswith(Path(fname).name):
        res_lines = res_lines[1:]

    if (
        res_lines
        and res_lines[0].startswith(fence[0])
        and res_lines[-1].startswith(fence[1])
    ):
        res_lines = res_lines[1:-1]

    result = "\n".join(res_lines)
    if result and result[-1] != "\n":
        result += "\n"

    return result


def do_replace(
    fname: str | Path,
    content: str | None,
    before_text: str,
    after_text: str,
    fence: Fence | None = None,
) -> str | None:
    local_fence = fence or DEFAULT_FENCE
    before_text = strip_quoted_wrapping(before_text, str(fname), local_fence)
    after_text = strip_quoted_wrapping(after_text, str(fname), local_fence)
    path = Path(fname)

    # Does it want to make a new file?
    if not path.exists() and not before_text.strip():
        path.touch()
        content = ""

    if content is None:
        return None

    new_content: str | None
    if not before_text.strip():
        # Append to existing file, or start a new file.
        new_content = content + after_text
    else:
        new_content = replace_most_similar_chunk(content, before_text, after_text)

    return new_content


def apply_edits(
    edits: Sequence[EditBlock],
    root: str | Path,
    chat_files: Sequence[str | Path] | None = None,
    fence: Fence = DEFAULT_FENCE,
    dry_run: bool = False,
) -> ApplyResult:
    failed: list[EditBlock] = []
    passed: list[EditBlock] = []
    updated_edits: list[EditBlock] = []

    root_path = Path(root)
    fallback_files = _resolve_chat_files(root_path, chat_files)

    for edit in edits:
        path = edit.path
        original = edit.original
        updated = edit.updated

        full_path = _resolve_path(root_path, path)
        new_content: str | None = None

        if full_path.exists():
            content = full_path.read_text(encoding="utf-8")
            new_content = do_replace(full_path, content, original, updated, fence)
        elif not original.strip():
            new_content = do_replace(full_path, None, original, updated, fence)

        # If the edit failed, and this is not a "create a new file" with an empty original...
        # https://github.com/Aider-AI/aider/issues/2258
        if not new_content and original.strip():
            # Try patching any of the other files in the chat.
            for candidate_file in fallback_files:
                content = candidate_file.read_text(encoding="utf-8")
                new_content = do_replace(
                    candidate_file, content, original, updated, fence
                )
                if new_content:
                    path = _make_relative(candidate_file, root_path)
                    full_path = candidate_file
                    break

        updated_edit = EditBlock(path=path, original=original, updated=updated)
        updated_edits.append(updated_edit)

        if new_content:
            if not dry_run:
                full_path.write_text(new_content, encoding="utf-8")
            passed.append(edit)
        else:
            failed.append(edit)

    if not failed:
        return ApplyResult(updated_edits=updated_edits)

    blocks = "block" if len(failed) == 1 else "blocks"
    result = f"# {len(failed)} SEARCH/REPLACE {blocks} failed to match!\n"
    for edit in failed:
        path = edit.path
        original = edit.original
        updated = edit.updated

        full_path = _resolve_path(root_path, path)
        content = full_path.read_text(encoding="utf-8")

        result += f"""
## SearchReplaceNoExactMatch: This SEARCH block failed to exactly match lines in {path}
<<<<<<< SEARCH
{original}=======
{updated}>>>>>>> REPLACE

"""
        did_you_mean = find_similar_lines(original, content)
        if did_you_mean:
            result += f"""Did you mean to match some of these actual lines from {path}?

{fence[0]}
{did_you_mean}
{fence[1]}

"""

        if updated in content and updated:
            result += f"""Are you sure you need this SEARCH/REPLACE block?
The REPLACE lines are already in {path}!

"""

    result += (
        "The SEARCH section must exactly match an existing block of lines including all white"
        " space, comments, indentation, docstrings, etc\n"
    )
    if passed:
        passed_blocks = "block" if len(passed) == 1 else "blocks"
        if dry_run:
            result += f"""
# The other {len(passed)} SEARCH/REPLACE {passed_blocks} would apply successfully.
"""
        else:
            result += f"""
# The other {len(passed)} SEARCH/REPLACE {passed_blocks} were applied successfully.
Don't re-send them.
Just reply with fixed versions of the {blocks} above that failed to match.
"""

    raise ApplyError(
        message=result, failed=failed, passed=passed, updated_edits=updated_edits
    )


def _resolve_path(root_path: Path, path: str | Path) -> Path:
    resolved_root = root_path.resolve()
    file_path = Path(path)
    if file_path.is_absolute():
        resolved_path = file_path.resolve()
    else:
        resolved_path = (resolved_root / file_path).resolve()

    try:
        resolved_path.relative_to(resolved_root)
    except ValueError as exc:
        raise PathEscapeError(
            f"Refusing to edit path '{path}' because it resolves outside root "
            f"'{resolved_root}'."
        ) from exc

    return resolved_path


def _resolve_chat_files(
    root_path: Path,
    chat_files: Sequence[str | Path] | None,
) -> list[Path]:
    if chat_files is None:
        return []

    resolved_paths: list[Path] = []
    for chat_file in chat_files:
        resolved_paths.append(_resolve_path(root_path, chat_file))
    return resolved_paths


def _make_relative(path: Path, root_path: Path) -> str:
    try:
        return str(path.relative_to(root_path))
    except ValueError:
        return str(path)


def apply_diff(
    llm_response: str,
    root: str | Path,
    chat_files: Sequence[str | Path] | None = None,
    fence: Fence = DEFAULT_FENCE,
) -> ApplyResult:
    """Parse SEARCH/REPLACE blocks from an LLM response and apply them to disk.

    Convenience wrapper around ``parse_edit_blocks`` + ``apply_edits``.
    Raises ``ParseError`` if the response contains no valid blocks or has
    malformed syntax, and ``ApplyError`` if one or more blocks fail to match.
    """
    result = parse_edit_blocks(llm_response, fence=fence)
    if not result.edits:
        raise ParseError("No SEARCH/REPLACE blocks found in the LLM response.")
    return apply_edits(result.edits, root=root, chat_files=chat_files, fence=fence)
