#!/usr/bin/env python3
"""Fetch Harbor's official v2 OpenAPI document at an immutable upstream commit."""

from pathlib import Path
import subprocess
import tempfile
from urllib.request import urlopen


ROOT = Path(__file__).resolve().parents[1]
REPOSITORY = "https://github.com/goharbor/harbor.git"
BRANCH = "refs/heads/main"


def main() -> None:
    result = subprocess.run(
        ["git", "ls-remote", REPOSITORY, BRANCH],
        check=True,
        capture_output=True,
        text=True,
    )
    revision = result.stdout.split()[0]
    url = f"https://raw.githubusercontent.com/goharbor/harbor/{revision}/api/v2.0/swagger.yaml"
    with urlopen(url, timeout=30) as response:
        spec = response.read()
    if b"swagger: '2.0'" not in spec[:256] or b"title: Harbor API" not in spec[:2048]:
        raise RuntimeError("upstream document is not Harbor's Swagger 2.0 API spec")

    write_atomically(ROOT / "hack/harbor-openapi.yaml", spec)
    write_atomically(ROOT / "hack/harbor-openapi.revision", f"{revision}\n".encode())
    print(f"Updated Harbor API specification to {revision}")


def write_atomically(path: Path, data: bytes) -> None:
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as temporary:
        temporary.write(data)
        temporary_path = Path(temporary.name)
    temporary_path.replace(path)


if __name__ == "__main__":
    main()
