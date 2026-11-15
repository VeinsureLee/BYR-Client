/// Endpoints and protocol constants recovered from the official APK.
///
/// Recovered from `package:byr_flutter/constants/constants.dart`:
///   `openUrl()  -> host + "/open"`
///   `exApiUrl() -> host + "/exp"`
///   `apiUrl()   -> host + "/api"`
/// The host switches between the IPv6 mirror and the IPv4 host.
class Constants {
  Constants._();

  static const String hostV6 = 'https://bbs6.byr.cn';
  static const String hostV4 = 'https://bbs.byr.cn';

  /// Toggle in the original app settings ("openIPv6"). Defaults to IPv4.
  static bool useIPv6 = false;

  static String get host => useIPv6 ? hostV6 : hostV4;

  /// Business API base (ForumService, BoardService, ...).
  static String get openUrl => '$host/open';

  /// Experimental encrypted API base (EncryptedDataService).
  static String get exApiUrl => '$host/exp';

  /// Login base. The original `Network._loginService()` hard-codes the IPv4
  /// host, so the login URL never carries `/open`.
  static const String loginUrl = '$hostV4/oauth2/official';

  /// OAuth request constants recovered from `OAuthInfoRequestBody.make`.
  static const String appKeySecret = '39c43ccebd1dafca245bec76b3be8107';
  static const String appKeySalt = 'wtfaaa';
  static const String appKeyPackage = 'com.byr.bbs';

  /// Ed25519 public key used to verify `/exp` responses (base64, 32 bytes).
  static const String ed25519PublicKey =
      'qtjZC1hWINhsGfWlPD6vK8WoaGtWUY4XRpusjw+2kOE=';

  /// Asset bundled in the original APK; usage unconfirmed.
  static const String legacyPublicKeyAsset = 'assets/public_key.pem';

  static const Duration connectTimeout = Duration(seconds: 15);
  static const Duration receiveTimeout = Duration(seconds: 30);
}
