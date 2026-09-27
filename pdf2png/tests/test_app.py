import io
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from pdf2image.exceptions import PDFPageCountError, PDFPopplerTimeoutError

from app import app


class FakePage:
    def save(self, destination, image_format):
        if image_format != "PNG":
            raise AssertionError(f"unexpected image format: {image_format}")
        destination.write(b"\x89PNG\r\n\x1a\nthumbnail")


class PdfToPngServiceTests(unittest.TestCase):
    def setUp(self):
        app.config.update(
            TESTING=True,
            MAX_CONTENT_LENGTH=1024 * 1024,
            PDF_CONVERSION_TIMEOUT_SECONDS=25,
            TEMP_DIR=None,
        )
        self.client = app.test_client()

    def test_health_endpoint(self):
        response = self.client.get("/health/live")

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.get_json(), {"status": "OK"})

    def test_file_is_required(self):
        response = self.client.post("/createthumbnail", data={"name": "sheet"})

        self.assertEqual(response.status_code, 400)
        self.assertEqual(response.get_json()["error"], "multipart field 'file' is required")

    def test_name_is_required(self):
        response = self.client.post(
            "/createthumbnail",
            data={"file": (io.BytesIO(b"%PDF-1.4"), "sheet.pdf")},
        )

        self.assertEqual(response.status_code, 400)
        self.assertEqual(response.get_json()["error"], "multipart field 'name' is required")

    def test_non_pdf_upload_is_rejected(self):
        response = self.client.post(
            "/createthumbnail",
            data={
                "file": (io.BytesIO(b"not a PDF"), "sheet.txt"),
                "name": "sheet",
            },
        )

        self.assertEqual(response.status_code, 415)
        self.assertEqual(response.get_json()["error"], "uploaded file is not a PDF")

    @patch("app.convert_from_path", return_value=[FakePage()])
    def test_pdf_is_converted_without_using_name_as_a_path(self, convert):
        with tempfile.TemporaryDirectory() as temporary_root:
            app.config["TEMP_DIR"] = temporary_root

            response = self.client.post(
                "/createthumbnail",
                data={
                    "file": (io.BytesIO(b"%PDF-1.4\ncontent"), "sheet.pdf"),
                    "name": "../../outside",
                },
            )

            self.assertEqual(response.status_code, 200)
            self.assertEqual(response.content_type, "image/png")
            self.assertTrue(response.data.startswith(b"\x89PNG"))
            self.assertEqual(list(Path(temporary_root).iterdir()), [])

        conversion_options = convert.call_args.kwargs
        self.assertEqual(conversion_options["first_page"], 1)
        self.assertEqual(conversion_options["last_page"], 1)
        self.assertEqual(conversion_options["size"], (380, 535))
        self.assertEqual(conversion_options["fmt"], "ppm")
        self.assertFalse(conversion_options["use_pdftocairo"])
        self.assertEqual(conversion_options["timeout"], 25)

    @patch("app.convert_from_path", side_effect=PDFPageCountError("invalid PDF"))
    def test_malformed_pdf_returns_controlled_error(self, _convert):
        response = self.client.post(
            "/createthumbnail",
            data={
                "file": (io.BytesIO(b"%PDF-invalid"), "sheet.pdf"),
                "name": "sheet",
            },
        )

        self.assertEqual(response.status_code, 422)
        self.assertEqual(response.get_json()["error"], "PDF could not be processed")

    @patch("app.convert_from_path", side_effect=PDFPopplerTimeoutError("timeout"))
    def test_conversion_timeout_returns_gateway_timeout(self, _convert):
        response = self.client.post(
            "/createthumbnail",
            data={
                "file": (io.BytesIO(b"%PDF-1.4"), "sheet.pdf"),
                "name": "sheet",
            },
        )

        self.assertEqual(response.status_code, 504)
        self.assertEqual(response.get_json()["error"], "PDF conversion timed out")

    def test_oversized_request_is_rejected(self):
        app.config["MAX_CONTENT_LENGTH"] = 128

        response = self.client.post(
            "/createthumbnail",
            data={
                "file": (io.BytesIO(b"%PDF-" + b"x" * 1024), "sheet.pdf"),
                "name": "sheet",
            },
        )

        self.assertEqual(response.status_code, 413)
        self.assertEqual(
            response.get_json()["error"], "request exceeds the configured size limit"
        )


if __name__ == "__main__":
    unittest.main()
