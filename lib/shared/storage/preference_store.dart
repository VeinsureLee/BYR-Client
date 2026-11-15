/// Application storage contract. Native persistence belongs to its Adapter.
abstract interface class PreferenceStore {
  Future<String?> readString(String key);
  Future<int?> readInt(String key);
  Future<double?> readDouble(String key);
  Future<bool?> readBool(String key);
  Future<void> write(String key, Object value);
  Future<void> remove(String key);
}
