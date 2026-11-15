import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/app/app_services.dart';
import 'package:byr_client/app/bootstrap.dart';
import 'package:byr_client/app/byr_app.dart';
import 'package:byr_client/features/compose/compose_screen.dart';
import 'package:byr_client/platforms/android/android_system_ui.dart';
import 'package:byr_client/platforms/app_platform.dart';
import 'package:byr_client/platforms/desktop/desktop_home.dart';
import 'package:byr_client/platforms/mobile/mobile_home.dart';
import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/widgets/post_content.dart';

import '../support/fakes.dart';

void main() {
  for (final platform in AppPlatform.values) {
    testWidgets('${platform.name} restores into its composed shell', (
      tester,
    ) async {
      tester.view.physicalSize = platform.isDesktop
          ? const Size(1280, 800)
          : const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final services = createAppServices(
        platform: platform,
        preferences: MemoryPreferences(),
        device: FakeDeviceServices(),
        client: FakeForumServer().createClient(),
      );
      await tester.pumpWidget(ByrApp(services: services, loggedIn: true));
      await tester.pumpAndSettle();
      expect(
        find.byType(DesktopHome),
        platform.isDesktop ? findsOneWidget : findsNothing,
      );
      expect(
        find.byType(MobileHome),
        platform.isDesktop ? findsNothing : findsOneWidget,
      );
      expect(
        find.byType(NavigationBar),
        platform.isDesktop ? findsNothing : findsOneWidget,
      );
      expect(
        services.systemUiStyle(Brightness.dark),
        platform.isDesktop ? isNull : isA<SystemUiOverlayStyle>(),
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('successful login uses the injected shell strategy', (
    tester,
  ) async {
    final store = MemoryPreferences();
    final server = FakeForumServer({
      '/oauth2/official': {'access_token': 'test-token'},
      '/open/user/getinfo.json': {'id': 'alice'},
    });
    final services = createAppServices(
      platform: AppPlatform.android,
      preferences: store,
      device: FakeDeviceServices(),
      client: server.createClient(),
      homeBuilder: (_) => const Scaffold(body: Text('injected home')),
    );
    await tester.pumpWidget(ByrApp(services: services, loggedIn: false));
    await tester.enterText(find.byType(TextField).at(0), 'alice');
    await tester.enterText(find.byType(TextField).at(1), 'password');
    await tester.tap(find.widgetWithText(FilledButton, '登录'));
    await tester.pumpAndSettle();
    expect(find.text('injected home'), findsOneWidget);
    expect(await store.readString('accessToken'), 'test-token');
  });

  testWidgets('shared attachment content opens through the device adapter', (
    tester,
  ) async {
    final device = FakeDeviceServices();
    final services = createAppServices(
      preferences: MemoryPreferences(),
      device: device,
    );
    await tester.pumpWidget(
      AppScope(
        services: services,
        child: const MaterialApp(
          home: Scaffold(
            body: PostContent(
              content: '',
              token: 'token',
              attachments: [
                Attachment(
                  files: [
                    FileElement(
                      name: 'report.pdf',
                      url: 'https://bbs.byr.cn/report.pdf',
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('report.pdf'));
    expect(device.openedLink?.path, '/report.pdf');
    expect(device.openedLink?.queryParameters['oauth_token'], 'token');
  });

  testWidgets('cancelled file selection stays inside the composer', (
    tester,
  ) async {
    final device = FakeDeviceServices();
    final server = FakeForumServer();
    final services = createAppServices(
      preferences: MemoryPreferences(),
      device: device,
      client: server.createClient(),
    );
    await tester.pumpWidget(
      AppScope(
        services: services,
        child: const MaterialApp(
          home: ComposeScreen(boardName: 'Joke', allowAnonymous: false),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('图片/附件'));
    await tester.pumpAndSettle();
    expect(device.pickerCalls, 1);
    expect(server.requests, isEmpty);
    expect(find.byType(ComposeScreen), findsOneWidget);
  });

  testWidgets('leaving the editor saves a snapshot after controller disposal', (
    tester,
  ) async {
    final services = createAppServices(
      preferences: MemoryPreferences(),
      device: FakeDeviceServices(),
      client: FakeForumServer().createClient(),
    );
    await _openComposer(tester, services);
    await tester.enterText(find.byType(TextField).at(0), 'draft title');
    await tester.enterText(find.byType(TextField).at(1), 'draft body');
    await tester.pageBack();
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, '退出'));
    await tester.pumpAndSettle();
    final draft = await services.drafts.load('composeDraft.Joke.new');
    expect(draft?.title, 'draft title');
    expect(draft?.content, 'draft body');
    expect(tester.takeException(), isNull);
  });

  testWidgets('publishing removes the draft without disposal re-saving it', (
    tester,
  ) async {
    final services = createAppServices(
      preferences: MemoryPreferences(),
      device: FakeDeviceServices(),
      client: FakeForumServer({
        '/open/article/Joke/post.json': {'id': '1'},
      }).createClient(),
    );
    await _openComposer(tester, services);
    await tester.enterText(find.byType(TextField).at(0), 'new thread');
    await tester.enterText(find.byType(TextField).at(1), 'content');
    await tester.pump(const Duration(milliseconds: 900));
    expect(await services.drafts.load('composeDraft.Joke.new'), isNotNull);
    await tester.tap(find.widgetWithText(FilledButton, '发布'));
    await tester.pumpAndSettle();
    expect(find.byType(ComposeScreen), findsNothing);
    expect(await services.drafts.load('composeDraft.Joke.new'), isNull);
    expect(tester.takeException(), isNull);
  });

  test('Android system bars match both themes and disable forced contrast', () {
    expect(
      androidSystemUiStyle(Brightness.dark).systemNavigationBarIconBrightness,
      Brightness.light,
    );
    expect(
      androidSystemUiStyle(Brightness.light).systemNavigationBarIconBrightness,
      Brightness.dark,
    );
    expect(
      androidSystemUiStyle(Brightness.dark).systemNavigationBarContrastEnforced,
      isFalse,
    );
  });
}

Future<void> _openComposer(WidgetTester tester, AppServices services) async {
  await tester.pumpWidget(
    AppScope(
      services: services,
      child: MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => const ComposeScreen(
                    boardName: 'Joke',
                    allowAnonymous: false,
                  ),
                ),
              ),
              child: const Text('open composer'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open composer'));
  await tester.pumpAndSettle();
}
