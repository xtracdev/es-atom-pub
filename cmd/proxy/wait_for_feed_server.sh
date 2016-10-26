#!/bin/bash

set -e

host="$1"
port="$2"
shift
shift
cmd="$@"

echo The command is $cmd

until nc -z $host $port; do
  >&2 echo "Feed server is unavailable - sleeping"
  sleep 3
done

>&2 echo "Feed server is up - executing command"
exec nginx -g 'daemon off;'