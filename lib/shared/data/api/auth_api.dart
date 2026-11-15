import 'dart:convert';

import 'package:crypto/crypto.dart';

import 'package:byr_client/shared/core/constants.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/oauth_info.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/data/api/api_base.dart';
import 'package:byr_client/shared/data/api/api_client.dart';

/// Result of the nForum web-login diagnostic.
class WebLoginResult {
  final bool ok;
  final String code;
  final String raw;
  final String message;

  const WebLoginResult({
    required this.ok,
    required this.code,
    required this.raw,
    required this.message,
  });
}

/// Login service.
///
/// Recovered from `_LoginService::postAuthInfo` and
/// `OAuthInfoRequestBody::make`, then verified against the live server:
///
/// ```text
/// ts      = round(DateTime.now().millisecondsSinceEpoch / 1000)
/// appkey  = md5("39c43..." + "wtfaaa" + "com.byr.bbs" + ts)
/// source  = ts + "-1"
/// username/password = base64(codeUnits)
/// POST https://bbs.byr.cn/oauth2/official  (form-urlencoded)
/// ```
///
/// The constants come first and the timestamp last (same layout as the
/// official v3 client). Reversing either order makes the server reply with an
/// empty 200 instead of a real JSON envelope.
class AuthApi extends BaseApi {
  AuthApi(super.client);

  Future<Result<OAuthInfo>> login(String username, String password) async {
    try {
      final json = await client.postAbsoluteForm(
        Constants.loginUrl,
        buildLoginBody(username: username, password: password),
      );

      if (json.isEmpty) {
        return const Result.failure(
          '登录接口返回空响应（HTTP 200）。\n'
          '这通常说明 appkey/source 签名不被服务器接受，'
          '请先确认设备时间准确，再把下面的诊断信息复制反馈。',
        );
      }

      final info = OAuthInfo.fromJson(json);
      if ((info.accessToken ?? '').isEmpty) {
        return Result.failure(info.msg ?? info.error ?? '登录响应缺少 access_token');
      }
      return Result.success(info);
    } on ApiException catch (error) {
      return Result.failure(friendlyMessage(error));
    } catch (error) {
      return Result.failure(error.toString());
    }
  }

  /// Turns the server's terse error envelope into something readable.
  static String friendlyMessage(ApiException error) {
    final code = error.code;
    if (code == '65') {
      return '用户名或密码错误（服务器返回 code 65）。\n'
          '请检查是否用论坛用户名（不是邮箱）登录，'
          '以及密码是否被输入法自动大写或加了空格。';
    }
    if (code != null && code.isNotEmpty) {
      return '${error.message}（code $code）';
    }
    return error.message;
  }

  /// Calls the normal nForum web login endpoint. Unlike `/oauth2/official`,
  /// it returns a readable `ajax_st` / `ajax_code` JSON envelope, so it can
  /// tell whether the credentials themselves are valid.
  Future<WebLoginResult> webLoginCheck(String username, String password) async {
    try {
      final outcome = await client.webLogin(username, password);
      return WebLoginResult(
        ok: outcome.ok,
        code: outcome.ajaxCode ?? '',
        raw:
            'HTTP ${outcome.statusCode}, '
            'ajax_st=${outcome.ajaxSt ?? '?'}, '
            'ajax_code=${outcome.ajaxCode ?? '?'}, '
            '${outcome.bodyLength}B, '
            'cookie=${outcome.cookieNames.join(',')}',
        message: outcome.explanation,
      );
    } catch (error) {
      return WebLoginResult(
        ok: false,
        code: '',
        raw: error.toString(),
        message: '请求失败',
      );
    }
  }

  /// Logs in through the nForum web session.
  ///
  /// This validates the credentials, but the web session cannot be used
  /// against `/open` (which requires `oauth_token`). Kept for diagnostics.
  Future<Result<User>> cookieLogin(String username, String password) async {
    try {
      final outcome = await client.webLogin(username, password);
      if (!outcome.ok) {
        return Result.failure(
          '网页登录失败（${outcome.explanation}'
          '${outcome.ajaxCode == null ? '' : '，ajax_code ${outcome.ajaxCode}'}）',
        );
      }
      return const Result.failure(
        '网页登录成功，但 /open 接口只接受 oauth_token，不接受网页 Cookie。\n'
        '请使用“登录”按钮（OAuth 通道）。',
      );
    } catch (error) {
      return Result.failure(error.toString());
    }
  }

  /// Probe results used when cookie login fails, so the user can report which
  /// backend actually accepts the web session.
  Future<List<String>> cookieProbes() async {
    final open = Constants.openUrl;
    return [
      'user/ajax_session.json → '
          '${await client.probe('https://bbs.byr.cn/user/ajax_session.json')}',
      'section/ajax_list.json → '
          '${await client.probe('https://bbs.byr.cn/section/ajax_list.json')}',
      'open/user/getinfo.json → ${await client.probe('$open/user/getinfo.json')}',
      'open/section.json → ${await client.probe('$open/section.json')}',
    ];
  }

  /// Exposed for tests and for the agent-side reimplementation.
  static Map<String, String> buildLoginBody({
    required String username,
    required String password,
    DateTime? now,
  }) {
    // The official app uses `round(ms / 1000)`, not integer division.
    final seconds = ((now ?? DateTime.now()).millisecondsSinceEpoch / 1000)
        .round();
    final timestamp = seconds.toString();
    final appkey = md5
        .convert(
          utf8.encode(
            // Constant block first, timestamp last. This is the order used by
            // the official v3 client and accepted by the v4 `/oauth2/official`
            // endpoint (verified against the live server).
            Constants.appKeySecret +
                Constants.appKeySalt +
                Constants.appKeyPackage +
                timestamp,
          ),
        )
        .toString();

    return {
      'appkey': appkey,
      // v3: `dateStr + identifier`; v4 embeds the same "-1" platform code
      // after the timestamp.
      'source': '$timestamp-1',
      // The APK base64-encodes `String.codeUnits` (UTF-16 code units), not
      // UTF-8 bytes.
      'username': base64Encode(username.codeUnits),
      'password': base64Encode(password.codeUnits),
    };
  }
}
