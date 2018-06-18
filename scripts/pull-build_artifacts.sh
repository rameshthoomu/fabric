#!/bin/bash -e
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

############################################
# Pull "1.2.0-stable" docker images from nexus3
# Tag it as $ARCH-$VERSION (BASE_VERSION)
# Push tagged images to hyperledger dockerhub
#############################################

ORG_NAME=hyperledger/fabric
NEXUS_URL=nexus3.hyperledger.org:10001
STABLE_VERSION=1.2.0-stable
export RELEASE_VERSION=${RELEASE_VERSION:-1.2.0}
export IMAGES_LIST=(peer orderer ccenv tools)
export THIRDPARTY_IMAGES_LIST=(kafka couchdb zookeeper)

ARCH=$(go env GOARCH)	
if [ "$ARCH" = "amd64" ]; then
	ARCH=amd64
else
    ARCH=$(uname -m)
fi

cleanup() {
    # Cleanup docker images
    make clean || true
    docker images -q | xargs docker rmi -f || true
    
}

# pull fabric thirdparty, docker images and binaries         
pull_All() {

    echo "-------> pull thirdparty docker images"
    pull_Thirdparty
    echo "-------> pull binaries"
    pull_Binary
    echo "-------> pull fabric docker images"
    pull_Images
}

# pull fabric docker images 
pull_Images() {
        for IMAGES in ${IMAGES_LIST[*]}; do
            docker pull $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$STABLE_VERSION
            docker tag $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$STABLE_VERSION $ORG_NAME-$IMAGES:$ARCH-$RELEASE_VERSION
            docker tag $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$STABLE_VERSION $ORG_NAME-$IMAGES:latest
            docker rmi -f $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$STABLE_VERSION
        done
}

# pull fabric binaries
pull_Binary() {
# 
    export MARCH=$(echo "$(uname -s|tr '[:upper:]' '[:lower:]'|sed 's/mingw64_nt.*/windows/')-$(uname -m | sed 's/x86_64/amd64/g')" | awk '{print tolower($0)}')
    echo "------> MARCH:" $MARCH
    echo "-------> pull stable binaries for all platforms (x and z)"
    MVN_METADATA=$(echo "https://nexus.hyperledger.org/content/repositories/releases/org/hyperledger/fabric/hyperledger-fabric-stable/maven-metadata.xml")
    curl -L "$MVN_METADATA" > maven-metadata.xml
    RELEASE_TAG=$(cat maven-metadata.xml | grep release)
    COMMIT=$(echo $RELEASE_TAG | awk -F - '{ print $4 }' | cut -d "<" -f1)
    echo "--------> COMMIT:" $COMMIT
    curl https://nexus.hyperledger.org/content/repositories/releases/org/hyperledger/fabric/hyperledger-fabric-stable/$MARCH.$STABLE_VERSION-$COMMIT/hyperledger-fabric-stable-$MARCH.$STABLE_VERSION-$COMMIT.tar.gz | tar xz 
}

# pull fabric docker images from amd64 and s390x platforms
pull_Platform_All() {

    # pull stable images from nexus and tag to hyperledger
    echo "-------> pull docker images for all platforms (x, z)"
    for arch in amd64 s390x; do
        for IMAGES in ${IMAGES_LIST[*]}; do
            docker pull $NEXUS_URL/$ORG_NAME-$IMAGES:$arch-$STABLE_VERSION
            docker tag $NEXUS_URL/$ORG_NAME-$IMAGES:$arch-$STABLE_VERSION $ORG_NAME-$IMAGES:$arch-$RELEASE_VERSION
            docker rmi -f $NEXUS_URL/$ORG_NAME-$IMAGES:$arch-$STABLE_VERSION
        done
    done
}

push() {

# pull fabric images
    pull_Platform_All
# push docker images
    echo "------> push docker images"
    docker login -u $DOCKER_HUB_USERNAME -p $DOCKER_HUB_PASSWORD
    for MARCH in amd64 s390x; do
        for IMAGES in ${IMAGES_LIST[*]}; do
            docker push $ORG_NAME-$IMAGES:$MARCH-$RELEASE_VERSION
        done
    done
}

# pull thirdparty docker images from nexus
pull_Thirdparty() {   
    echo "------> pull thirdparty docker images from nexus"
    BASE_VERSION=$(curl --silent  https://raw.githubusercontent.com/hyperledger/fabric/master/Makefile 2>&1 | tee Makefile | grep "BASEIMAGE_RELEASE=" | cut -d "=" -f2)
    for IMAGES in ${THIRDPARTY_IMAGES_LIST[*]}; do
          docker pull $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$BASE_VERSION
          docker tag $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$BASE_VERSION $ORG_NAME-$IMAGES
          docker tag $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$BASE_VERSION $ORG_NAME-$IMAGES:$ARCH-$BASE_VERSION
          docker rmi -f $NEXUS_URL/$ORG_NAME-$IMAGES:$ARCH-$BASE_VERSION
    done
}

Parse_Arguments() {
    while [ $# -gt 0 ]; do
        case $1 in
            --cleanup)
                cleanup
                ;;
            --pull_All)
                pull_All
                ;;
            --pull_Thirdparty)
                pull_Thirdparty
                ;;
            --pull_Binary)
                pull_Binary
                ;;
            --pull_Images)
                pull_Images
                ;;
            --pull_Platform_All)
                pull_Platform_All
                ;;
	    # Ready to release?
            --push)
		push;
                ;;
        esac
        shift
    done
}
Parse_Arguments $@
