import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:byr_client/platforms/adapters/shared_preferences_store.dart';
import 'package:byr_client/shared/storage/draft_store.dart';
import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/storage/session.dart';

import '../support/fakes.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'preferences adapter persists supported types and removes values',
    () async {
      SharedPreferences.setMockInitialValues({});
      final store = SharedPreferencesStore();
      await store.write('text', 'hello');
      await store.write('number', 2);
      await store.write('scale', 1.2);
      await store.write('toggle', true);
      expect(await store.readString('text'), 'hello');
      expect(await store.readInt('number'), 2);
      expect(await store.readDouble('scale'), 1.2);
      expect(await store.readBool('toggle'), true);
      await store.remove('text');
      expect(await store.readString('text'), isNull);
      await expectLater(
        store.write('unsupported', Object()),
        throwsArgumentError,
      );
    },
  );

  test(
    'session restores without native plugins and notifies theme observers',
    () async {
      final store = MemoryPreferences();
      final original = Session(store);
      var notifications = 0;
      original.themeMode.addListener(() => notifications++);
      await original.saveToken('saved-token');
      await original.saveUsername('alice');
      await original.saveThemeMode(2);
      await original.saveFontScale(1.2);
      await original.saveUseIPv6(true);
      expect(notifications, 1);

      final restored = Session(store);
      await restored.load();
      expect(restored.isAuthenticated, isTrue);
      expect(restored.username, 'alice');
      expect(restored.themeMode.value, 2);
      expect(restored.fontScale.value, 1.2);
      expect(restored.useIPv6, isTrue);
      await restored.clear();
      final signedOut = Session(store);
      await signedOut.load();
      expect(signedOut.isAuthenticated, isFalse);
      expect(signedOut.username, isNull);
      expect(signedOut.themeMode.value, 2);
    },
  );

  test(
    'history remains bounded and restores through another instance',
    () async {
      final store = MemoryPreferences();
      final history = HistoryStore(store);
      for (var index = 0; index < 105; index++) {
        await history.add(
          HistoryEntry(
            board: 'Joke',
            id: '$index',
            title: '$index',
            visitedAt: index,
          ),
        );
      }
      final entries = await HistoryStore(store).load();
      expect(entries, hasLength(100));
      expect(entries.first.id, '104');
      expect(entries.last.id, '5');
      await history.clear();
      expect(await history.load(), isEmpty);
    },
  );

  test(
    'draft snapshots round-trip, discard corruption and remove empty drafts',
    () async {
      final store = MemoryPreferences();
      final drafts = DraftStore(store);
      const key = 'composeDraft.Joke.new';
      await drafts.save(key, const ComposeDraft(title: '标题', content: '正文'));
      final restored = await DraftStore(store).load(key);
      expect(restored?.title, '标题');
      expect(restored?.content, '正文');
      await store.write(key, '{broken');
      expect(await drafts.load(key), isNull);
      await drafts.save(key, const ComposeDraft(title: '', content: ''));
      expect(await store.readString(key), isNull);
    },
  );
}
