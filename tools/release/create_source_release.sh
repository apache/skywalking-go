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

make version-check

cd ..

tar -zcvf ${PRODUCT_NAME}-src.tgz \
	--exclude bin \
	--exclude .git \
	--exclude .cache \
	--exclude .DS_Store \
	--exclude .github \
	--exclude .gitignore \
	--exclude .gitmodules \
	--exclude .gitattributes \
	--exclude dist \
	--exclude ${PRODUCT_NAME}-src.tgz \
	${PRODUCT_NAME}

gpg --batch --yes --armor --detach-sig ${PRODUCT_NAME}-src.tgz
shasum -a 512 ${PRODUCT_NAME}-src.tgz > ${PRODUCT_NAME}-src.tgz.sha512

rm -rf ${PRODUCT_NAME}
