import 'dart:io';

import 'package:byr_client/platforms/app_platform.dart';

import 'release/package_release.dart';
import 'release/release_target.dart';

const _usage = '''
Usage: dart run tool/build_release.dart --platform <android|windows|macos|linux>
                                      [--build-name 1.2.3[-pre]]
                                      [--package-only]

Builds a Release and writes runnable software/packages to dist/<platform>/.
--package-only packages an existing build without invoking Flutter.
Desktop targets require their matching operating system. Android works on all
hosts with the Android SDK. Run flutter pub get before invoking this tool.
''';

Future<void> main(List<String> arguments) async {
  if (arguments.contains('--help') || arguments.contains('-h')) {
    stdout.write(_usage);
    return;
  }
  try {
    final options = _parse(arguments);
    final target = releaseTarget(options.platform);
    target.checkHost(Platform.operatingSystem);
    final project = Directory.current;
    if (!File('pubspec.yaml').existsSync() || !Directory('lib').existsSync()) {
      throw ArgumentError('Run this tool from the BYR Client project root');
    }
    if (!options.packageOnly) {
      final command = [
        'build',
        target.flutterTarget,
        '--release',
        '--no-pub',
        if (options.buildName != null) '--build-name=${options.buildName}',
      ];
      stdout.writeln('Building ${target.platform.name} Release...');
      final flutter = await Process.start(
        Platform.isWindows ? 'flutter.bat' : 'flutter',
        command,
        runInShell: Platform.isWindows,
        mode: ProcessStartMode.inheritStdio,
      );
      final code = await flutter.exitCode;
      if (code != 0) {
        exitCode = code;
        return;
      }
    }
    final output = await packageRelease(
      project: project,
      target: target,
      buildName: options.buildName,
    );
    stdout.writeln('Release: ${output.path}');
    stdout.writeln('Package: ${output.path}/${target.packageName}');
  } catch (error) {
    stderr.writeln('Release failed: $error');
    exitCode = 1;
  }
}

({AppPlatform platform, String? buildName, bool packageOnly}) _parse(
  List<String> arguments,
) {
  String? name;
  String? buildName;
  var packageOnly = false;
  for (var index = 0; index < arguments.length; index++) {
    final flag = arguments[index];
    if (flag == '--package-only') {
      packageOnly = true;
    } else if ((flag == '--platform' || flag == '--build-name') &&
        index + 1 < arguments.length) {
      final value = arguments[++index];
      if (flag == '--platform') {
        name = value;
      } else {
        buildName = value;
      }
    } else {
      throw ArgumentError('Unknown or incomplete argument: $flag\n$_usage');
    }
  }
  final platform = AppPlatform.values.where((value) => value.name == name);
  if (platform.isEmpty) throw ArgumentError(_usage);
  if (buildName != null &&
      !RegExp(r'^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$').hasMatch(buildName)) {
    throw ArgumentError('Invalid build name: $buildName');
  }
  if (packageOnly && buildName != null) {
    throw ArgumentError('--build-name requires a new build');
  }
  return (
    platform: platform.single,
    buildName: buildName,
    packageOnly: packageOnly,
  );
}
