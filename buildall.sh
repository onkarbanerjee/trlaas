#!/bin/bash

#In case of any convern/suggestion reach @sayan.sarkar@mavenir.com"

##############################################
#usage: ./buildall.sh <TPAAS_VERSION> [<CLEAN_BUILD_BUILDER_IMAGE>] [<DELETE_BUILDER_IMAGE>]
# TPAAS_VERSION :(Mandatory): This argument can be passed from jenkins job or manual, Artifact tpaas container image will be tagged with this version. 
# CLEAN_BUILD_BUILDER_IMAGE:(Optional): ["yes"|"no"] (Default: no). This will clean do clean build builder image.
# DELETE_BUILDER_IMAGE:(Optional): ["yes"|"no"] (Default: no). This will delete the image after artifacts are released.
##############################################

set -e

BUILD_DISTRO_IMAGE="rhel-ubi7-minimal"
BUILD_DISTRO_VERSION="v2.8"

GOLANG_VERSION="1.19.1"

BUILDER_IMAGE="tpaas-builder"
BUILDER_VERSION="v0.2"

BASE_IMAGE="rhel-ubi7-minimal"
BASE_VERSION="v2.8"

MICROSERVICE_NAME="tpaas"
MICROSERVICE_VERSION=$1

CLEAN_BUILD_BUILDER_IMAGE=$2
DELETE_BUILDER_IMAGE=$3

ARTIFACTS_PATH="./artifacts"
BUILDER_ARG=""
COMMON_ARG=""

if [[ -n "$CLEAN_BUILD_BUILDER_IMAGE" ]] && [[ "$CLEAN_BUILD_BUILDER_IMAGE" == "yes" ]]; then
echo -e "\e[1;32;40m[TPAAS-BUILD] Clean Build Builder Image...\e[0m"
BUILDER_ARG="--no-cache"
fi

echo -e "\e[1;32;40m[TPAAS-BUILD] Build Builder:$BUILDER_IMAGE, Version:$BUILDER_VERSION \e[0m"
docker build --rm $BUILDER_ARG \
             $COMMON_ARG \
             --build-arg BUILD_DISTRO_IMAGE=$BUILD_DISTRO_IMAGE \
             --build-arg BUILD_DISTRO_VERSION=$BUILD_DISTRO_VERSION \
             --build-arg GOLANG_VERSION=$GOLANG_VERSION \
             -f ./build_spec/tpaas-builder_dockerfile \
             -t $BUILDER_IMAGE:$BUILDER_VERSION .

##NANO SEC timestamp LABEL, to enable multiple build in same system
BUILDER_LABEL="tpaas-builder-$(date +%s%9N)"
echo -e "\e[1;32;40m[TPAAS-BUILD] Build MICROSERVICE_NAME:$MICROSERVICE_NAME, Version:$MICROSERVICE_VERSION \e[0m"
docker build --rm \
             $COMMON_ARG \
             --build-arg BUILDER_LABEL=$BUILDER_LABEL \
             --build-arg BUILDER_IMAGE=$BUILDER_IMAGE \
             --build-arg BUILDER_VERSION=$BUILDER_VERSION \
             --build-arg BUILD_DISTRO_IMAGE=$BUILD_DISTRO_IMAGE \
             --build-arg BUILD_DISTRO_VERSION=$BUILD_DISTRO_VERSION \
             --build-arg BASE_IMAGE=$BASE_IMAGE \
             --build-arg BASE_VERSION=$BASE_VERSION \
             -f ./build_spec/tpaas_dockerfile \
             -t $MICROSERVICE_NAME:$MICROSERVICE_VERSION .

echo -e "\e[1;32;40m[TPAAS-BUILD] Setting Artifacts Environment \e[0m"
rm -rf $ARTIFACTS_PATH
mkdir -p $ARTIFACTS_PATH
mkdir -p $ARTIFACTS_PATH/images
mkdir -p $ARTIFACTS_PATH/charts

echo -e "\e[1;32;40m[TPAAS-BUILD] Upating tpaas-service chart \e[0m"
cp -r ./charts/tpaas $ARTIFACTS_PATH/charts/.
sed -i "s/tpaas_tag/$1/" $ARTIFACTS_PATH/charts/tpaas/values.yaml

echo -e "\e[1;32;40m[TPAAS-BUILD] Releasing Artifacts... @$ARTIFACTS_PATH \e[0m"
docker save $MICROSERVICE_NAME:$MICROSERVICE_VERSION | gzip > $ARTIFACTS_PATH/images/$MICROSERVICE_NAME-$MICROSERVICE_VERSION.tar.gz 

echo -e "\e[1;32;40m[TPAAS-BUILD] Deleting Intermidiate Containers... \e[0m"
docker image prune -f --filter "label=IMAGE-TYPE=$BUILDER_LABEL"
docker rmi -f $MICROSERVICE_NAME:$MICROSERVICE_VERSION

if [[ -n "$DELETE_BUILDER_IMAGE" ]] && [[ "$DELETE_BUILDER_IMAGE" == "yes" ]]; then
echo -e "\e[1;32;40m[TPAAS-BUILD] Deleting Builder Image... \e[0m"
docker rmi -f $BUILDER_IMAGE:$BUILDER_VERSION
fi
