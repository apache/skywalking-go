#!/bin/bash
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -ex
cp -rf /mnt/d/a/skywalking-go/skywalking-go/test/plugins/workspace /root/repo/skywalking-go/test/plugins/
cd {{.Context.WorkSpaceDir}}
export WINDOWS_HOST=`cat /etc/resolv.conf | grep nameserver | cut -d ' ' -f 2`
sed -i "s/service:8080/$WINDOWS_HOST:8080/g" ./config/excepted.yml
sed -i "s/HTTP_HOST=127\.0\.0\.1/HTTP_HOST=$WINDOWS_HOST/g" validator.sh

compose_version=$(docker-compose version --short)

if [[ $compose_version =~ ^(v)?1 ]]; then
    separator="_"
elif [[ $compose_version =~ ^(v)?2 ]]; then
    separator="-"
else
    echo "Unsupported Docker Compose version: $compose_version"
    exit 1
fi

project_name=$(echo "{{.Context.ScenarioName}}" |sed -e "s/\.//g" |awk '{print tolower($0)}')
validator_container_name="${project_name}${separator}validator${separator}1"

docker-compose -p "${project_name}" -f "docker-compose.yml" up -d --build
[[ $? -ne 0 ]] && exit 1
set -ex

sleep 5

validator_container_name="${project_name}${separator}validator${separator}1"

validator_container_id=`docker ps -aqf "name=${validator_container_name}"`

status=$(docker wait ${validator_container_id})

if [[ -z ${validator_container_id} ]]; then
    echo "docker startup failure!" >&2
    status=1
else
    [[ $status -ne 0 ]] && docker logs ${validator_container_id} >&2

    docker-compose -p ${project_name} -f "docker-compose.yml" kill
    docker-compose -p ${project_name} -f "docker-compose.yml" rm -f
fi

exit $status