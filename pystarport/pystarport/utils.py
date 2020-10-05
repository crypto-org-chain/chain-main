import configparser
import subprocess


def interact(cmd, ignore_error=False, input=None, **kwargs):
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        shell=True,
        **kwargs,
    )
    # begin = time.perf_counter()
    (stdout, _) = proc.communicate(input=input)
    # print('[%.02f] %s' % (time.perf_counter() - begin, cmd))
    if not ignore_error:
        assert proc.returncode == 0, f'{stdout.decode("utf-8")} ({cmd})'
    return stdout


def local_ip():
    return "127.0.0.1"


def write_ini(fp, cfg):
    ini = configparser.ConfigParser()
    for section, items in cfg.items():
        ini.add_section(section)
        sec = ini[section]
        sec.update(items)
    ini.write(fp)
