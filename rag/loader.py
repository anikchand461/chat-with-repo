import json
from pathlib import Path
from typing import Any


class JSONLoader:
    """
    Loads repository JSON data from a file.
    """

    def __init__(self, file_path: str):
        self.file_path = Path(file_path)

    def load(self) -> Any:
        """
        Load JSON file and return the parsed object.

        Returns:
            dict | list: Parsed JSON data
        """

        if not self.file_path.exists():
            raise FileNotFoundError(
                f"JSON file not found: {self.file_path}"
            )

        with self.file_path.open(
            "r",
            encoding="utf-8"
        ) as file:
            data = json.load(file)

        return data