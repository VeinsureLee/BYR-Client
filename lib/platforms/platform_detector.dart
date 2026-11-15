import 'dart:io' show Platform;

import 'package:byr_client/platforms/app_platform.dart';

AppPlatform detectPlatform() {
  if (Platform.isAndroid) return AppPlatform.android;
  if (Platform.isWindows) return AppPlatform.windows;
  if (Platform.isMacOS) return AppPlatform.macos;
  if (Platform.isLinux) return AppPlatform.linux;
  throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
}
