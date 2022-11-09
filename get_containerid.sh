#!/bin/sh
ctrlen=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 5 | wc -c`
if [ "$ctrlen" == "65" ] ;
then
  export K8S_CONTAINER_ID=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 5`
else
  export K8S_CONTAINER_ID=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 4`
fi

if [ -z "$K8S_CONTAINER_ID" ]
then
  export K8S_CONTAINER_ID=`cidvar="$(cat /proc/1/cpuset | head -1)" && echo ${cidvar##*/}`
fi

echo $K8S_CONTAINER_ID