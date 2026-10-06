import unittest

from search_replace.parser import find_filename, find_original_update_blocks


class TestParser(unittest.TestCase):
    def test_find_filename(self) -> None:
        fence = ("```", "```")
        valid_fnames = ["file1.py", "file2.py", "dir/file3.py", r"\windows\__init__.py"]

        lines = ["file1.py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "file1.py")

        lines = ["```python", "file3.py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "dir/file3.py")

        lines = ["```", "invalid_file.py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "invalid_file.py")

        lines = ["```python", "file1.py", "```", "```", "file2.py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "file2.py")

        lines = ["# file1.py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "file1.py")

        lines = ["file1_py", "```"]
        self.assertEqual(find_filename(lines, fence, valid_fnames), "file1.py")

        lines = [r"\windows__init__.py", "```"]
        self.assertEqual(
            find_filename(lines, fence, valid_fnames), r"\windows\__init__.py"
        )

    def test_find_original_update_blocks(self) -> None:
        edit = """
Here's the change:

```text
foo.txt
<<<<<<< SEARCH
Two
=======
Tooooo
>>>>>>> REPLACE
```

Hope you like it!
"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(edits, [("foo.txt", "Two\n", "Tooooo\n")])

    def test_find_original_update_blocks_missing_filename(self) -> None:
        edit = """
```text
<<<<<<< SEARCH
Two
=======
Tooooo
>>>>>>> REPLACE
"""
        with self.assertRaises(ValueError) as ctx:
            _ = list(find_original_update_blocks(edit))
        self.assertIn("filename", str(ctx.exception))

    def test_find_original_update_blocks_unclosed(self) -> None:
        edit = """
```text
foo.txt
<<<<<<< SEARCH
Two
=======
Tooooo
"""
        with self.assertRaises(ValueError) as ctx:
            _ = list(find_original_update_blocks(edit))
        self.assertIn("Expected `>>>>>>> REPLACE` or `=======`", str(ctx.exception))

    def test_find_original_update_blocks_no_final_newline(self) -> None:
        edit = """aider/coder.py
<<<<<<< SEARCH
old
=======
new
>>>>>>> REPLACE"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(edits, [("aider/coder.py", "old\n", "new\n")])

    def test_find_original_update_blocks_multiple_same_file(self) -> None:
        edit = """
```text
foo.txt
<<<<<<< SEARCH
one
=======
two
>>>>>>> REPLACE

...

<<<<<<< SEARCH
three
=======
four
>>>>>>> REPLACE
```
"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(
            edits,
            [
                ("foo.txt", "one\n", "two\n"),
                ("foo.txt", "three\n", "four\n"),
            ],
        )

    def test_find_original_update_blocks_quad_backticks(self) -> None:
        edit = """
foo.txt
```text
<<<<<<< SEARCH
=======
Tooooo
>>>>>>> REPLACE
```
"""
        quad_backticks = ("`" * 4, "`" * 4)
        edits = list(find_original_update_blocks(edit, fence=quad_backticks))
        self.assertEqual(edits, [("foo.txt", "", "Tooooo\n")])

    def test_find_original_update_blocks_quote_below_filename(self) -> None:
        edit = """
Here's the change:

foo.txt
```text
<<<<<<< SEARCH
Two
=======
Tooooo
>>>>>>> REPLACE
```

Hope you like it!
"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(edits, [("foo.txt", "Two\n", "Tooooo\n")])

    def test_incomplete_edit_block_missing_filename_continuation(self) -> None:
        # The second block has no filename; parser should inherit current_filename.
        edit = """
No problem! Here are the changes:

```python
tests/test_repomap.py
<<<<<<< SEARCH
    def test_check_for_ctags_failure(self):
        with patch("subprocess.run") as mock_run:
            mock_run.side_effect = Exception("ctags not found")
=======
    def test_check_for_ctags_failure(self):
        with patch("subprocess.check_output") as mock_check_output:
            mock_check_output.side_effect = Exception("ctags not found")
>>>>>>> REPLACE

<<<<<<< SEARCH
    def test_check_for_ctags_success(self):
        with patch("subprocess.run") as mock_run:
=======
    def test_check_for_ctags_success(self):
        with patch("subprocess.check_output") as mock_check_output:
>>>>>>> REPLACE
```
"""
        edit_blocks = list(find_original_update_blocks(edit))
        self.assertEqual(len(edit_blocks), 2)
        self.assertEqual(edit_blocks[0][0], "tests/test_repomap.py")
        self.assertEqual(edit_blocks[1][0], "tests/test_repomap.py")

    def test_deepseek_coder_v2_filename_mangling(self) -> None:
        # DeepSeek sometimes puts a space before the opening fence.
        edit = """
Here's the change:

 ```python
foo.txt
```
```python
<<<<<<< SEARCH
one
=======
two
>>>>>>> REPLACE
```

Hope you like it!
"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(edits, [("foo.txt", "one\n", "two\n")])

    def test_new_file_created_in_same_folder(self) -> None:
        edit = """
Here's the change:

path/to/a/file2.txt
```python
<<<<<<< SEARCH
=======
three
>>>>>>> REPLACE
```

another change

path/to/a/file1.txt
```python
<<<<<<< SEARCH
one
=======
two
>>>>>>> REPLACE
```

Hope you like it!
"""
        edits = list(
            find_original_update_blocks(edit, valid_fnames=["path/to/a/file1.txt"])
        )
        self.assertEqual(
            edits,
            [
                ("path/to/a/file2.txt", "", "three\n"),
                ("path/to/a/file1.txt", "one\n", "two\n"),
            ],
        )

    def test_find_original_update_blocks_with_sh_language_identifier(self) -> None:
        # `sh` as a language identifier should be treated as a regular edit block.
        edit = """
Here's a shell script:

```sh
test_hello.sh
<<<<<<< SEARCH
=======
#!/bin/bash
echo "$1"
exit 0
>>>>>>> REPLACE
```
"""
        edits = list(find_original_update_blocks(edit))
        self.assertEqual(len(edits), 1)
        self.assertEqual(edits[0][0], "test_hello.sh")
        self.assertEqual(edits[0][1], "")
        self.assertIn("#!/bin/bash", edits[0][2])
        self.assertIn('echo "$1"', edits[0][2])

    def test_find_original_update_blocks_with_csharp_language_identifier(self) -> None:
        edit = """
Here's a C# code change:

```csharp
Program.cs
<<<<<<< SEARCH
Console.WriteLine("Hello World!");
=======
Console.WriteLine("Hello, C# World!");
>>>>>>> REPLACE
```
"""
        edits = list(find_original_update_blocks(edit))
        search_text = 'Console.WriteLine("Hello World!");\n'
        replace_text = 'Console.WriteLine("Hello, C# World!");\n'
        self.assertEqual(edits, [("Program.cs", search_text, replace_text)])


if __name__ == "__main__":
    unittest.main()
