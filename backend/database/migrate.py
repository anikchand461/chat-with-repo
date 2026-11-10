"""
Bring an existing database up to date with the models (new nullable columns
and indexes only - no table is dropped or recreated, no row is changed).

Importing backend.database.db already does this on every backend start, so
running it by hand is optional; it shows exactly what was changed:

    DATABASE_URL=... .venv/bin/python -m backend.database.migrate
"""

from sqlalchemy import inspect

from backend.database.db import MIGRATIONS_APPLIED, engine


def main():
    if MIGRATIONS_APPLIED:
        for statement in MIGRATIONS_APPLIED:
            print("applied:", statement)
    else:
        print("database already up to date")

    columns = {c["name"] for c in inspect(engine).get_columns("users")}
    missing = {"google_sub", "google_name", "google_picture", "avatar_data", "avatar_type"} - columns
    if missing:
        raise SystemExit(f"users is still missing: {', '.join(sorted(missing))}")
    print("users columns ok:", ", ".join(sorted(columns)))


if __name__ == "__main__":
    main()
