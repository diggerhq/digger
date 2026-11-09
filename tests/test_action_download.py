"""Run the prebuilt action's Bash with real curl and a local release server.

Run with: python3 -m unittest discover -s tests -p 'test_action_download.py' -v
"""
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]
PAYLOAD = b'#!/usr/bin/env bash\nprintf "%s" "$INPUT_DIGGER_COMMAND" > "$GITHUB_WORKSPACE/executed"\n'


def download_script(ee="false", fips="false"):
    step = (ROOT / "action.yml").read_text().split(
        "    - name: download, install, and run digger\n", 1
    )[1].split("    - name:", 1)[0]
    script = textwrap.dedent(step.split("      run: |\n", 1)[1])
    for key, value in {"inputs.ee": ee, "inputs.fips": fips,
                       "runner.os": "Linux", "runner.arch": "X64"}.items():
        script = script.replace("${{ " + key + " }}", value)
    return script


class DownloadTest(unittest.TestCase):
    def run_download(self, *, failures=None, body=PAYLOAD, digest=None,
                     assets=None, ee="false", fips="false", platform=("Linux", "X64"),
                     command="digger plan", failure_mode="http"):
        counts = {"metadata": 0, "binary": 0}
        name = ("digger-ee-cli" if ee == "true" else "digger-cli")
        name += "-" + "-".join(platform) + ("-fips" if fips == "true" else "")
        if assets is None:
            assets = [{"name": name, "digest": digest if digest is not None else
                       "sha256:" + hashlib.sha256(PAYLOAD).hexdigest()}]

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                kind = "metadata" if self.path.startswith("/repos/") else "binary"
                counts[kind] += 1
                if counts[kind] <= (failures or {}).get(kind, 0):
                    if failure_mode == "partial":
                        self.send_response(200)
                        self.send_header("Content-Length", str(len(PAYLOAD) + 100))
                        self.end_headers()
                        self.wfile.write(PAYLOAD[:20])
                        self.close_connection = True
                    else:
                        self.send_response(404)
                        self.end_headers()
                    return
                content = json.dumps({"assets": assets}).encode() if kind == "metadata" else body
                self.send_response(200)
                self.send_header("Content-Length", str(len(content)))
                self.end_headers()
                self.wfile.write(content)

            def log_message(self, *args):
                pass

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as temp:
                work = Path(temp)
                bindir = work / "bin"
                bindir.mkdir()
                # Only redirect URLs and shorten test backoff. Retry decisions stay in curl.
                wrapper = bindir / "curl"
                wrapper.write_text(textwrap.dedent(f'''\
                    #!/usr/bin/env bash
                    args=()
                    for arg in "$@"; do
                      case "$arg" in
                        https://api.github.com/*)
                          arg="http://127.0.0.1:{server.server_port}/${{arg#https://api.github.com/}}" ;;
                        https://github.com/*)
                          arg="http://127.0.0.1:{server.server_port}/${{arg#https://github.com/}}" ;;
                      esac
                      args+=("$arg")
                    done
                    exec "{shutil.which('curl')}" "${{args[@]}}" --retry-delay 1
                    '''))
                wrapper.chmod(0o755)
                # A PATH entry must never supersede the bytes we just verified.
                shadow = bindir / "digger"
                shadow.write_text('#!/usr/bin/env bash\ntouch "$GITHUB_WORKSPACE/shadowed"\n')
                shadow.chmod(0o755)
                env = dict(os.environ, PATH=str(bindir) + os.pathsep + os.environ["PATH"],
                           GITHUB_WORKSPACE=temp, DIGGER_VERSION="v0.6.150",
                           DIGGER_OS=platform[0], DIGGER_ARCH=platform[1],
                           DIGGER_RELEASE_TOKEN="", INPUT_DIGGER_COMMAND=command)
                result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c",
                                         download_script(ee, fips)], cwd=temp, env=env,
                                        capture_output=True, text=True, timeout=50)
                executed = (work / "executed").read_text() if (work / "executed").exists() else None
                self.assertFalse((work / "shadowed").exists(), result.stdout + result.stderr)
                return result, counts, executed
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_verified_binary_runs_for_both_commands_and_asset_variants(self):
        for command in ("digger plan", "digger apply"):
            for platform in (("Linux", "X64"), ("linux", "arm64"),
                             ("darwin", "arm64"), ("windows", "amd64.exe"), ("freebsd", "386")):
                for ee, fips in (("false", "false"), ("true", "false"), ("true", "true")):
                    with self.subTest(command=command, platform=platform, ee=ee, fips=fips):
                        result, _, executed = self.run_download(command=command, platform=platform,
                                                               ee=ee, fips=fips)
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                        self.assertEqual(executed, command)
                        self.assertIn("Verified Digger CLI v0.6.150", result.stdout)

    def test_recovers_from_transient_metadata_and_binary_errors(self):
        for failure_mode in ("http", "partial"):
            with self.subTest(failure_mode=failure_mode):
                result, counts, executed = self.run_download(
                    failures={"metadata": 1, "binary": 1}, failure_mode=failure_mode)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(counts, {"metadata": 2, "binary": 2})
                self.assertEqual(executed, "digger plan")

    def test_persistent_failure_stops_after_five_retries(self):
        for kind in ("metadata", "binary"):
            with self.subTest(kind=kind):
                result, counts, executed = self.run_download(failures={kind: 100})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(counts[kind], 6)
                self.assertIsNone(executed)

    def test_corrupted_binary_never_runs(self):
        result, _, executed = self.run_download(body=PAYLOAD + b'# corrupted\n')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SHA-256 mismatch", result.stdout)
        self.assertIsNone(executed)

    def test_empty_binary_never_runs(self):
        result, _, executed = self.run_download(body=b"", digest="sha256:" + hashlib.sha256(b"").hexdigest())
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("empty or doesn't exist", result.stdout)
        self.assertIsNone(executed)

    def test_missing_or_invalid_asset_digest_fails_closed(self):
        for digest in ("", "md5:" + "a" * 32, "sha256:1234", "sha256:" + "z" * 64):
            with self.subTest(digest=digest):
                result, counts, executed = self.run_download(digest=digest)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Missing or invalid SHA-256", result.stdout)
                self.assertEqual(counts["binary"], 0)
                self.assertIsNone(executed)
        for assets in ([], [{"name": "unrelated", "digest": "sha256:" + "a" * 64}],
                       [{"name": "digger-cli-Linux-X64", "digest": None}]):
            with self.subTest(assets=assets):
                result, counts, executed = self.run_download(assets=assets)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(counts["binary"], 0)
                self.assertIsNone(executed)


if __name__ == "__main__":
    unittest.main()
