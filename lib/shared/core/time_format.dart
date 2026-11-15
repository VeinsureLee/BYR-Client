/// Formats the time values returned by the nForum `/open` API.
///
/// The server uses Unix timestamps (seconds) for article and mail times, but a
/// few endpoints already return a formatted string. This helper accepts both
/// so list screens never show a raw `1794497375` value.
String formatTimestamp(String? raw, {bool alwaysShowYear = false}) {
  if (raw == null) return '';
  final text = raw.trim();
  if (text.isEmpty) return '';

  var value = int.tryParse(text);
  if (value == null) return text;
  if (value <= 0) return '';

  // Some responses use milliseconds. Values around 1e12 are milliseconds;
  // current Unix seconds are around 1.7e9.
  if (value > 100000000000) value ~/= 1000;

  final time = DateTime.fromMillisecondsSinceEpoch(value * 1000).toLocal();
  final now = DateTime.now();
  String two(int part) => part.toString().padLeft(2, '0');

  if (alwaysShowYear || time.year != now.year) {
    return '${time.year}-${two(time.month)}-${two(time.day)} '
        '${two(time.hour)}:${two(time.minute)}';
  }
  if (time.year == now.year && time.month == now.month && time.day == now.day) {
    return '${two(time.hour)}:${two(time.minute)}';
  }
  return '${two(time.month)}-${two(time.day)} '
      '${two(time.hour)}:${two(time.minute)}';
}

/// Converts a Unix-seconds value used by local models (chat/history) into the
/// same compact representation as [formatTimestamp].
String formatUnixSeconds(int? seconds, {bool alwaysShowYear = false}) {
  if (seconds == null || seconds <= 0) return '';
  return formatTimestamp('$seconds', alwaysShowYear: alwaysShowYear);
}
