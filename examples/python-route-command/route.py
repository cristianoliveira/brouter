#!/usr/bin/env python3
"""Example brouter route command using only Python's standard library."""

from datetime import datetime, time
import re
import sys

# These are intentionally invented example domains.
WORK = re.compile(rb"meetings\.example\.com|issues\.example\.com")
DEV = re.compile(rb"localhost|dev\.example\.test")
WORK_START = time(9, 0)
WORK_END = time(17, 0)


def is_working_hours(now: datetime) -> bool:
    """Return whether local time is Monday-Friday, 09:00-17:00."""
    return now.weekday() < 5 and WORK_START <= now.time() < WORK_END


def choose_target(url: bytes, now: datetime) -> bytes:
    """Choose a configured target without reading stdin or the clock."""
    is_work_url = WORK.search(url) is not None
    is_dev_url = DEV.search(url) is not None

    if is_work_url and is_working_hours(now):
        return b"work"
    if is_dev_url:
        return b"dev"
    if is_work_url:
        # Return personal explicitly. @default would re-enable static rules.
        return b"personal"
    return b"@default"


def main() -> None:
    url = sys.stdin.buffer.read()
    if url.endswith(b"\n"):
        url = url[:-1]
    sys.stdout.buffer.write(choose_target(url, datetime.now()) + b"\n")


if __name__ == "__main__":
    main()
