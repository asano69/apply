import tempfile
import unittest
from pathlib import Path

from search_replace import EditBlock
from search_replace.apply import (
    apply_edits,
    replace_most_similar_chunk,
    strip_quoted_wrapping,
)
from search_replace.errors import ApplyError, PathEscapeError


class TestApply(unittest.TestCase):
    def test_strip_quoted_wrapping(self) -> None:
        input_text = "filename.ext\n```\nWe just want this content\nNot the filename and triple quotes\n```"
        expected_output = (
            "We just want this content\nNot the filename and triple quotes\n"
        )
        result = strip_quoted_wrapping(input_text, "filename.ext")
        self.assertEqual(result, expected_output)

    def test_strip_quoted_wrapping_no_filename(self) -> None:
        input_text = "```\nWe just want this content\nNot the triple quotes\n```"
        expected_output = "We just want this content\nNot the triple quotes\n"
        result = strip_quoted_wrapping(input_text)
        self.assertEqual(result, expected_output)

    def test_strip_quoted_wrapping_no_wrapping(self) -> None:
        input_text = "We just want this content\nNot the triple quotes\n"
        expected_output = "We just want this content\nNot the triple quotes\n"
        result = strip_quoted_wrapping(input_text)
        self.assertEqual(result, expected_output)

    def test_replace_part_with_missing_leading_whitespace(self) -> None:
        whole = "    line1\n    line2\n    line3\n"
        part = "line1\nline2\n"
        replace = "new_line1\nnew_line2\n"
        expected_output = "    new_line1\n    new_line2\n    line3\n"
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_part_with_just_some_missing_leading_whitespace(self) -> None:
        whole = "    line1\n    line2\n    line3\n"
        part = " line1\n line2\n"
        replace = " new_line1\n     new_line2\n"
        expected_output = "    new_line1\n        new_line2\n    line3\n"
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_part_with_missing_varied_leading_whitespace(self) -> None:
        whole = """
    line1
    line2
        line3
    line4
"""
        part = "line2\n    line3\n"
        replace = "new_line2\n    new_line3\n"
        expected_output = """
    line1
    new_line2
        new_line3
    line4
"""
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_multiple_matches(self) -> None:
        whole = "line1\nline2\nline1\nline3\n"
        part = "line1\n"
        replace = "new_line\n"
        expected_output = "new_line\nline2\nline1\nline3\n"
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_multiple_matches_missing_whitespace(self) -> None:
        whole = "    line1\n    line2\n    line1\n    line3\n"
        part = "line1\n"
        replace = "new_line\n"
        expected_output = "    new_line\n    line2\n    line1\n    line3\n"
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_part_with_missing_leading_whitespace_including_blank_line(
        self,
    ) -> None:
        whole = "    line1\n    line2\n    line3\n"
        part = "\n  line1\n  line2\n"
        replace = "  new_line1\n  new_line2\n"
        expected_output = "    new_line1\n    new_line2\n    line3\n"
        result = replace_most_similar_chunk(whole, part, replace)
        self.assertEqual(result, expected_output)

    def test_replace_with_fuzzy_matching(self) -> None:
        whole = "alpha = 1\nbeta = 2\ngamma = 3\n"
        part = "beta = 22\n"
        replace = "beta = 200\n"
        expected_output = "alpha = 1\nbeta = 200\ngamma = 3\n"

        result = replace_most_similar_chunk(whole, part, replace)

        self.assertEqual(result, expected_output)

    def test_replace_with_fuzzy_matching_below_threshold_returns_none(self) -> None:
        whole = "alpha = 1\nbeta = 2\ngamma = 3\n"
        part = "totally unrelated sentence here\n"
        replace = "beta = 200\n"

        result = replace_most_similar_chunk(whole, part, replace)

        self.assertIsNone(result)

    def test_create_new_file_with_other_file_in_chat(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            file1 = root / "file.txt"
            file1.write_text("one\ntwo\nthree\n", encoding="utf-8")

            edits = [
                EditBlock(
                    path="newfile.txt",
                    original="",
                    updated="creating a new file\n",
                )
            ]
            result = apply_edits(edits, root=root, chat_files=["file.txt"])
            self.assertEqual(len(result.updated_edits), 1)

            content = file1.read_text(encoding="utf-8")
            self.assertEqual(content, "one\ntwo\nthree\n")

            new_file_content = (root / "newfile.txt").read_text(encoding="utf-8")
            self.assertEqual(new_file_content, "creating a new file\n")

    def test_dry_run_does_not_write(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            file1 = root / "file.txt"
            original_content = "one\ntwo\nthree\n"
            file1.write_text(original_content, encoding="utf-8")

            edits = [EditBlock(path="file.txt", original="two\n", updated="new\n")]
            apply_edits(edits, root=root, dry_run=True)

            self.assertEqual(file1.read_text(encoding="utf-8"), original_content)

    def test_dry_run_raises_on_failed_block(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            (root / "file.txt").write_text("one\ntwo\nthree\n", encoding="utf-8")

            edits = [
                EditBlock(path="file.txt", original="does-not-exist\n", updated="new\n")
            ]

            with self.assertRaises(ApplyError):
                apply_edits(edits, root=root, dry_run=True)

    def test_dry_run_raises_before_any_write_on_partial_failure(self) -> None:
        """Blocks that would match must NOT be written when a later block fails."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            file1 = root / "file.txt"
            original_content = "one\ntwo\nthree\n"
            file1.write_text(original_content, encoding="utf-8")

            edits = [
                EditBlock(path="file.txt", original="two\n", updated="TWO\n"),
                EditBlock(path="file.txt", original="does-not-exist\n", updated="x\n"),
            ]

            with self.assertRaises(ApplyError):
                apply_edits(edits, root=root, dry_run=True)

            self.assertEqual(file1.read_text(encoding="utf-8"), original_content)

    def test_dry_run_error_message_says_would_apply(self) -> None:
        """The error message uses 'would apply' language, not 'were applied'."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            (root / "file.txt").write_text("one\ntwo\nthree\n", encoding="utf-8")

            edits = [
                EditBlock(path="file.txt", original="two\n", updated="TWO\n"),
                EditBlock(path="file.txt", original="does-not-exist\n", updated="x\n"),
            ]

            with self.assertRaises(ApplyError) as ctx:
                apply_edits(edits, root=root, dry_run=True)

            text = str(ctx.exception)
            self.assertIn("would apply successfully", text)
            self.assertNotIn("were applied successfully", text)

    def test_rejects_relative_path_escape(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            temp_path = Path(tmp_dir)
            root = temp_path / "root"
            root.mkdir()
            outside_file = temp_path / "outside.txt"
            edits = [EditBlock(path="../outside.txt", original="", updated="escaped\n")]

            with self.assertRaises(PathEscapeError):
                apply_edits(edits, root=root)

            self.assertFalse(outside_file.exists())

    def test_rejects_absolute_path_escape(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            temp_path = Path(tmp_dir)
            root = temp_path / "root"
            root.mkdir()
            outside_file = temp_path / "outside.txt"
            edits = [
                EditBlock(path=str(outside_file), original="", updated="escaped\n")
            ]

            with self.assertRaises(PathEscapeError):
                apply_edits(edits, root=root)

            self.assertFalse(outside_file.exists())

    def test_allows_absolute_path_within_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            inside_file = root / "inside.txt"
            edits = [EditBlock(path=str(inside_file), original="", updated="created\n")]

            result = apply_edits(edits, root=root)

            self.assertEqual(len(result.updated_edits), 1)
            self.assertEqual(inside_file.read_text(encoding="utf-8"), "created\n")

    def test_failed_apply_reports_exact_message_shape(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            file1 = root / "file.txt"
            file1.write_text("one\ntwo\nthree\n", encoding="utf-8")

            edits = [
                EditBlock(path="file.txt", original="does-not-exist\n", updated="new\n")
            ]

            with self.assertRaises(ApplyError) as ctx:
                _ = apply_edits(edits, root=root, chat_files=["file.txt"])

            text = str(ctx.exception)
            self.assertIn("SEARCH/REPLACE block failed to match", text)
            self.assertIn("SearchReplaceNoExactMatch", text)
            self.assertIn("The SEARCH section must exactly match", text)


if __name__ == "__main__":
    unittest.main()
