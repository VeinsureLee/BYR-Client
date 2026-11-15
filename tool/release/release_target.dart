import 'dart:ffi' show Abi;
import 'dart:io';

import 'package:byr_client/platforms/app_platform.dart';

typedef PackageAssembler = Future<void> Function(
  Directory project,
  Directory output,
);

/// A release Strategy is build metadata plus a packaging function, not a
/// subclass hierarchy. All four targets have real packaging differences.
class ReleaseTarget {
  const ReleaseTarget({
    required this.platform,
    required this.flutterTarget,
    required this.packageName,
    required this.assemble,
    this.requiredHost,
  });

  final AppPlatform platform;
  final String flutterTarget;
  final String packageName;
  final String? requiredHost;
  final PackageAssembler assemble;

  void checkHost(String host) {
    if (requiredHost != null && host != requiredHost) {
      throw UnsupportedError('${platform.name} requires a $requiredHost host');
    }
  }
}

ReleaseTarget releaseTarget(AppPlatform platform) => switch (platform) {
  AppPlatform.android => const ReleaseTarget(
    platform: AppPlatform.android,
    flutterTarget: 'apk',
    packageName: 'BYR-Client-android.apk',
    assemble: _android,
  ),
  AppPlatform.windows => const ReleaseTarget(
    platform: AppPlatform.windows,
    flutterTarget: 'windows',
    requiredHost: 'windows',
    packageName: 'BYR-Client-windows-x64.zip',
    assemble: _windows,
  ),
  AppPlatform.macos => const ReleaseTarget(
    platform: AppPlatform.macos,
    flutterTarget: 'macos',
    requiredHost: 'macos',
    packageName: 'BYR-Client-macos.zip',
    assemble: _macos,
  ),
  AppPlatform.linux => const ReleaseTarget(
    platform: AppPlatform.linux,
    flutterTarget: 'linux',
    requiredHost: 'linux',
    packageName: 'BYR-Client-linux.tar.gz',
    assemble: _linux,
  ),
};

Future<void> _requireFile(String path) async {
  final file = File(path);
  if (!await file.exists() || await file.length() == 0) {
    throw StateError('Missing or empty build artifact: $path');
  }
}

Future<void> _requireBundle(Directory bundle, List<String> files) async {
  for (final file in files) {
    await _requireFile('${bundle.path}/$file');
  }
  if (!await Directory('${bundle.path}/data/flutter_assets').exists()) {
    throw StateError('Missing Flutter assets: ${bundle.path}');
  }
}

Future<void> _run(String executable, List<String> arguments) async {
  final result = await Process.run(executable, arguments);
  if (result.exitCode != 0) {
    throw ProcessException(
      executable,
      arguments,
      '${result.stdout}\n${result.stderr}',
      result.exitCode,
    );
  }
}

Future<void> _copyWindowsBundle(Directory source, Directory output) async {
  await output.create(recursive: true);
  await for (final entity in source.list()) {
    final name = entity.uri.pathSegments.where((part) => part.isNotEmpty).last;
    final destination = '${output.path}/$name';
    if (entity is Directory) {
      await _copyWindowsBundle(entity, Directory(destination));
    } else if (entity is File) {
      await entity.copy(destination);
    } else {
      throw StateError('Unexpected link in Windows bundle: ${entity.path}');
    }
  }
}

Future<void> _android(Directory project, Directory output) async {
  final source =
      '${project.path}/build/app/outputs/flutter-apk/app-release.apk';
  await _requireFile(source);
  await File(source).copy('${output.path}/BYR-Client-android.apk');
}

Future<void> _windows(Directory project, Directory output) async {
  final source = Directory('${project.path}/build/windows/x64/runner/Release');
  await _requireBundle(source, [
    'byr_client.exe',
    'flutter_windows.dll',
    'data/icudtl.dat',
    'data/app.so',
  ]);
  final app = Directory('${output.path}/app');
  await _copyWindowsBundle(source, app);
  String literal(String value) => "'${value.replaceAll("'", "''")}'";
  // LiteralPath avoids interpreting spaces, brackets, or apostrophes in paths.
  await _run('powershell.exe', [
    '-NoLogo',
    '-NoProfile',
    '-NonInteractive',
    '-Command',
    "\$ErrorActionPreference = 'Stop'; "
        '\$items = Get-ChildItem -LiteralPath ${literal(app.path)} -Force; '
        'Compress-Archive -LiteralPath \$items.FullName '
        '-DestinationPath ${literal('${output.path}/BYR-Client-windows-x64.zip')}',
  ]);
}

Future<void> _macos(Directory project, Directory output) async {
  final products = Directory(
    '${project.path}/build/macos/Build/Products/Release',
  );
  final apps = await products
      .list()
      .where((entity) => entity is Directory && entity.path.endsWith('.app'))
      .cast<Directory>()
      .toList();
  if (apps.length != 1) throw StateError('Expected one macOS .app bundle');
  final source = apps.single;
  await _requireFile('${source.path}/Contents/Info.plist');
  await _requireFile('${source.path}/Contents/MacOS/byr_client');
  final name = source.uri.pathSegments.where((part) => part.isNotEmpty).last;
  final app = '${output.path}/app/$name';
  // ditto preserves framework links, resource forks and executable permissions.
  await _run('ditto', [source.path, app]);
  await _run('ditto', [
    '-c',
    '-k',
    '--sequesterRsrc',
    '--keepParent',
    app,
    '${output.path}/BYR-Client-macos.zip',
  ]);
}

Future<void> _linux(Directory project, Directory output) async {
  final arch = Abi.current() == Abi.linuxArm64 ? 'arm64' : 'x64';
  final source = Directory('${project.path}/build/linux/$arch/release/bundle');
  await _requireBundle(source, [
    'byr_client',
    'lib/libflutter_linux_gtk.so',
    'data/icudtl.dat',
    'data/app.so',
  ]);
  final app = Directory('${output.path}/app');
  await app.create(recursive: true);
  // Native cp/tar preserve executable modes and relative library links.
  await _run('cp', ['-a', '${source.path}/.', app.path]);
  await _run('tar', [
    '-czf',
    '${output.path}/BYR-Client-linux.tar.gz',
    '-C',
    app.path,
    '.',
  ]);
}
