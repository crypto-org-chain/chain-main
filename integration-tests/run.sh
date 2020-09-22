#!/usr/bin/env bash
cd "$(dirname "${BASH_SOURCE[0]}")"
pystarport serve config.yml > test.log &
PID=$!
pytest -v --ignore data
RETCODE=$?
kill $PID
wait
if [ $RETCODE -ne 0 ]; then
    cat test.log
else
    echo 'success'
fi
rm test.log
exit $RETCODE
