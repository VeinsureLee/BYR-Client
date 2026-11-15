import 'dart:convert';
import 'dart:io' show Cookie;

import 'package:flutter/foundation.dart';
import 'package:byr_client/shared/storage/preference_store.dart';

/// Shared session state and persistence, independent of the storage plugin.
/// Theme and font listenables use Flutter's native Observer mechanism.
class Session {
  Session(this.preferences);

  final PreferenceStore preferences;
  static const String _tokenKey = 'accessToken';
  static const String _usernameKey = 'username';
  static const String _ipv6Key = 'openIPv6';
  static const String _cookiesKey = 'nforumCookies';
  static const String _themeKey = 'themeMode';
  static const String _fontScaleKey = 'fontScale';

  String? token;
  String? username;
  bool useIPv6 = false;
  List<Cookie> cookies = const [];

  /// 0 = system, 1 = light, 2 = dark (ThemeMode ordering).
  final ValueNotifier<int> themeMode = ValueNotifier<int>(0);
  final ValueNotifier<double> fontScale = ValueNotifier<double>(1.0);

  Future<void> load() async {
    token = await preferences.readString(_tokenKey);
    username = await preferences.readString(_usernameKey);
    useIPv6 = (await preferences.readBool(_ipv6Key)) ?? false;
    cookies = _decodeCookies(await preferences.readString(_cookiesKey));
    themeMode.value = (await preferences.readInt(_themeKey)) ?? 0;
    fontScale.value = (await preferences.readDouble(_fontScaleKey)) ?? 1.0;
  }

  Future<void> saveThemeMode(int value) async {
    themeMode.value = value.clamp(0, 2).toInt();
    await preferences.write(_themeKey, themeMode.value);
  }

  Future<void> saveFontScale(double value) async {
    fontScale.value = value.clamp(0.8, 1.4).toDouble();
    await preferences.write(_fontScaleKey, fontScale.value);
  }

  Future<void> saveUseIPv6(bool value) async {
    useIPv6 = value;
    await preferences.write(_ipv6Key, value);
  }

  Future<void> saveToken(String value) async {
    token = value;
    await preferences.write(_tokenKey, value);
  }

  Future<void> saveUsername(String value) async {
    username = value;
    await preferences.write(_usernameKey, value);
  }

  Future<void> saveCookies(List<Cookie> value) async {
    cookies = List.unmodifiable(value);
    await preferences.write(_cookiesKey, _encodeCookies(value));
  }

  Future<void> clear() async {
    token = null;
    username = null;
    cookies = const [];
    await preferences.remove(_tokenKey);
    await preferences.remove(_usernameKey);
    await preferences.remove(_cookiesKey);
  }

  bool get isLoggedIn => token != null && token!.isNotEmpty;
  bool get hasCookies => cookies.isNotEmpty;

  /// Cookie-only sessions cannot call `/open` (it requires `oauth_token`),
  /// so they must not be treated as logged in.
  bool get isAuthenticated => isLoggedIn;

  static String _encodeCookies(List<Cookie> cookies) => jsonEncode(
    cookies
        .map(
          (cookie) => {
            'name': cookie.name,
            'value': cookie.value,
            'domain': cookie.domain,
            'path': cookie.path,
            'expires': cookie.expires?.toIso8601String(),
            'httpOnly': cookie.httpOnly,
            'secure': cookie.secure,
          },
        )
        .toList(),
  );

  static List<Cookie> _decodeCookies(String? raw) {
    if (raw == null || raw.isEmpty) return const [];
    try {
      final list = jsonDecode(raw);
      if (list is! List) return const [];
      return list
          .whereType<Map>()
          .map((item) {
            final map = item.cast<String, dynamic>();
            final cookie = Cookie(
              map['name']?.toString() ?? '',
              map['value']?.toString() ?? '',
            );
            final domain = map['domain']?.toString();
            if (domain != null && domain.isNotEmpty) cookie.domain = domain;
            final path = map['path']?.toString();
            if (path != null && path.isNotEmpty) cookie.path = path;
            final expires = map['expires']?.toString();
            if (expires != null && expires.isNotEmpty) {
              cookie.expires = DateTime.tryParse(expires);
            }
            cookie.httpOnly = map['httpOnly'] == true;
            cookie.secure = map['secure'] == true;
            return cookie;
          })
          .where((cookie) => cookie.name.isNotEmpty)
          .toList(growable: false);
    } catch (_) {
      return const [];
    }
  }
}
