#!/bin/bash

set -eu

function setup_suite {
  export BATS_TEST_TIMEOUT=150

  cd "$BATS_TEST_DIRNAME"/..
  make
  make docker-build

}

function teardown_suite {
}