import 'package:byr_client/app/bootstrap.dart';

import 'dart:typed_data';

import 'package:byr_client/app/byr_app.dart';
import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/data/api/auth_api.dart';
import 'package:byr_client/shared/data/api/favorite_api.dart';
import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/collection.dart';
import 'package:byr_client/shared/data/models/encrypted_data.dart';
import 'package:byr_client/shared/data/models/thread.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/data/models/vote.dart';
import 'package:byr_client/features/login/login_screen.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:byr_client/shared/widgets/reaction_button.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:material_ui/material_ui.dart';

void main() {
  test('login body matches the recovered protocol', () {
    final body = AuthApi.buildLoginBody(
      username: 'alice',
      password: 'secret',
      now: DateTime.fromMillisecondsSinceEpoch(1700000000000),
    );

    expect(body['source'], '1700000000-1');
    expect(body['username'], 'YWxpY2U=');
    expect(body['password'], 'c2VjcmV0');
    expect(body['appkey'], '3f8dc52a83ee06a56d62645863ce4a69');
  });

  test('article parses the server envelope', () {
    final article = Article.fromJson({
      'request': '/article/Test/1.json',
      'code': null,
      'msg': null,
      'id': '1',
      'title': 'hello',
      'board_name': 'Test',
      'reply_count': 3,
      'is_top': 0,
      'user': {'id': 'u1', 'user_name': 'alice'},
    });

    expect(article.id, '1');
    expect(article.title, 'hello');
    expect(article.replyCount, 3);
    expect(article.isTop, isFalse);
    expect(article.user?.userName, 'alice');
  });

  test('thread parses nested articles and pagination', () {
    final thread = Thread.fromJson({
      'id': '1',
      'title': 'topic',
      'article': [
        {'id': '1', 'title': 'topic', 'content': 'first'},
        {'id': '2', 'content': 'reply'},
      ],
      'pagination': {
        'page_all_count': 2,
        'page_current_count': 1,
        'item_page_count': 2,
        'item_all_count': 4,
      },
    });

    expect(thread.articles, hasLength(2));
    expect(thread.articles.first.content, 'first');
    expect(thread.pagination?.pageAllCount, 2);
  });

  test('board parses manager and article list', () {
    final board = Board.fromJson({
      'name': 'Test',
      'description': 'test board',
      'class': 'c',
      'section': 'Section',
      'manager': [
        {'id': 'u1', 'user_name': 'mod'},
      ],
      'is_favorite': 1,
      'article': [
        {'id': '9', 'title': 'topic'},
      ],
    });

    expect(board.name, 'Test');
    expect(board.boardClass, 'c');
    expect(board.section, 'Section');
    expect(board.managers.single.userName, 'mod');
    expect(board.isFavorite, isTrue);
    expect(board.articles.single.title, 'topic');
  });

  test('vote parses options and status', () {
    final vote = Vote.fromJson({
      'vid': '42',
      'title': 'pick one',
      'type': 1,
      'options': [
        {'viid': '1', 'label': 'A', 'num': 3},
      ],
      'vote_status': {'viid': '1', 'time': '2026-01-01'},
    });

    expect(vote.vid, '42');
    expect(vote.options.single.label, 'A');
    expect((vote.voteStatus as VoteStatus).viid, '1');
  });

  test('encrypted data parses the experimental envelope', () {
    final data = EncryptedData.fromJson({
      'content': 'abc',
      'public_key': 'def',
      'sign': 'ghi',
    });

    expect(data.content, 'abc');
    expect(data.publicKey, 'def');
    expect(data.sign, 'ghi');
  });

  test('single attachment object is accepted', () {
    final article = Article.fromJson({
      'id': '1',
      'attachment': {
        'file': {'name': 'sand.jpg', 'url': '/api/attachment/sand.jpg'},
      },
    });

    expect(article.attachments, hasLength(1));
    expect(article.attachments.single.files.single.name, 'sand.jpg');
  });

  test('attachment parses live size and remaining-space strings', () {
    final attachment = Attachment.fromJson({
      'file': [
        {'name': 'a.jpg', 'size': '3.2MB'},
      ],
      'remain_space': '1.8MB',
      'remain_count': 19,
    });

    expect(attachment.files.single.size, '3.2MB');
    expect(attachment.remainSpace, '1.8MB');
    expect(attachment.remainCount, 19);
  });

  test('attachment urls are normalized and tokenized', () {
    expect(
      prepareAttachmentUrl('/api/attachment/a.jpg', 'tok'),
      'https://bbs.byr.cn/open/attachment/a.jpg?oauth_token=tok',
    );
    expect(
      prepareAttachmentUrl('/files/a.jpg', 'tok'),
      'https://bbs.byr.cn/files/a.jpg?oauth_token=tok',
    );
    expect(
      prepareAttachmentUrl('//static.byr.cn/a.jpg', 'tok'),
      'https://static.byr.cn/a.jpg?oauth_token=tok',
    );
    expect(
      prepareAttachmentUrl('https://example.com/a.jpg', 'tok'),
      'https://example.com/a.jpg',
    );
    expect(
      prepareAttachmentUrl('https://bbs.byr.cn/files/a.jpg?x=1', 'tok'),
      'https://bbs.byr.cn/files/a.jpg?x=1&oauth_token=tok',
    );
    expect(prepareAttachmentUrl(null, 'tok'), isNull);
  });

  testWidgets('upload markers become images', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: PostContent(
            content: '沙雕图来了\n[upload=1][/upload]',
            token: 'tok',
            attachments: [
              Attachment(
                files: [
                  FileElement(
                    name: 'sand.jpg',
                    url: '/api/attachment/sand.jpg',
                    thumbnailMiddle: '/api/attachment/sand_m.jpg',
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );

    expect(find.byType(CachedNetworkImage), findsOneWidget);
    expect(find.textContaining('[upload='), findsNothing);
    expect(find.textContaining('沙雕图来了'), findsOneWidget);
  });

  test('collection parses the live article list shape', () {
    final collection = Collection.fromJson({
      'article': [
        {
          'createdTime': 1794153774,
          'title': '[求助]本科生算法秋招',
          'postTime': 1794089596,
          'bname': 'WorkLife',
          'num': 21,
          'gid': 1241765,
          'user': {'id': 'Lambkin', 'user_name': '秦绍峰'},
        },
        {
          'createdTime': 1794153775,
          'title': '匿名帖',
          'postTime': 1794089597,
          'bname': 'IWhisper',
          'num': 2,
          'gid': 9061765,
          'user': {'id': 'IWhisper#398', 'user_name': null},
        },
      ],
      'pagination': {
        'page_all_count': 4,
        'page_current_count': 1,
        'item_page_count': 50,
        'item_all_count': 178,
      },
    });

    expect(collection.entries, hasLength(2));
    expect(collection.entries.first.bname, 'WorkLife');
    expect(collection.entries.first.gid, 1241765);
    expect(collection.entries.first.num, 21);
    expect(displayUserName(collection.entries.last.user), 'IWhisper#398');
    expect(collection.pagination?.pageAllCount, 4);
  });

  test('anonymous whisper users fall back to their generated id', () {
    final whisper = User.fromJson({'id': 'IWhisper#93', 'user_name': null});
    expect(displayUserName(whisper), 'IWhisper#93');
    expect(isAnonymousUser(whisper), isTrue);

    final normal = User.fromJson({'id': 'alice', 'user_name': 'Alice'});
    expect(displayUserName(normal), 'Alice');
    expect(isAnonymousUser(normal), isFalse);
  });

  test('board parses the allow_anonymous flag', () {
    final whisper = Board.fromJson({
      'name': 'IWhisper',
      'description': '悄悄话',
      'allow_anonymous': true,
    });
    final joke = Board.fromJson({
      'name': 'Joke',
      'description': '笑话',
      'allow_anonymous': false,
    });
    expect(whisper.allowAnonymous, isTrue);
    expect(joke.allowAnonymous, isFalse);
  });

  testWidgets('reaction button animates between unselected and selected', (
    tester,
  ) async {
    Widget build(bool selected) => MaterialApp(
      home: Scaffold(
        body: ReactionButton(
          selected: selected,
          icon: Icons.star_border,
          selectedIcon: Icons.star,
          label: selected ? '已收藏' : '收藏',
          onPressed: () {},
        ),
      ),
    );

    await tester.pumpWidget(build(false));
    expect(find.byIcon(Icons.star_border), findsOneWidget);
    expect(find.text('收藏'), findsOneWidget);

    await tester.pumpWidget(build(true));
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.star), findsOneWidget);
    expect(find.text('已收藏'), findsOneWidget);
  });

  testWidgets('theme and font scale are applied to the app', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final services = createAppServices();
    final api = services.api;
    api.session.fontScale.value = 1.3;
    api.session.themeMode.value = 2;
    await tester.pumpWidget(ByrApp(services: services, loggedIn: false));
    await tester.pump();
    final context = tester.element(find.byType(LoginScreen));
    expect(MediaQuery.textScalerOf(context).scale(10), closeTo(13, 0.001));
    expect(Theme.of(context).brightness, Brightness.dark);
  });

  test('history store deduplicates thread entries', () async {
    SharedPreferences.setMockInitialValues({});
    final history = createAppServices().history;
    await history.add(
      const HistoryEntry(board: 'Joke', id: '1', title: 'old', visitedAt: 1),
    );
    await history.add(
      const HistoryEntry(board: 'Joke', id: '1', title: 'new', visitedAt: 2),
    );
    final entries = await history.load();
    expect(entries, hasLength(1));
    expect(entries.single.title, 'new');
  });

  testWidgets('post content renders the bundled emoji asset', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: PostContent(content: 'hello[em1] world')),
      ),
    );

    expect(find.byType(Image), findsOneWidget);
    expect(find.textContaining('[em1]'), findsNothing);
  });

  test('timestamps are converted from Unix seconds', () {
    expect(formatTimestamp(null), '');
    expect(formatTimestamp(''), '');
    expect(formatTimestamp('2026-01-02 03:04'), '2026-01-02 03:04');

    final local = DateTime(2026, 1, 2, 3, 4);
    final seconds = local.millisecondsSinceEpoch ~/ 1000;
    expect(
      formatTimestamp('$seconds', alwaysShowYear: true),
      '2026-01-02 03:04',
    );
    expect(formatTimestamp('0'), '');
  });

  test(
    'board favorites use the fixed directory path and form fields',
    () async {
      final adapter = _RecordingAdapter();
      final dio = Dio(
        BaseOptions(
          responseType: ResponseType.plain,
          validateStatus: (_) => true,
        ),
      );
      dio.httpClientAdapter = adapter;
      final api = FavoriteApi(ApiClient(dio: dio));

      await api.addFavorite('Joke');

      final request = adapter.last!;
      expect(request.method, 'POST');
      expect(request.uri.path, '/open/favorite/add/0.json');
      final form = request.data as Map;
      expect(form['name'], 'Joke');
      expect(form['dir'], '0');
    },
  );
}

class _RecordingAdapter implements HttpClientAdapter {
  RequestOptions? last;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    last = options;
    return ResponseBody.fromString('{}', 200);
  }

  @override
  void close({bool force = false}) {}
}
