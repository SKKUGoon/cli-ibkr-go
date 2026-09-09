"""Real PTY regression checks: uv run --with pexpect --with pyte scripts/test_terminal.py."""
import os
from pathlib import Path
import signal
import tempfile
import time
import termios
import unittest

import pexpect
import pyte

BINARY = str(Path(__file__).resolve().parents[1] / "bin" / "ibkr")


class TerminalTests(unittest.TestCase):
    def start_prompt(self, width=80):
        home = tempfile.TemporaryDirectory()
        self.addCleanup(home.cleanup)
        child = pexpect.spawn(BINARY, ["configure"], env={**os.environ, "HOME": home.name, "TERM": "xterm-256color", "COLORFGBG": "15;0"}, encoding="utf-8", dimensions=(24, width), timeout=5)
        self.addCleanup(child.close, force=True)
        screen = pyte.Screen(width, 24)
        stream = pyte.Stream(screen)
        for _ in range(10):
            self.read_screen(child, stream)
            if "기존 .env 파일 경로" in "\n".join(screen.display):
                break
        self.assertIn("기존 .env 파일 경로", "\n".join(screen.display))
        return child, screen, stream

    def read_screen(self, child, stream):
        deadline = time.monotonic() + 0.5
        while time.monotonic() < deadline:
            try:
                chunk = child.read_nonblocking(65536, timeout=0.05)
                if "\x1b]11;?" in chunk:
                    child.send("\x1b]11;rgb:0000/0000/0000\x1b\\")
                if "\x1b[6n" in chunk:
                    child.send("\x1b[1;1R")
                stream.feed(chunk)
            except pexpect.TIMEOUT:
                continue
            except pexpect.EOF:
                break

    def assert_cancelled(self, child):
        child.expect(pexpect.EOF)
        flags = termios.tcgetattr(child.child_fd)[3]
        self.assertTrue(flags & termios.ICANON)
        self.assertTrue(flags & termios.ECHO)
        child.close()
        self.assertEqual(child.exitstatus, 130)

    def test_ctrl_c_and_escape(self):
        for key in ("\x03", "\x1b", "\x04"):
            with self.subTest(key=repr(key)):
                child, _, _ = self.start_prompt()
                child.send(key)
                self.assert_cancelled(child)

    def test_external_interrupt(self):
        child, _, _ = self.start_prompt()
        os.kill(child.pid, signal.SIGINT)
        self.assert_cancelled(child)

    def test_korean_backspace_arrows_and_prompt_boundary(self):
        for width in (80, 30):
            with self.subTest(width=width):
                child, screen, stream = self.start_prompt(width)
                child.send("한글ab")
                child.send("\x1b[D\x1b[3~")
                self.read_screen(child, stream)
                self.assertIn("> 한글a", "\n".join(screen.display))
                child.send("\x7f" * 12)
                self.read_screen(child, stream)
                self.assertIn("기존 .env 파일 경로", "\n".join(screen.display))
                self.assertNotIn("한글", "\n".join(screen.display))
                child.send("\x1b")
                self.assert_cancelled(child)

    def test_enter_advances_and_escape_cancels_without_saving(self):
        child, screen, stream = self.start_prompt()
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "test settings.env"
            source.write_text("IBKR_CONSUMER_KEY=test\nIBKR_ACCESS_TOKEN=test\nIBKR_ACCESS_TOKEN_SECRET=test\n")
            child.send(str(source) + "\r")
            self.read_screen(child, stream)
            self.assertIn("dhparam.pem", "\n".join(screen.display))
            child.send("\r")
            self.read_screen(child, stream)
            self.assertIn("private_encryption.pem", "\n".join(screen.display))
            child.send("\x1b")
            self.assert_cancelled(child)

    def test_long_input_and_resize(self):
        child, screen, stream = self.start_prompt(30)
        child.send("가" * 60)
        self.read_screen(child, stream)
        child.setwinsize(24, 50)
        screen.resize(24, 50)
        self.read_screen(child, stream)
        child.send("\x7f" * 70)
        self.read_screen(child, stream)
        self.assertIn("기존 .env 파일 경로", "\n".join(screen.display))
        child.send("\x03")
        self.assert_cancelled(child)


if __name__ == "__main__":
    unittest.main()
