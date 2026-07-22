import math
from difflib import SequenceMatcher


def replace_closest_edit_distance(
    whole_lines: list[str],
    part: str,
    part_lines: list[str],
    replace_lines: list[str],
) -> str | None:
    similarity_thresh = 0.8

    max_similarity = 0.0
    most_similar_chunk_start = -1
    most_similar_chunk_end = -1

    scale = 0.1
    min_len = math.floor(len(part_lines) * (1 - scale))
    max_len = math.ceil(len(part_lines) * (1 + scale))

    for length in range(min_len, max_len):
        for i in range(len(whole_lines) - length + 1):
            chunk_lines = whole_lines[i : i + length]
            chunk = "".join(chunk_lines)

            similarity = SequenceMatcher(None, chunk, part).ratio()

            if similarity > max_similarity and similarity:
                max_similarity = similarity
                most_similar_chunk_start = i
                most_similar_chunk_end = i + length

    if max_similarity < similarity_thresh:
        return None

    modified_whole = (
        whole_lines[:most_similar_chunk_start]
        + replace_lines
        + whole_lines[most_similar_chunk_end:]
    )
    return "".join(modified_whole)


def find_similar_lines(
    search_lines: str, content_lines: str, threshold: float = 0.6
) -> str:
    search_lines_list = search_lines.splitlines()
    content_lines_list = content_lines.splitlines()

    best_ratio = 0.0
    best_match: list[str] | None = None
    best_match_i = -1

    for i in range(len(content_lines_list) - len(search_lines_list) + 1):
        chunk = content_lines_list[i : i + len(search_lines_list)]
        ratio = SequenceMatcher(None, search_lines_list, chunk).ratio()
        if ratio > best_ratio:
            best_ratio = ratio
            best_match = chunk
            best_match_i = i

    if best_ratio < threshold or best_match is None:
        return ""

    if (
        best_match
        and search_lines_list
        and best_match[0] == search_lines_list[0]
        and best_match[-1] == search_lines_list[-1]
    ):
        return "\n".join(best_match)

    context_lines = 5
    best_match_end = min(
        len(content_lines_list), best_match_i + len(search_lines_list) + context_lines
    )
    best_match_i = max(0, best_match_i - context_lines)

    best = content_lines_list[best_match_i:best_match_end]
    return "\n".join(best)
