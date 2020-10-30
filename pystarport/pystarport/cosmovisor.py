import os
import re
import signal
import subprocess
import sys
from pathlib import Path

reg = re.compile(r'UPGRADE "(.*)" NEEDED at (height: \d+|time: \S+): (\S*)')


def package_path():
    return Path(os.environ["PACKAGE_PATH"])


def inner():
    name = os.environ["DAEMON_NAME"]
    home = Path(os.environ["DAEMON_HOME"])
    current = home / "current"
    if not current.exists():
        current.symlink_to(package_path() / "v0")
    cmd = current / "bin" / name
    if len(sys.argv) < 2 or sys.argv[1] != "start":
        # no need to capture the outputs, just exec the binary
        os.execvp(cmd, sys.argv)
    else:
        assert (
            "--home" not in sys.argv
        ), "no need to pass --home when start with cosmovisor"
        args = sys.argv[1:] + ["--home", home]
        with subprocess.Popen([cmd] + args, stdout=subprocess.PIPE) as proc:
            for signame in ("SIGINT", "SIGTERM"):
                signal.signal(getattr(signal, signame), lambda *args: proc.terminate())
            for line in proc.stdout:
                line = line.decode("utf-8")
                print(line, flush=True, end="")
                m = reg.search(line)
                if m:
                    plan = m.group(1)
                    # upgrade the current link
                    current.unlink()
                    current.symlink_to(package_path() / plan)
                    proc.terminate()
                    return True


def main():
    while True:
        retry = inner()
        if not retry:
            break


if __name__ == "__main__":
    main()
