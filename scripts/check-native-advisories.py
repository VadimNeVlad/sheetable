"""Temporary gate for known upstream CVEs missing from Alpine's advisory DB.

Owner: repository maintainer. Reviewed 2026-09-27. Evidence and removal criteria:
docs/security-remediation-2026-09-27.md. This is not a general-purpose scanner.
"""

import hashlib
import platform
import re
import subprocess
from datetime import date
from importlib.metadata import version as python_version
from pathlib import Path

from PIL import features

pillow_tiff_version = features.version("libtiff")


def version_tuple(version):
    match = re.match(r"(\d+)\.(\d+)\.(\d+)(?:-r\d+)?$", version)
    if not match:
        raise SystemExit(f"Cannot evaluate native package version: {version}")
    return tuple(int(part) for part in match.groups())


installed = {}
for record in Path("/lib/apk/db/installed").read_text().split("\n\n"):
    fields = dict(line.split(":", 1) for line in record.splitlines() if ":" in line)
    if "P" in fields and "V" in fields:
        installed[fields["P"]] = fields["V"]

# Conservative upstream version floors. A distro backport requires reviewing
# the actual patch and updating this gate; an empty Trivy report is insufficient.
advisories = (
    ("tiff", (4, 7, 2), "CVE-2026-36849"),
    ("libx11", (1, 8, 14), "CVE-2026-88806"),
    ("libxrender", (0, 9, 13), "CVE-2026-88807"),
)


def reviewed_path_applies():
    """Time-limited non-applicability, not a claim that libraries are patched.

    Pin the reviewed source and dependency graph. Any change needs a fresh
    assessment; a different route must not inherit this result automatically.
    """
    if date.today() >= date(2026, 10, 27) or platform.machine() != "x86_64":
        return False
    reviewed_packages = {
        "poppler": "25.12.0-r1",
        "poppler-utils": "25.12.0-r1",
        "tiff": "4.7.1-r0",
        "libx11": "1.8.13-r0",
        "libxrender": "0.9.12-r0",
    }
    if any(installed.get(name) != value for name, value in reviewed_packages.items()):
        return False
    if python_version("pdf2image") != "1.17.0" or python_version("Pillow") != "12.3.0":
        return False
    source = Path("/app/app.py").read_bytes().replace(b"\r\n", b"\n")
    if hashlib.sha256(source).hexdigest() != "8fd381fd894fdd9b77cb1992c32d432b9f42e6d58dc2ab9f9698cc18a200101a":
        return False
    for command in ("/usr/bin/pdfinfo", "/usr/bin/pdftoppm"):
        dependencies = subprocess.run(
            ["ldd", command], check=True, text=True, capture_output=True
        ).stdout
        if "libX11" in dependencies or "libXrender" in dependencies:
            return False
    # Pillow's wheel bundles its own TIFF 4.7.1. The reviewed route decodes
    # Poppler-generated PPM and writes PNG; it never decodes uploaded TIFF.
    return pillow_tiff_version == "4.7.1"


blocked = []
for package, fixed, cve in advisories:
    if package in installed and version_tuple(installed[package]) < fixed:
        blocked.append(f"{cve}: {package} {installed[package]}; upstream floor {'.'.join(map(str, fixed))}")
if pillow_tiff_version is not None and version_tuple(pillow_tiff_version) < (4, 7, 2):
    blocked.append(f"CVE-2026-36849: Pillow bundled libtiff {pillow_tiff_version}; upstream floor 4.7.2")
if blocked:
    if not reviewed_path_applies():
        raise SystemExit("Known native advisories still require resolution:\n" + "\n".join(blocked))
    print("Reviewed PDF-to-PPM-to-PNG path: three advisories not applicable.")
    print("Libraries remain unpatched. Assessment expires 2026-10-27; see remediation record.")
print("Known native advisory gate passed.")
