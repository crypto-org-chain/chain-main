import asyncio


async def interact(cmd, ignore_error=False, input=None, **kwargs):
    proc = await asyncio.create_subprocess_shell(
        cmd,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        **kwargs,
    )
    # begin = time.perf_counter()
    (stdout, _) = await proc.communicate(input=input)
    # print('[%.02f] %s' % (time.perf_counter() - begin, cmd))
    if not ignore_error:
        assert proc.returncode == 0, f'{stdout.decode("utf-8")} ({cmd})'
    return stdout


def local_ip():
    return "127.0.0.1"
