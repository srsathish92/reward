#!/bin/bash

function setup() {
  source "$(dirname "$(realpath "${BASH_SOURCE[0]}")")/post-start.sh"
}

function test_shopware_link_shared_files() {
  # Test with a valid SHARED_CONFIG_PATH
  local SHARED_CONFIG_PATH="./test-data/config"
  mkdir -p "${SHARED_CONFIG_PATH}"
  touch "${SHARED_CONFIG_PATH}/.env"

  local APP_PATH="./test-data/var/www/html"
  mkdir -p "${APP_PATH}"

  setup

  shopware_link_shared_files
  assert_exit_code 0 "$(test -L './test-data/var/www/html/.env')"

  rm -fr "./test-data"
}

function test_run_hooks() {
  setup

  local APP_PATH="./test-data/app"
  mkdir -p "${APP_PATH}/hooks/post-start.d"
  printf "#!/bin/bash\necho 'test-123'" >"${APP_PATH}/hooks/post-start.d/01-test.sh"
  assert_contains "test-123" "$(main)"
  rm -fr "./test-data"
}
