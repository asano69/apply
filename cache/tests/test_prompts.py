import unittest

from search_replace.prompts import (
    EditBlockFencedPrompts,
    FewShotExampleMessages,
    get_example_messages,
    render_system_prompt,
)


class TestRenderSystemPrompt(unittest.TestCase):
    def test_returns_string(self):
        self.assertIsInstance(render_system_prompt(), str)

    def test_contains_search_replace_instructions(self):
        self.assertIn("SEARCH/REPLACE", render_system_prompt())

    def test_contains_reminder_rules(self):
        self.assertIn("SEARCH/REPLACE block* Rules", render_system_prompt())

    def test_fence_placeholder_expanded(self):
        result = render_system_prompt(fence=("```", "```"))
        self.assertNotIn("{fence[0]}", result)
        self.assertNotIn("{fence[1]}", result)

    def test_final_reminders_injected(self):
        result = render_system_prompt(final_reminders="Always add type hints.")
        self.assertIn("Always add type hints.", result)

    def test_quad_backtick_reminder_injected(self):
        result = render_system_prompt(quad_backtick_reminder="Use 4 backticks.")
        self.assertIn("Use 4 backticks.", result)

    def test_no_unresolved_placeholders(self):
        result = render_system_prompt(fence=("```", "```"))
        self.assertNotIn("{", result)


class TestGetExampleMessages(unittest.TestCase):
    def test_returns_named_tuple(self):
        msgs = get_example_messages()
        self.assertIsInstance(msgs, FewShotExampleMessages)

    def test_all_fields_are_strings(self):
        msgs = get_example_messages()
        self.assertIsInstance(msgs.first_user_message, str)
        self.assertIsInstance(msgs.first_assistant_message, str)
        self.assertIsInstance(msgs.second_user_message, str)
        self.assertIsInstance(msgs.second_assistant_message, str)

    def test_fence_placeholder_expanded(self):
        fence = ("```", "```")
        msgs = get_example_messages(fence=fence)
        for field in msgs:
            self.assertNotIn("{fence[0]}", field)
            self.assertNotIn("{fence[1]}", field)

    def test_filename_inside_fence_in_assistant_messages(self):
        fence = ("```", "```")
        msgs = get_example_messages(fence=fence)
        self.assertIn("mathweb/flask/app.py", msgs.first_assistant_message)

    def test_field_count_matches_class_definition(self):
        self.assertEqual(
            len(FewShotExampleMessages._fields),
            len(EditBlockFencedPrompts.example_messages),
        )
