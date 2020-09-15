import socket
import asyncio


async def execute(cmd, ignore_error=False, **kwargs):
    proc = await asyncio.create_subprocess_shell(cmd, **kwargs)
    # begin = time.perf_counter()
    retcode = await proc.wait()
    # print('[%.02f] %s' % (time.perf_counter() - begin, cmd))
    if not ignore_error:
        assert retcode == 0, cmd


async def interact(cmd, ignore_error=False, input=None, **kwargs):
    proc = await asyncio.create_subprocess_shell(
        cmd,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        **kwargs
    )
    # begin = time.perf_counter()
    (stdout, stderr) = await proc.communicate(input=input)
    # print('[%.02f] %s' % (time.perf_counter() - begin, cmd))
    if not ignore_error:
        assert proc.returncode == 0, f'{stdout.decode("utf-8")} ({cmd})'
    return stdout


def local_ip():
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        s.connect(("8.8.8.8", 80))
    except IOError:
        addr = '127.0.0.1'
    else:
        addr = s.getsockname()[0]
    finally:
        s.close()
    return addr
