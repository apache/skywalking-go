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

home="$(cd "$(dirname $0)"; pwd)"
scenario_name=
debug_mode="off"
cleanup="off"

scenarios_home="${home}/scenarios"
num_of_testcases=0

start_stamp=`date +%s`

replace() {
  if [ $# -lt 2 ]; then
    echo 1
    return
  fi

  opt=''
  cmd=$1
  file=$2
  if [ $# -eq 3 ]; then
    opt=$1
    cmd=$2
    file=$3
  fi

  if [ $(uname) = 'Darwin' ]; then
    sed -i '' $opt "$cmd" "$file"
  else
    sed -i $opt "$cmd" "$file"
  fi
}
export -f replace

print_help() {
    echo  "Usage: run.sh [OPTION] SCENARIO_NAME"
    echo -e "\t--clean, \t\t\t remove the related images and directories"
    echo -e "\t--debug, \t\t\t to save the log files and actualData.yaml"
}

remove_dir() {
    dir=$1
    if [[ "${os}" == "Darwin" ]]; then
        find ${dir} -type d -exec chmod -a "$(whoami) deny delete" {} \;
    fi
    rm -rf $dir
}

exitAndClean() {
    elapsed=$(( `date +%s` - $start_stamp ))
    printf "Scenarios: ${scenario_name}, Testcases: ${num_of_testcases}, Elapsed: %02d:%02d:%02d \n" \
        $(( ${elapsed}/3600 )) $(( ${elapsed}%3600/60 )) $(( ${elapsed}%60 ))
    exit $1
}

exitWithMessage() {
    echo -e "\033[31m[ERROR] $1\033[0m">&2
    exitAndClean 1
}

# parse command line
parse_commandline() {
  while test $# -gt 0; do
    _key="$1"
    case "$_key" in
      --debug)
        debug_mode="on"
        ;;
      --clean)
        cleanup="on"
        ;;
      -h|--help)
        print_help
        exit 0
        ;;
      *)
        scenario_name=$1
        ;;
    esac
    shift
  done
}
parse_commandline "$@"

do_cleanup() {
  images=$(docker images -qf "dangling=true")
  [[ -n "${images}" ]] && docker rmi -f ${images}

  docker network prune -f
  docker volume prune -f

  [[ -d ${home}/dist ]] && rm -rf ${home}/dist
  [[ -d ${home}/workspace ]] && rm -rf ${home}/workspace
  return
}

if [[ "$cleanup" == "on" ]]; then
    do_cleanup
    [[ -z "${scenario_name}" ]] && exit 0
fi

test -z "$scenario_name" && exitWithMessage "Missing value for the scenario argument"

scenario_home=${scenarios_home}/${scenario_name}

# reading versions from plugin configuration
configuration=${scenario_home}/plugin.yml
if [[ ! -f $configuration ]]; then
    exitWithMessage "cannot found 'plugin.yml' in directory ${scenario_name}"
fi

# support go, framework versions
framework_name=$(yq e '.framework' $configuration)
if [ -z "$framework_name" ]; then
  exitWithMessage "Missing framework name in configuration"
fi
support_version_count=$(yq e '.support-version | length' $configuration)
if [ "$support_version_count" -eq 0 ]; then
  exitWithMessage "Missing support-version list in configuration"
fi
index=0
while [ $index -lt $support_version_count ]; do
  go_version=$(yq e ".support-version[$index].go" $configuration)
  framework_count=$(yq e ".support-version[$index].framework | length" $configuration)

  if [ -z "$go_version" ] || ([[ "$framework_name" != "go" ]] && [ "$framework_count" -eq 0 ]); then
    exitWithMessage "Missing go or framework in list entry $index."
  fi

  index=$((index+1))
done

workspace="${home}/workspace/${scenario_name}"
[[ -d ${workspace} ]] && rm -rf $workspace

plugin_runner_helper="${home}/dist/runner-helper"
if [[ ! -f $plugin_runner_helper ]]; then
    exitWithMessage "cannot found 'runner-helper' in directory ${home}/dist"
fi
go_agent="${home}/dist/skywalking-go-agent"
if [[ ! -f $go_agent ]]; then
    exitWithMessage "cannot found 'go-agent' in directory ${home}/dist"
fi

yq e '.support-version[].go' $configuration | while read -r go_version; do
frameworks=$(yq e ".support-version[] | select(.go == \"$go_version\") | .framework[]" $configuration)
if [[ "$framework_name" == "go" ]]; then
  frameworks=("native")
fi
for framework_version in $frameworks; do
  echo "ready to run test case: ${scenario_name} with go version: ${go_version} and framework version: ${framework_version}"
  case_name="go${go_version}-${framework_version}"

  # copy test case to workspace
  case_home="${workspace}/${case_name}"
  case_logs="${case_home}/logs"
  mkdir -p ${case_home}
  mkdir -p ${case_logs}
  cp -rf ${scenario_home}/* ${case_home}
  cd ${case_home}

  # append the module name(for go1.20, module name cannot be same in same workspace)
  mod_case_name=$(echo "${case_name}" | sed 's/\//_/g; s/\./_/g; s/-/_/g')
  mod_name=$(head -n 1 go.mod | cut -d " " -f 2)
  # replace go version
  replace "s/^go [0-9]*\.[0-9]*/go ${go_version}/" go.mod
  replace "s/^module /module ${mod_case_name}\//" go.mod
  find . -name "*.go" -type f -exec bash -c "replace \"s|${mod_name}|${mod_case_name}/${mod_name}|g\" \"{}\"" \;
  # ajust the plugin replace path
  replace -E '/^replace/ s#(\.\./)#\1../#' go.mod

  # replace framework version
  if [[ "$framework_version" != "native" ]]; then
    replace "s|$framework_name v.*|$framework_name $framework_version|" go.mod
  fi

  # run runner helper for prepare running docker-compose
  ${plugin_runner_helper} \
    -workspace ${case_home} \
    -project ${home}/../../ \
    -go-version ${go_version} \
    -scenario ${scenario_name} \
    -case ${case_name} \
    -debug ${debug_mode} \
    -go-agent ${go_agent} > ${case_logs}/runner-helper.log

  echo "staring the testcase ${scenario_name}, ${case_name}"

  bash ${case_home}/scenarios.sh > ${case_logs}/scenarios.log
  status=$?
  if [[ $status == 0 ]]; then
      [[ $debug_mode == "off" ]] && remove_dir ${case_home}
  else
      exitWithMessage "Testcase ${case_name} failed!"
  fi
  num_of_testcases=$(($num_of_testcases+1))

done
done

exitAndClean 0