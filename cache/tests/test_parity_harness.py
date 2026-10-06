import difflib
import io
import re
import unittest
from pathlib import Path

from search_replace.parser import all_fences, find_original_update_blocks


def process_markdown(filename: str, fh: io.StringIO) -> None:
    file_path = Path(filename)
    if not file_path.exists():
        print(f"@@@ File '{filename}' not found.", "@" * 20, file=fh, flush=True)
        return

    content = file_path.read_text(encoding="utf-8")

    # Split the content into sections based on '####' headers.
    sections = re.split(r"(?=####\s)", content)

    for section in sections:
        if "editblock_coder.py" in section or "test_editblock.py" in section:
            continue

        if not section.strip():
            continue

        header = section.split("\n")[0].strip()
        section_content = "".join(section.splitlines(keepends=True)[1:])

        for fence in all_fences[1:] + all_fences[:1]:
            if "\n" + fence[0] in section_content:
                break

        try:
            blocks = list(find_original_update_blocks(section_content, fence))
        except ValueError as exc:
            print("\n\n@@@", header, "@" * 20, file=fh, flush=True)
            print(str(exc), file=fh, flush=True)
            continue

        if blocks:
            print("\n\n@@@", header, "@" * 20, file=fh, flush=True)

        for block in blocks:
            print("@@@ SEARCH:", block[0], "@" * 20, file=fh, flush=True)
            print(block[1], end="", file=fh, flush=True)
            print("@" * 20, file=fh, flush=True)
            print(block[2], end="", file=fh, flush=True)
            print("@@@ REPLACE", "@" * 20, file=fh, flush=True)


class TestParityHarness(unittest.TestCase):
    def test_process_markdown_matches_frozen_output(self) -> None:
        fixture_dir = Path(__file__).parent / "fixtures"
        input_file = fixture_dir / "chat-history.md"
        expected_output_file = fixture_dir / "chat-history-search-replace-gold.txt"

        output = io.StringIO()
        process_markdown(str(input_file), output)
        actual_output = output.getvalue()
        expected_output = expected_output_file.read_text(encoding="utf-8")

        if actual_output != expected_output:
            diff = difflib.unified_diff(
                expected_output.splitlines(keepends=True),
                actual_output.splitlines(keepends=True),
                fromfile=str(expected_output_file),
                tofile="actual output",
            )
            diff_text = "".join(diff)
            self.fail(f"Output doesn't match expected output. Diff:\n{diff_text}")


if __name__ == "__main__":
    unittest.main()
