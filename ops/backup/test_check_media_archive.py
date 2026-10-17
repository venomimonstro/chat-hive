"""Offline regression tests for backup media archive safety."""
import io
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_media_archive import inspect

class TestMediaArchive(unittest.TestCase):
    def archive(self, directory, entries):
        path = Path(directory) / "media.tar.gz"
        with tarfile.open(path, "w:gz") as stream:
            for name, kind in entries:
                item = tarfile.TarInfo(name)
                if kind == "dir":
                    item.type = tarfile.DIRTYPE
                elif kind == "symlink":
                    item.type = tarfile.SYMTYPE
                    item.linkname = "/etc/passwd"
                elif kind == "hardlink":
                    item.type = tarfile.LNKTYPE
                    item.linkname = "other"
                else:
                    item.size = 5
                stream.addfile(item, io.BytesIO(b"hello") if kind == "file" else None)
        return str(path)

    def test_good(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self.archive(directory, [(".", "dir"), ("./photos", "dir"), ("./photos/a.jpg", "file")])
            self.assertEqual(inspect(path), (3, 5))

    def test_rejects_unsafe_members(self):
        cases = [
            [("../outside", "file")], [("/etc/passwd", "file")],
            [("./link", "symlink")], [("./link", "hardlink")],
            [("./file", "file"), ("./file/child", "file")],
            [("./repeat", "file"), ("./repeat", "file")],
        ]
        for members in cases:
            with self.subTest(members=members), tempfile.TemporaryDirectory() as directory:
                with self.assertRaises(ValueError):
                    inspect(self.archive(directory, members))

if __name__ == "__main__":
    unittest.main()
