#!/bin/sh
# The event queue's process tests (#1013) against TINHUB_TEST_DB. Usage: queue.sh WORKER (a built queue_worker).
set -eu
q=$1
# four workers and 10,000 jobs: every job is done and none is lost
"$q" setup 10000
"$q" work 30000 & a=$!
"$q" work 30000 & b=$!
"$q" work 30000 & c=$!
"$q" work 30000 & d=$!
wait $a $b $c $d
"$q" check 10000
# a worker killed in the middle of a job: the job is claimed again once its 2 s lease has passed
"$q" setup 0
"$q" sleeper
"$q" work 2000 & a=$!
sleep 1
kill -9 $a
wait $a 2>/dev/null || true
"$q" work 2000
"$q" checksleep
# a job over its memory budget fails alone; the others are done
"$q" setup 0
"$q" hog 200
"$q" work 30000 > /dev/null
"$q" checkhog 200
echo "PASS tinhub queue"
