import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:byr_client/platforms/app_platform.dart';

import '../../tool/release/package_release.dart';
import '../../tool/release/release_target.dart';

void main() {
  late Directory project;

  setUp(() async {
    project = await Directory.systemTemp.createTemp('byr-release-test-');
    await File('${project.path}/pubspec.yaml')
        .writeAsString('version: 4.0.0+1\n');
  });

  tearDown(() async => project.delete(recursive: true));

  test('desktop releases require their own host, Android is cross-host', () {
    releaseTarget(AppPlatform.android).checkHost('windows');
    releaseTarget(AppPlatform.android).checkHost('macos');
    releaseTarget(AppPlatform.android).checkHost('linux');
    expect(
      () => releaseTarget(AppPlatform.windows).checkHost('linux'),
      throwsUnsupportedError,
    );
    expect(
      () => releaseTarget(AppPlatform.macos).checkHost('windows'),
      throwsUnsupportedError,
    );
    expect(
      () => releaseTarget(AppPlatform.linux).checkHost('macos'),
      throwsUnsupportedError,
    );
  });

  test('a missing build preserves the previous downloadable release', () async {
    final previous = File('${project.path}/dist/android/previous.apk');
    await previous.parent.create(recursive: true);
    await previous.writeAsString('previous');
    await expectLater(
      packageRelease(
        project: project,
        target: releaseTarget(AppPlatform.android),
      ),
      throwsStateError,
    );
    expect(await previous.readAsString(), 'previous');
    expect(await Directory('${project.path}/dist').list().length, 1);
  });

  test(
    'Android packaging isolates platforms and publishes a manifest',
    () async {
      final apk = File(
        '${project.path}/build/app/outputs/flutter-apk/app-release.apk',
      );
      await apk.parent.create(recursive: true);
      await apk.writeAsBytes([0x50, 0x4b, 1, 2]);
      final untouched = File('${project.path}/dist/windows/keep.txt');
      await untouched.parent.create(recursive: true);
      await untouched.writeAsString('windows');
      final output = await packageRelease(
        project: project,
        target: releaseTarget(AppPlatform.android),
      );
      expect(
        await File('${output.path}/BYR-Client-android.apk').readAsBytes(),
        await apk.readAsBytes(),
      );
      final manifest = jsonDecode(
        await File('${output.path}/manifest.json').readAsString(),
      ) as Map;
      expect(manifest['platform'], 'android');
      expect(manifest['version'], '4.0.0');
      expect(manifest['buildNumber'], '1');
      expect(manifest['signing'], 'debug');
      expect(await untouched.readAsString(), 'windows');
    },
  );

  test(
    'Windows packages the complete runtime and keeps a runnable folder',
    () async {
      final bundle = Directory(
        '${project.path}/build/windows/x64/runner/Release',
      );
      for (final name in [
        'byr_client.exe',
        'flutter_windows.dll',
        'data/icudtl.dat',
        'data/app.so',
        'data/flutter_assets/asset.txt',
      ]) {
        final file = File('${bundle.path}/$name');
        await file.parent.create(recursive: true);
        await file.writeAsString(name);
      }
      final output = await packageRelease(
        project: project,
        target: releaseTarget(AppPlatform.windows),
      );
      expect(
        File('${output.path}/BYR-Client-windows-x64.zip').lengthSync(),
        greaterThan(0),
      );
      expect(
        File('${output.path}/app/data/flutter_assets/asset.txt')
            .readAsStringSync(),
        'data/flutter_assets/asset.txt',
      );
      expect(
        File('${output.path}/app/flutter_windows.dll').existsSync(),
        isTrue,
      );
    },
    skip: !Platform.isWindows,
  );
}
