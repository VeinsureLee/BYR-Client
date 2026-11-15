class SelectedAttachment {
  const SelectedAttachment({required this.path, required this.name});

  final String path;
  final String name;
}

/// Native actions used by shared screens, independent of plugin return types.
abstract interface class DeviceServices {
  Future<void> copyText(String text);
  Future<bool> openLink(Uri uri);
  Future<List<SelectedAttachment>> pickAttachments();
}
