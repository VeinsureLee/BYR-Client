/// Defensive JSON helpers.
///
/// The original server mixes strings, numbers and booleans for the same field
/// across endpoints, so parsing is intentionally lenient.
library;

String? asString(dynamic value) {
  if (value == null) return null;
  if (value is String) return value;
  return value.toString();
}

int? asInt(dynamic value) {
  if (value == null) return null;
  if (value is int) return value;
  if (value is num) return value.toInt();
  if (value is bool) return value ? 1 : 0;
  return int.tryParse(value.toString());
}

double? asDouble(dynamic value) {
  if (value == null) return null;
  if (value is num) return value.toDouble();
  return double.tryParse(value.toString());
}

bool? asBool(dynamic value) {
  if (value == null) return null;
  if (value is bool) return value;
  if (value is num) return value != 0;
  final text = value.toString().toLowerCase();
  if (text == 'true' || text == '1') return true;
  if (text == 'false' || text == '0' || text.isEmpty) return false;
  return null;
}

List<dynamic> asList(dynamic value) {
  if (value is List) return value;
  // Some endpoints collapse a single-element array into an object.
  if (value is Map) return [value];
  return const [];
}

Map<String, dynamic> asMap(dynamic value) {
  if (value is Map) return value.cast<String, dynamic>();
  if (value is String && value.isNotEmpty) {
    return const {};
  }
  return const {};
}

List<T> parseList<T>(dynamic value, T Function(Map<String, dynamic>) fromJson) {
  return asList(value)
      .whereType<Map>()
      .map((item) => fromJson(item.cast<String, dynamic>()))
      .toList(growable: false);
}
