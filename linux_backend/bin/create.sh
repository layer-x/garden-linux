#!/bin/bash

[ -n "$DEBUG" ] && set -o xtrace
set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

cd $(dirname "${0}")/..

if [ $# -ne 1 ]; then
  echo "Usage: ${0} <instance_path>"
  exit 1
fi

target=${1}

if [ ! -d ${target} ]; then
  echo "\"${target}\" does not exist, aborting..."
  exit 1
fi

cp -r skeleton/* "${target}"/.
ln `pwd`/bin/wshd "${target}"/bin/
ln `pwd`/lib/pivotter "${target}"/lib/
ln `pwd`/bin/iodaemon "${target}"/bin/
ln `pwd`/bin/wsh "${target}"/bin/
ln `pwd`/bin/initc "${target}"/bin/
ln `pwd`/lib/hook "${target}"/lib/
ln `pwd`/bin/oom "${target}"/bin/
ln `pwd`/bin/nstar "${target}"/bin/
unshare -m "${target}"/setup.sh
echo ${target}
