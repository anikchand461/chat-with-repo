import json
import os


def build_structure(tree_items):
    structure = {}

    for item in tree_items:
        current = structure
        parts = item["path"].split("/")

        for i, part in enumerate(parts):
            is_last = i == len(parts) - 1

            if is_last:
                current.setdefault(part, {
                    "type": "directory" if item["type"] == "tree" else "file"
                })

                if item["type"] == "tree":
                    current[part].setdefault("children", {})

            else:
                node = current.setdefault(part, {
                    "type": "directory",
                    "children": {}
                })

                current = node["children"]

    return structure


def save_json(data, filename):
    """
    Save data as a JSON file.
    """

    # Automatically add the nested structure
    if "tree" in data:
        data["structure"] = build_structure(data["tree"])

    directory = os.path.dirname(filename)

    if directory:
        os.makedirs(directory, exist_ok=True)

    with open(filename, "w", encoding="utf-8", newline="\n") as f:
        json.dump(
            data,
            f,
            indent=4,
            ensure_ascii=False,
            sort_keys=False,
        )
        f.write("\n")

    print(f"Saved: {os.path.abspath(filename)}")
