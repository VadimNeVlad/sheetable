import io
import os
import tempfile
from pathlib import Path

from flask import Flask, jsonify, request, send_file
from pdf2image import convert_from_path
from pdf2image.exceptions import (
    PDFInfoNotInstalledError,
    PDFPageCountError,
    PDFPopplerTimeoutError,
    PDFSyntaxError,
)


def _positive_int_from_env(name, default):
    value = int(os.getenv(name, default))
    if value <= 0:
        raise ValueError(f"{name} must be a positive integer")
    return value


app = Flask(__name__)
app.config["MAX_CONTENT_LENGTH"] = _positive_int_from_env(
    "PDF2PNG_MAX_REQUEST_BYTES", 20 * 1024 * 1024
)
app.config["PDF_CONVERSION_TIMEOUT_SECONDS"] = _positive_int_from_env(
    "PDF2PNG_CONVERSION_TIMEOUT_SECONDS", 25
)


def _error(message, status):
    return jsonify(error=message), status


@app.get("/health/live")
def health():
    return jsonify(status="OK")


@app.errorhandler(413)
def request_too_large(_exception):
    return _error("request exceeds the configured size limit", 413)


@app.post("/createthumbnail")
def create_thumbnail():
    upload = request.files.get("file")
    if upload is None or not upload.filename:
        return _error("multipart field 'file' is required", 400)

    if not request.form.get("name", "").strip():
        return _error("multipart field 'name' is required", 400)

    header = upload.stream.read(1024)
    upload.stream.seek(0)
    if b"%PDF-" not in header:
        return _error("uploaded file is not a PDF", 415)

    temporary_root = app.config.get("TEMP_DIR")

    try:
        with tempfile.TemporaryDirectory(
            prefix="sheetable-pdf-", dir=temporary_root
        ) as request_directory:
            input_path = Path(request_directory) / "input.pdf"
            upload.save(input_path)

            pages = convert_from_path(
                str(input_path),
                first_page=1,
                last_page=1,
                single_file=True,
                size=(380, 535),
                # Keep the reviewed headless PPM path; do not select Cairo/TIFF.
                fmt="ppm",
                use_pdftocairo=False,
                timeout=app.config["PDF_CONVERSION_TIMEOUT_SECONDS"],
            )
            if not pages:
                return _error("PDF contains no renderable pages", 422)

            thumbnail = io.BytesIO()
            pages[0].save(thumbnail, "PNG")
            thumbnail.seek(0)
    except PDFPopplerTimeoutError:
        app.logger.warning("PDF conversion exceeded its timeout")
        return _error("PDF conversion timed out", 504)
    except (PDFPageCountError, PDFSyntaxError):
        app.logger.info("PDF conversion rejected malformed input")
        return _error("PDF could not be processed", 422)
    except PDFInfoNotInstalledError:
        app.logger.exception("Poppler is unavailable")
        return _error("PDF conversion service is unavailable", 503)

    return send_file(
        thumbnail,
        download_name="thumbnail.png",
        mimetype="image/png",
    )
