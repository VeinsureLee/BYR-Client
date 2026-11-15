import 'dart:convert';
import 'dart:io';

import 'package:yaml/yaml.dart';

import 'release_target.dart';

/// Publishes only complete artifacts. A failed assembler leaves the previous
/// platform release intact; platform outputs never share a directory.
Future<Directory> packageRelease({
  required Directory project,
  required ReleaseTarget target,
  String? buildName,
}) async {
  target.checkHost(Platform.operatingSystem);
  final root = await project.resolveSymbolicLinks();
  final outputRoot = Directory('$root/dist');
  await outputRoot.create(recursive: true);
  await _checkInside(project, outputRoot);
  final staging = await outputRoot.createTemp('.${target.platform.name}-');
  final destination = Directory('${outputRoot.path}/${target.platform.name}');
  try {
    await target.assemble(Directory(root), staging);
    final package = File('${staging.path}/${target.packageName}');
    if (!await package.exists() || await package.length() == 0) {
      throw StateError('Assembler did not produce ${target.packageName}');
    }
    final pubspec =
        loadYaml(await File('$root/pubspec.yaml').readAsString()) as Map;
    final version = pubspec['version'] as String;
    final parts = version.split('+');
    final metadata = {
      'platform': target.platform.name,
      'version': buildName ?? parts.first,
      'buildNumber': parts.length > 1 ? parts[1] : '1',
      'mode': 'release',
      'package': target.packageName,
      'runnable': target.platform.isDesktop ? 'app/' : target.packageName,
      'packagedAt': DateTime.now().toUtc().toIso8601String(),
      if (target.flutterTarget == 'apk') 'signing': 'debug',
    };
    await File('${staging.path}/manifest.json').writeAsString(
      '${const JsonEncoder.withIndent('  ').convert(metadata)}\n',
    );
    if (await destination.exists()) {
      await _checkInside(outputRoot, destination);
      await destination.delete(recursive: true);
    }
    return await staging.rename(destination.path);
  } catch (_) {
    if (await staging.exists()) {
      await _checkInside(outputRoot, staging);
      await staging.delete(recursive: true);
    }
    rethrow;
  }
}

Future<void> _checkInside(Directory parent, Directory child) async {
  final root = await parent.resolveSymbolicLinks();
  final path = await child.resolveSymbolicLinks();
  final prefix = '$root${Platform.pathSeparator}';
  final inside = Platform.isWindows
      ? path.toLowerCase().startsWith(prefix.toLowerCase())
      : path.startsWith(prefix);
  if (!inside) throw StateError('Output must remain inside $root: $path');
}
