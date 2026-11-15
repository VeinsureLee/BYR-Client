import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import 'package:byr_client/platforms/app_platform.dart';
import 'package:byr_client/shared/data/byr_api.dart';
import 'package:byr_client/shared/services/device_services.dart';
import 'package:byr_client/shared/storage/draft_store.dart';
import 'package:byr_client/shared/storage/history_store.dart';

/// Explicit dependencies owned by one application instance; no global registry.
class AppServices {
  const AppServices({
    required this.api,
    required this.history,
    required this.drafts,
    required this.device,
    required this.platform,
    required this.homeBuilder,
    required this.systemUiStyle,
  });

  final ByrApi api;
  final HistoryStore history;
  final DraftStore drafts;
  final DeviceServices device;
  final AppPlatform platform;

  /// Strategy in function form: startup and login use the same injected shell.
  final WidgetBuilder homeBuilder;
  final SystemUiOverlayStyle? Function(Brightness) systemUiStyle;
}
