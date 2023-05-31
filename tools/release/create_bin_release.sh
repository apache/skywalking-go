#!/usr/bin/env bash

#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# This script relies on few environment variables to determine source code package
# behavior, those variables are:
#   VERSION -- The version of this source package.
# For example: VERSION=0.1.0


VERSION=${VERSION}
TAG_NAME=v${VERSION}

PRODUCT_NAME="apache-skywalking-go"

echo "Release version "${VERSION}
echo "Source tag "${TAG_NAME}

if [ "$VERSION" == "" ]; then
  echo "VERSION environment variable not found, Please setting the VERSION."
  echo "For example: export VERSION=0.1.0"
  exit 1
fi

echo "Creating source package"

PRODUCT_NAME=${PRODUCT_NAME}-${VERSION}

rm -rf ${PRODUCT_NAME}
mkdir ${PRODUCT_NAME}

git clone https://github.com/apache/skywalking-go.git ./${PRODUCT_NAME}
cd ${PRODUCT_NAME}

TAG_EXIST=`git tag -l ${TAG_NAME} | wc -l`

if [ ${TAG_EXIST} -ne 1 ]; then
    echo "Could not find the tag named" ${TAG_NAME}
    exit 1
fi

git checkout ${TAG_NAME}

make build

cd ..

RELEASE_BIN=${PRODUCT_NAME}-bin

mkdir ${RELEASE_BIN}
cp -R ${PRODUCT_NAME}/bin ${RELEASE_BIN}
cp -R ${PRODUCT_NAME}/dist/* ${RELEASE_BIN}
cp -R ${PRODUCT_NAME}/CHANGES.md ${RELEASE_BIN}
cp -R ${PRODUCT_NAME}/README.md ${RELEASE_BIN}

tar -zcvf ${RELEASE_BIN}.tgz ${RELEASE_BIN} && rm -rf ${RELEASE_BIN}

gpg --batch --yes --armor --detach-sig ${RELEASE_BIN}.tgz
shasum -a 512 ${RELEASE_BIN}.tgz > ${RELEASE_BIN}.tgz.sha512

rm -rf ${PRODUCT_NAME}
