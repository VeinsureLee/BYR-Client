import 'dart:convert';

import 'package:byr_client/shared/storage/preference_store.dart';

class ComposeDraft {
  const ComposeDraft({required this.title, required this.content});

  final String title;
  final String content;

  bool get isEmpty => title.isEmpty && content.isEmpty;
}

/// Shared draft serialization; the editor only supplies an immutable snapshot.
class DraftStore {
  const DraftStore(this.preferences);

  final PreferenceStore preferences;

  Future<ComposeDraft?> load(String key) async {
    final raw = await preferences.readString(key);
    if (raw == null || raw.isEmpty) return null;
    try {
      final map = jsonDecode(raw) as Map<String, dynamic>;
      return ComposeDraft(
        title: map['title'] as String? ?? '',
        content: map['content'] as String? ?? '',
      );
    } catch (_) {
      return null;
    }
  }

  Future<void> save(String key, ComposeDraft draft) async {
    if (draft.isEmpty) {
      await remove(key);
      return;
    }
    await preferences.write(
      key,
      jsonEncode({'title': draft.title, 'content': draft.content}),
    );
  }

  Future<void> remove(String key) => preferences.remove(key);
}
