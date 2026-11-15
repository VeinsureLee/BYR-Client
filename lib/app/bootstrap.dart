import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_services.dart';
import 'package:byr_client/platforms/adapters/flutter_device_services.dart';
import 'package:byr_client/platforms/adapters/shared_preferences_store.dart';
import 'package:byr_client/platforms/android/android_system_ui.dart';
import 'package:byr_client/platforms/app_platform.dart';
import 'package:byr_client/platforms/desktop/desktop_home.dart';
import 'package:byr_client/platforms/mobile/mobile_home.dart';
import 'package:byr_client/platforms/platform_detector.dart';
import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/data/byr_api.dart';
import 'package:byr_client/shared/services/device_services.dart';
import 'package:byr_client/shared/storage/draft_store.dart';
import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/storage/preference_store.dart';
import 'package:byr_client/shared/storage/session.dart';

/// Composition root: platform decisions and concrete Adapters meet here only.
AppServices createAppServices({
  AppPlatform? platform,
  PreferenceStore? preferences,
  DeviceServices? device,
  ApiClient? client,
  ByrApi? api,
  WidgetBuilder? homeBuilder,
}) {
  final target = platform ?? detectPlatform();
  final store = preferences ?? SharedPreferencesStore();
  return AppServices(
    api: api ?? ByrApi(client: client, session: Session(store)),
    history: HistoryStore(store),
    drafts: DraftStore(store),
    device: device ?? const FlutterDeviceServices(),
    platform: target,
    homeBuilder:
        homeBuilder ??
        (target.isDesktop
            ? (_) => const DesktopHome()
            : (_) => const MobileHome()),
    systemUiStyle: target == AppPlatform.android
        ? androidSystemUiStyle
        : (_) => null,
  );
}

Future<AppServices> bootstrapApplication() async {
  WidgetsFlutterBinding.ensureInitialized();
  final services = createAppServices();
  if (services.platform == AppPlatform.android) {
    await initializeAndroidSystemUi();
  }
  await services.api.restore();
  return services;
}
