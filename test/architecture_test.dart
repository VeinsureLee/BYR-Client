import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('shared logic and feature screens never import concrete native adapters', () {
    final forbidden = RegExp(
      r"import 'package:(?:file_picker|shared_preferences|url_launcher)/|"
      r"import 'package:byr_client/platforms/|\bPlatform\.is|\bSystemChrome\.",
    );
    for (final root in ['lib/shared', 'lib/features']) {
      for (final file
          in Directory(root)
              .listSync(recursive: true)
              .whereType<File>()
              .where((file) => file.path.endsWith('.dart'))) {
        expect(
          forbidden.hasMatch(file.readAsStringSync()),
          isFalse,
          reason: file.path,
        );
      }
    }
  });
}
