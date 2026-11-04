from pathlib import Path
import re
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/admin-core-getting-started.md"


class GettingStartedCwdTests(unittest.TestCase):
    def test_tutorial_shell_syntax(self) -> None:
        for block in re.findall(r"```bash\n(.*?)\n```", DOC.read_text(), re.DOTALL):
            subprocess.run(["bash", "-n"], input=block, text=True, check=True)
