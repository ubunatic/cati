#!/bin/sh
set -u

mkdir -p /out
if make ci-test > /out/ci-test.log 2>&1
then status=0
else
	status=$?
fi
cat /out/ci-test.log
exit "$status"
