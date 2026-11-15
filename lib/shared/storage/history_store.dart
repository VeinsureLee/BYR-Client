import 'dart:convert';

import 'package:byr_client/shared/storage/preference_store.dart';

class HistoryEntry {
  const HistoryEntry({
    required this.board,
    required this.id,
    required this.title,
    required this.visitedAt,
  });

  final String board;
  final String id;
  final String title;
  final int visitedAt;

  factory HistoryEntry.fromJson(Map<String, dynamic> json) => HistoryEntry(
    board: json['board'] as String? ?? '',
    id: json['id'] as String? ?? '',
    title: json['title'] as String? ?? '',
    visitedAt: json['visitedAt'] as int? ?? 0,
  );

  Map<String, dynamic> toJson() => {
    'board': board,
    'id': id,
    'title': title,
    'visitedAt': visitedAt,
  };
}

/// Local browsing history of opened threads (most recent first).
class HistoryStore {
  HistoryStore(this.preferences);

  final PreferenceStore preferences;
  static const String _key = 'browseHistory';
  static const int _maxEntries = 100;

  Future<List<HistoryEntry>> load() async {
    final raw = await preferences.readString(_key);
    if (raw == null || raw.isEmpty) return const [];
    try {
      final list = jsonDecode(raw);
      if (list is! List) return const [];
      return list
          .whereType<Map>()
          .map((e) => HistoryEntry.fromJson(e.cast<String, dynamic>()))
          .where((e) => e.board.isNotEmpty && e.id.isNotEmpty)
          .toList();
    } catch (_) {
      return const [];
    }
  }

  Future<void> add(HistoryEntry entry) async {
    final entries = (await load()).toList();
    entries.removeWhere((e) => e.board == entry.board && e.id == entry.id);
    entries.insert(0, entry);
    if (entries.length > _maxEntries) {
      entries.removeRange(_maxEntries, entries.length);
    }
    await preferences.write(
      _key,
      jsonEncode(entries.map((e) => e.toJson()).toList()),
    );
  }

  Future<void> clear() async {
    await preferences.remove(_key);
  }
}
