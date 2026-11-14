"""Validate local authentication heartbeats without contacting the game.

Exit 0: authentication is recovering/waiting; suppress dependent game-stall checks.
Exit 1: every character is online; apply ordinary game-stall checks.
Exit 2: absent, stale, invalid, stopped, failed or hung; supervisor handles failure.
Process liveness must still be checked independently by the supervisor.
"""
import json
from datetime import datetime, timezone
from pathlib import Path
import sys


MAX_BYTES = 64 * 1024
ATTEMPT_PHASES = {"starting", "queued", "authenticating", "connecting"}


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("timestamp required")
    result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if result.tzinfo is None:
        raise ValueError("timezone required")
    return result


def evaluate_status(snapshot, now):
    if not isinstance(snapshot, dict) or snapshot.get("version") != 1:
        raise ValueError("unsupported status")
    if type(snapshot.get("pid")) is not int or snapshot["pid"] <= 0:
        raise ValueError("invalid process identity")
    age = (now - timestamp(snapshot.get("updated_at"))).total_seconds()
    if not -5 <= age <= 60:
        raise ValueError("stale or future heartbeat")
    characters = snapshot.get("characters")
    if not isinstance(characters, list) or not characters:
        raise ValueError("character states required")
    waiting = False
    seen = set()
    for character in characters:
        if not isinstance(character, dict):
            raise ValueError("invalid character")
        ident = character.get("id")
        if type(ident) is not int or ident <= 0 or ident in seen:
            raise ValueError("invalid character identity")
        seen.add(ident)
        phase = character.get("phase")
        if phase == "online":
            continue
        if phase == "session_required":
            waiting = True
        elif phase in {"backoff", "blocked"}:
            # A timer can expire just before the worker publishes its next phase.
            # One heartbeat of grace avoids restarting during that transition.
            if (now - timestamp(character.get("next_attempt_at"))).total_seconds() > 10:
                raise ValueError("retry schedule expired")
            waiting = True
        elif phase in ATTEMPT_PHASES:
            remaining = (timestamp(character.get("attempt_deadline")) - now).total_seconds()
            if not 0 < remaining <= 65:
                raise ValueError("attempt deadline invalid or expired")
            waiting = True
        else:
            raise ValueError("authentication stopped or failed")
    return waiting


def inspect(paths, now=None):
    now = now or datetime.now(timezone.utc)
    waiting = False
    try:
        for path in paths:
            with Path(path).open("rb") as stream:
                data = stream.read(MAX_BYTES + 1)
            if len(data) > MAX_BYTES:
                raise ValueError("status exceeds limit")
            waiting = evaluate_status(json.loads(data), now) or waiting
    except (OSError, ValueError, TypeError, OverflowError):
        # Never echo file contents, paths or exception messages that may contain secrets.
        return 2
    return 0 if waiting else 1


def main():
    if len(sys.argv) < 2:
        return 2
    return inspect(sys.argv[1:])


if __name__ == "__main__":
    sys.exit(main())
