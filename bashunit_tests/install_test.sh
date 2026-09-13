#!/usr/bin/env bash

# Exercise the curl installer as a black box. Each test uses an isolated HOME
# and fake external commands so it cannot reach the network or write to system
# install locations.

installer_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/bin/install.sh"

# write_executable writes standard input
# into the file specified by the first argument,
# then makes the file executable
write_executable() {
  local path="$1"

  cat > "$path"
  chmod +x "$path"
}

# set_up isolates installer side effects and replaces host-dependent executables
# with faked test implementations.
function set_up() {
  test_dir="$(bashunit::temp_dir installer)"
  test_home="$test_dir/home"
  fake_bin="$test_dir/bin"
  download_url_log="$test_home/download-url"
  render_invocations_log="$test_home/render-invocations"
  fake_render="$test_dir/render"
  mkdir -p "$test_home" "$fake_bin"

  # Force the non-root install path so the installer writes under the fake HOME.
  write_executable "$fake_bin/id" << 'EOF'
#!/bin/sh
printf "1000\n"
EOF

  # Select one supported platform regardless of the host running the test.
  write_executable "$fake_bin/uname" << 'EOF'
#!/bin/sh
case "$1" in
  -s) printf "Linux\n" ;;
  -m) printf "x86_64\n" ;;
esac
EOF

  # Return a fixed release and capture its download URL without using the network.
  write_executable "$fake_bin/curl" << 'EOF'
#!/bin/sh
case "$2" in
  https://api.github.com/repos/render-oss/cli/releases/latest)
    printf '{"tag_name": "v9.8.7"}\n'
    ;;
  https://github.com/render-oss/cli/releases/download/*)
    printf "%s\n" "$2" >"$HOME/download-url"
    : >"$4"
    ;;
  *)
    printf "Unexpected curl URL: %s\n" "$2" >&2
    exit 1
    ;;
esac
EOF

  # Simulate extracting the expected binary without requiring a real ZIP archive.
  write_executable "$fake_bin/unzip" << 'EOF'
#!/bin/sh
cp "$INSTALL_TEST_FAKE_RENDER" "$4/cli_v9.8.7"
EOF
  write_executable "$fake_render" << 'EOF'
#!/bin/sh
printf "%s\n" "$*" >>"$HOME/render-invocations"
if [ "$#" -eq 2 ] && [ "$1" = "analytics" ] && [ "$2" = "notice" ]; then
  exit "$INSTALL_TEST_ANALYTICS_NOTICE_EXIT_CODE"
fi
exit 0
EOF
}

# run_installer executes the installer in a separate sh process to approximate
# an end-user "curl install" of render CLI while keeping it isolated and
# deterministic. Its optional argument sets the fake analytics notice exit code.
function run_installer() {
  local analytics_notice_exit_code="${1:-0}"

  HOME="$test_home" \
    PATH="$fake_bin:/usr/bin:/bin" \
    INSTALL_TEST_ANALYTICS_NOTICE_EXIT_CODE="$analytics_notice_exit_code" \
    INSTALL_TEST_FAKE_RENDER="$fake_render" \
    sh "$installer_path"
}

function test_installs_the_latest_release() {
  local installed_render="$test_home/.local/bin/render"
  local stdout="$test_dir/stdout"
  local stderr="$test_dir/stderr"

  run_installer > "$stdout" 2> "$stderr"

  assert_exit_code 0
  assert_empty "$(< "$stderr")"
  assert_is_file_executable "$installed_render"
  assert_same \
    "https://github.com/render-oss/cli/releases/download/v9.8.7/cli_9.8.7_linux_amd64.zip" \
    "$(< "$download_url_log")"
  assert_contains "Installing Render CLI version v9.8.7..." "$(< "$stdout")"
  assert_contains "Successfully installed Render CLI to $installed_render" "$(< "$stdout")"
  assert_contains "export PATH=\$PATH:$test_home/.local/bin" "$(< "$stdout")"
  assert_not_contains $'\033[' "$(< "$stdout")"
}

function test_install_fails_when_release_download_fails() {
  local installed_render="$test_home/.local/bin/render"
  local stdout="$test_dir/stdout"
  local stderr="$test_dir/stderr"

  write_executable "$fake_bin/curl" << 'EOF'
#!/bin/sh
case "$2" in
  https://api.github.com/repos/render-oss/cli/releases/latest)
    printf '{"tag_name": "v9.8.7"}\n'
    ;;
  https://github.com/render-oss/cli/releases/download/*)
    printf "Download failed\n" >&2
    exit 22
    ;;
esac
EOF

  run_installer > "$stdout" 2> "$stderr"

  assert_exit_code 22
  assert_contains "Download failed" "$(< "$stderr")"
  assert_file_not_exists "$installed_render"
}

function test_invokes_the_analytics_notice_after_installing() {
  run_installer > /dev/null

  assert_same "analytics notice" "$(< "$render_invocations_log")"
}

function test_notice_failure_does_not_fail_the_install() {
  run_installer 1 > /dev/null

  assert_successful_code
  assert_same "analytics notice" "$(< "$render_invocations_log")"
}

function test_sudo_install_defers_the_analytics_notice() {
  # Keep the installation under the fake HOME, then report root when the
  # installer checks whether it should defer the notice for the sudo user.
  write_executable "$fake_bin/id" << 'EOF'
#!/bin/sh
if [ -e "$HOME/id-called" ]; then
  printf "0\n"
else
  : >"$HOME/id-called"
  printf "1000\n"
fi
EOF

  SUDO_USER=alice run_installer > /dev/null

  assert_successful_code
  assert_file_not_exists "$render_invocations_log"
}
