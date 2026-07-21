import json
import os


def save_json(data, filename):
    """
    Save data as a JSON file.

    Parameters
    ----------
    data : dict
        Data to save.

    filename : str
        Relative or absolute path of the JSON file.
    """

    directory = os.path.dirname(filename)

    if directory:
        os.makedirs(directory, exist_ok=True)

    with open(filename, "w", encoding="utf-8") as f:
        json.dump(
            data,
            f,
            indent=4,
            ensure_ascii=False,
        )

    print(f"Saved: {os.path.abspath(filename)}")