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


def write_ini(fp, cfg):
    ini = configparser.RawConfigParser()
    for section, items in cfg.items():
        ini.add_section(section)
        sec = ini[section]
        sec.update(items)
    ini.write(fp)


def safe_cli_string(s):
    'wrap string in "", used for cli argument when contains spaces'
    if len(f"{s}".split()) > 1:
        return f'"{s}"'
    return f"{s}"


def build_cli_args_safe(*args, **kwargs):
    args = [safe_cli_string(arg) for arg in args if arg]
    for k, v in kwargs.items():
        if v is None:
            continue
        args.append("--" + k.strip("_").replace("_", "-"))
        args.append(safe_cli_string(v))
    return list(map(str, args))


def build_cli_args(*args, **kwargs):
    args = [arg for arg in args if arg is not None]
    for k, v in kwargs.items():
        if v is None:
            continue
        args.append("--" + k.strip("_").replace("_", "-"))
        args.append(v)
    return list(map(str, args))


def format_doc_string(**kwargs):
    def decorator(target):
        target.__doc__ = target.__doc__.format(**kwargs)
        return target

    return decorator
