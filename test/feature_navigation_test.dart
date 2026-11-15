import 'dart:convert';
import 'dart:typed_data';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/data/byr_api.dart';
import 'package:byr_client/app/bootstrap.dart';
import 'package:byr_client/features/top_ten/top_ten_page.dart';
import 'package:byr_client/features/board/board_screen.dart';
import 'package:byr_client/platforms/mobile/mobile_home.dart';
import 'package:byr_client/features/mail/chat_screen.dart';
import 'package:byr_client/features/thread/thread_screen.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('board page has one favorite toggle, no compose FAB and time', (
    tester,
  ) async {
    final seconds = DateTime(2026, 1, 2, 3, 4).millisecondsSinceEpoch ~/ 1000;
    final adapter = _FakeApiAdapter({
      '/open/board/Joke.json': {
        'name': 'Joke',
        'description': '笑话',
        'is_favorite': true,
        'pagination': {
          'page_all_count': 1,
          'page_current_count': 1,
          'item_page_count': 50,
          'item_all_count': 1,
        },
        'article': [
          {
            'id': '1',
            'title': 'hello',
            'post_time': seconds,
            'reply_count': 2,
            'user': {'id': 'alice', 'user_name': 'Alice'},
          },
        ],
      },
      '/open/favorite/delete/0.json': {'board': []},
    });
    final api = _api(adapter);

    await tester.pumpWidget(_wrap(api, const BoardScreen(boardName: 'Joke')));
    await tester.pumpAndSettle();

    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.byIcon(Icons.star), findsOneWidget);
    expect(find.textContaining(formatTimestamp('$seconds')), findsOneWidget);
    expect(find.textContaining('2 回复'), findsOneWidget);
    expect(find.text('第 1 / 1 页'), findsOneWidget);
    final next = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '下一页'),
    );
    expect(next.onPressed, isNull);

    await tester.tap(find.byIcon(Icons.star));
    await tester.pumpAndSettle();
    expect(adapter.lastPath, '/open/favorite/delete/0.json');
    expect(find.byIcon(Icons.star_border), findsOneWidget);
  });

  testWidgets('top-ten thread opens directly and shows a board badge', (
    tester,
  ) async {
    final adapter = _FakeApiAdapter({
      '/open/widget/topten.json': {
        'title': '今日十大',
        'article': [
          {
            'id': '1',
            'title': 'top story',
            'board_name': 'Joke',
            'reply_count': 5,
            'post_time': 1794497375,
            'user': {'id': 'alice', 'user_name': 'Alice'},
          },
        ],
      },
      '/open/threads/Joke/1.json': {
        'id': '1',
        'title': 'top story',
        'board_name': 'Joke',
        'board_description': '笑话',
        'pagination': {
          'page_all_count': 1,
          'page_current_count': 1,
          'item_page_count': 50,
          'item_all_count': 1,
        },
        'article': [
          {
            'id': '1',
            'title': 'top story',
            'content': 'hello',
            'post_time': 1794497375,
            'user': {'id': 'alice', 'user_name': 'Alice'},
          },
        ],
      },
      '/open/board/Joke.json': {
        'name': 'Joke',
        'description': '笑话',
        'article': [],
      },
    });
    final api = _api(adapter);

    await tester.pumpWidget(_wrap(api, const TopTenPage()));
    await tester.pumpAndSettle();

    await tester.tap(find.text('top story'));
    await tester.pumpAndSettle();

    expect(find.byType(ThreadScreen), findsOneWidget);
    expect(find.byType(BoardScreen), findsNothing);
    expect(find.byType(ActionChip), findsOneWidget);
    expect(find.textContaining('笑话'), findsOneWidget);
    expect(find.text('第 1 / 1 页'), findsOneWidget);
    final next = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '下一页'),
    );
    expect(next.onPressed, isNull);

    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.byType(ThreadScreen), findsNothing);
  });

  testWidgets('bottom navigation has global compose and messages', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final adapter = _FakeApiAdapter({
      '/open/section.json': {
        'section': [
          {'name': 'TestSection', 'description': '测试分区'},
        ],
      },
      '/open/section/TestSection.json': {
        'name': 'TestSection',
        'description': '测试分区',
        'board': [
          {'name': 'Joke', 'description': '笑话'},
        ],
      },
    });
    final api = _api(adapter);

    await tester.pumpWidget(_wrap(api, const MobileHome()));
    await tester.pumpAndSettle();

    expect(find.text('首页'), findsWidgets);
    expect(find.text('十大'), findsWidgets);
    expect(find.text('发帖'), findsOneWidget);
    expect(find.text('消息'), findsOneWidget);
    expect(find.text('我'), findsOneWidget);

    await tester.tap(find.text('发帖'));
    await tester.pumpAndSettle();
    expect(find.text('选择分区'), findsOneWidget);

    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.text('测试分区'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('选择版面'), findsOneWidget);
    expect(find.text('笑话'), findsOneWidget);

    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('消息'));
    await tester.pumpAndSettle();
    expect(find.text('私信'), findsOneWidget);
    expect(find.text('信箱'), findsOneWidget);
    expect(find.text('提醒'), findsOneWidget);
  });

  test(
    'new posts omit reid while replies send the target article id',
    () async {
      final adapter = _FakeApiAdapter({});
      final api = _api(adapter);

      await api.forum.postArticle(board: 'Joke', title: 't', content: 'c');
      var form = adapter.last!.data as Map;
      expect(adapter.last!.uri.path, '/open/article/Joke/post.json');
      expect(form.containsKey('reid'), isFalse);

      await api.forum.postArticle(
        board: 'Joke',
        title: 'Re: t',
        content: 'c',
        reid: '123',
      );
      form = adapter.last!.data as Map;
      expect(form['reid'], '123');
    },
  );

  testWidgets('chat loads the body from the mail detail endpoint', (
    tester,
  ) async {
    final adapter = _FakeApiAdapter({
      '/open/mail/inbox.json': {
        'mail': [
          {
            'index': 1,
            'title': 'hello',
            'post_time': 1794497375,
            'user': {'id': 'alice', 'user_name': 'Alice'},
          },
        ],
      },
      '/open/mail/outbox.json': {'mail': []},
      '/open/mail/inbox/1.json': {
        'index': 1,
        'title': 'hello',
        'post_time': 1794497375,
        'user': {'id': 'alice', 'user_name': 'Alice'},
        'content': 'actual body',
        'attachment': [],
      },
    });
    final api = _api(adapter);

    await tester.pumpWidget(
      _wrap(api, const ChatScreen(contactId: 'alice', contactName: 'Alice')),
    );
    await tester.pumpAndSettle();

    expect(find.text('(无内容)'), findsNothing);
    final content = tester.widget<PostContent>(find.byType(PostContent));
    expect(content.content, 'actual body');
  });

  testWidgets('edit and delete are hidden on someone else\'s thread', (
    tester,
  ) async {
    final adapter = _FakeApiAdapter({
      '/open/threads/Joke/1.json': {
        'id': '1',
        'title': 'not mine',
        'board_name': 'Joke',
        'user': {'id': 'someone', 'user_name': 'Someone'},
        'article': [
          {
            'id': '1',
            'title': 'not mine',
            'content': 'hello',
            'user': {'id': 'someone', 'user_name': 'Someone'},
          },
        ],
      },
    });
    final api = _api(adapter);
    api.session.username = 'me';

    await tester.pumpWidget(
      _wrap(api, const ThreadScreen(boardName: 'Joke', threadId: '1')),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(PopupMenuButton<String>));
    await tester.pumpAndSettle();
    expect(find.text('编辑本帖'), findsNothing);
    expect(find.text('删除本帖'), findsNothing);
  });
}

ByrApi _api(_FakeApiAdapter adapter) {
  final dio = Dio(
    BaseOptions(responseType: ResponseType.plain, validateStatus: (_) => true),
  );
  dio.httpClientAdapter = adapter;
  return createAppServices(client: ApiClient(dio: dio)).api;
}

Widget _wrap(ByrApi api, Widget child) {
  return AppScope(
    services: createAppServices(api: api),
    child: MaterialApp(home: child),
  );
}

class _FakeApiAdapter implements HttpClientAdapter {
  _FakeApiAdapter(this.responses);

  final Map<String, Object?> responses;
  String? lastPath;
  RequestOptions? last;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    last = options;
    lastPath = options.uri.path;
    final body = responses[options.uri.path] ?? const {};
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
