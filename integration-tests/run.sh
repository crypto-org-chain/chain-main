#!/usr/bin/env bash
cd "$(dirname "${BASH_SOURCE[0]}")"
pystarport serve config.yml --quiet &
PID=$!

python -mpytest -v --ignore data
RETCODE=$?

echo 'quit starport...'
kill -TERM $PID
wait $PID

if [ $RETCODE -ne 0 ]; then
    cat data/node*.log
else
    echo 'success'
fi

exit $RETCODE
