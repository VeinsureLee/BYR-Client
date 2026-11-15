#!/usr/bin/env bash
# Build with Flutter on PATH, or an optional FLUTTER_BIN directory.
set -euo pipefail

if [[ -n "${FLUTTER_BIN:-}" ]]; then
  export PATH="$FLUTTER_BIN:$PATH"
fi
if [[ -n "${ANDROID_HOME:-}" ]]; then
  export ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-$ANDROID_HOME}"
fi

cd "$(dirname "$0")/../.."
flutter pub get --enforce-lockfile
flutter analyze --no-fatal-infos
flutter test
dart run tool/build_release.dart --platform android
