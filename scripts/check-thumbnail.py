"""Check the rendered content of output/pdf/sheetable-smoke-test.pdf."""

import sys

from PIL import Image

with Image.open(sys.argv[1]) as thumbnail:
    if thumbnail.format != "PNG" or thumbnail.size != (380, 535):
        raise SystemExit("Smoke thumbnail must be a 380x535 PNG")
    # The fixture's heading/body text is above the coloured shapes. Missing
    # fallback fonts leave this region white even though a valid PNG is returned.
    text = thumbnail.crop((20, 20, 360, 100)).convert("L")
    histogram = text.histogram()
    if sum(histogram[:128]) < 100:
        raise SystemExit("Smoke thumbnail text is missing; check runtime fonts")

print("Smoke thumbnail dimensions and rendered text verified.")
