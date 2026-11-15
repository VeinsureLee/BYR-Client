import 'package:file_picker/file_picker.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:byr_client/shared/services/device_services.dart';

/// Adapter over federated plugins. OS implementations remain in the plugins.
class FlutterDeviceServices implements DeviceServices {
  const FlutterDeviceServices();

  @override
  Future<void> copyText(String text) =>
      Clipboard.setData(ClipboardData(text: text));

  @override
  Future<bool> openLink(Uri uri) => launchUrl(uri);

  @override
  Future<List<SelectedAttachment>> pickAttachments() async {
    final files = await FilePicker.pickFiles();
    return [
      for (final file in files)
        if (file.path case final String path)
          SelectedAttachment(path: path, name: file.name),
    ];
  }
}
