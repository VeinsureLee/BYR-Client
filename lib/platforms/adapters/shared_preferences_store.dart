import 'package:shared_preferences/shared_preferences.dart';

import 'package:byr_client/shared/storage/preference_store.dart';

/// Adapter: translates the plugin's typed setters to the shared storage port.
/// The plugin selects the Android/Windows/macOS/Linux implementation itself.
class SharedPreferencesStore implements PreferenceStore {
  SharedPreferencesStore() : _preferences = SharedPreferences.getInstance();

  final Future<SharedPreferences> _preferences;

  @override
  Future<String?> readString(String key) async =>
      (await _preferences).getString(key);

  @override
  Future<int?> readInt(String key) async => (await _preferences).getInt(key);

  @override
  Future<double?> readDouble(String key) async =>
      (await _preferences).getDouble(key);

  @override
  Future<bool?> readBool(String key) async => (await _preferences).getBool(key);

  @override
  Future<void> write(String key, Object value) async {
    final preferences = await _preferences;
    final saved = switch (value) {
      String() => await preferences.setString(key, value),
      int() => await preferences.setInt(key, value),
      double() => await preferences.setDouble(key, value),
      bool() => await preferences.setBool(key, value),
      _ => throw ArgumentError.value(value, 'value', 'Unsupported preference'),
    };
    if (!saved) throw StateError('Could not persist preference: $key');
  }

  @override
  Future<void> remove(String key) async {
    if (!await (await _preferences).remove(key)) {
      throw StateError('Could not remove preference: $key');
    }
  }
}
