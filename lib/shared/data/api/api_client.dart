import 'dart:convert';

import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio/dio.dart';

import 'package:byr_client/shared/core/constants.dart';

class ApiException implements Exception {
  final String? code;
  final String message;

  const ApiException(this.code, this.message);

  @override
  String toString() => 'ApiException(${code ?? '-'}): $message';
}

/// Outcome of the nForum web-login endpoint (`/user/ajax_login.json`).
class WebLoginResponse {
  const WebLoginResponse({
    required this.statusCode,
    required this.ajaxSt,
    required this.ajaxCode,
    required this.bodyLength,
    required this.cookieNames,
  });

  final int statusCode;
  final String? ajaxSt;
  final String? ajaxCode;
  final int bodyLength;
  final List<String> cookieNames;

  bool get ok => ajaxSt == '1';

  /// Human readable explanation for the common `ajax_code` values.
  String get explanation {
    switch (ajaxCode) {
      case '0100':
        return '未输入用户名';
      case '0101':
        return '用户名不存在，或密码错误';
      case '0102':
        return '登录过于频繁，请稍后再试';
      case '0103':
        return '账号已被封禁';
      case '0104':
        return '需要验证码';
      default:
        return ajaxSt == '1' ? '网页登录成功' : '网页登录失败';
    }
  }
}

/// HTTP client for the `/open` API.
///
/// Behaviour recovered from the APK:
///  * business requests live under [Constants.openUrl];
///  * every authenticated request carries the token as the `oauth_token`
///    query parameter (no Authorization header);
///  * API paths must end with `.json`.
class ApiClient {
  ApiClient({Dio? dio})
    : dio =
          dio ??
          Dio(
            BaseOptions(
              connectTimeout: Constants.connectTimeout,
              receiveTimeout: Constants.receiveTimeout,
              responseType: ResponseType.plain,
              validateStatus: (_) => true,
              headers: {
                'User-Agent': 'byr_client/4.0.0 (Dart)',
                'Accept': 'application/json, text/plain, */*',
              },
            ),
          ) {
    // NOTE: no CookieManager interceptor here. The nForum server uses cookie
    // names such as `nforum[UTMPUSERID]`; `dart:io`'s Cookie rejects `[`/`]`
    // when serializing, and the interceptor then throws a
    // CookieManagerSaveException, masking every response. The OAuth `/open`
    // API authenticates with the `oauth_token` query parameter instead.
  }

  final Dio dio;

  /// Cookie store kept for manual web-session handling.
  final CookieJar cookieJar = CookieJar();

  /// Current OAuth token (`accessToken` in the original app).
  String? token;

  /// When true, requests rely on nForum session cookies instead of a token.
  bool cookieMode = false;

  String get baseUrl => Constants.openUrl;

  /// Logs in through the normal nForum web endpoint and keeps the cookies.
  ///
  /// Returns the raw `ajax_st` value (1 means success).
  Future<WebLoginResponse> webLogin(String username, String password) async {
    final response = await dio.post<String>(
      'https://bbs.byr.cn/user/ajax_login.json',
      data: {'id': username, 'passwd': password},
      options: Options(
        contentType: Headers.formUrlEncodedContentType,
        responseType: ResponseType.plain,
        validateStatus: (_) => true,
        headers: {
          'User-Agent': 'Mozilla/5.0',
          'X-Requested-With': 'XMLHttpRequest',
          'Referer': 'https://bbs.byr.cn/',
          'Origin': 'https://bbs.byr.cn',
        },
      ),
    );
    final text = response.data ?? '';
    final st = RegExp(r'"ajax_st"\s*:\s*(\d+)').firstMatch(text)?.group(1);
    final code = RegExp(r'"ajax_code"\s*:\s*"([^"]*)"')
        .firstMatch(text)
        ?.group(1);
    final setCookies = response.headers.map['set-cookie'] ?? const <String>[];
    final cookieNames = setCookies
        .map((value) => value.split(';').first.split('=').first.trim())
        .where((name) => name.isNotEmpty)
        .toSet()
        .toList();
    if (st == '1') {
      cookieMode = true;
      token = null;
    }
    return WebLoginResponse(
      statusCode: response.statusCode ?? 0,
      ajaxSt: st,
      ajaxCode: code,
      bodyLength: text.length,
      cookieNames: cookieNames,
    );
  }

  /// Restores cookies saved by [Session.saveCookies].
  Future<void> restoreCookies(List<Cookie> cookies) async {
    if (cookies.isEmpty) return;
    await cookieJar.saveFromResponse(Uri.parse('https://bbs.byr.cn'), cookies);
    cookieMode = true;
  }

  /// Returns the cookies currently stored for bbs.byr.cn.
  Future<List<Cookie>> currentCookies() =>
      cookieJar.loadForRequest(Uri.parse('https://bbs.byr.cn'));

  /// Fetches a URL with the current cookie jar and returns a diagnostic line.
  Future<String> probe(String url) async {
    try {
      final response = await dio.get<String>(
        url,
        options: Options(
          responseType: ResponseType.plain,
          validateStatus: (_) => true,
          headers: {
            'User-Agent': 'Mozilla/5.0',
            'X-Requested-With': 'XMLHttpRequest',
            'Accept': 'application/json, text/plain, */*',
          },
        ),
      );
      final body = response.data ?? '';
      final preview = body.length > 120 ? '${body.substring(0, 120)}…' : body;
      return 'HTTP ${response.statusCode}, ${body.length} 字节: $preview';
    } catch (error) {
      return '请求失败: $error';
    }
  }

  Future<Map<String, dynamic>> get(
    String path, {
    Map<String, dynamic>? query,
    bool auth = true,
  }) => _request('GET', path, query: query, auth: auth);

  Future<Map<String, dynamic>> postForm(
    String path, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? form,
    bool auth = true,
  }) => _request(
    'POST',
    path,
    query: query,
    form: form,
    auth: auth,
    contentType: Headers.formUrlEncodedContentType,
  );

  Future<Map<String, dynamic>> postJson(
    String path, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? body,
    bool auth = true,
  }) => _request(
    'POST',
    path,
    query: query,
    jsonBody: body,
    auth: auth,
    contentType: Headers.jsonContentType,
  );

  Future<Map<String, dynamic>> postMultipart(
    String path, {
    Map<String, dynamic>? query,
    required FormData form,
    bool auth = true,
  }) => _request('POST', path, query: query, formData: form, auth: auth);

  /// Login posts to an absolute URL outside `/open`.
  Future<Map<String, dynamic>> postAbsoluteForm(
    String url,
    Map<String, dynamic> form,
  ) => _execute(
    url,
    'POST',
    form: form,
    contentType: Headers.formUrlEncodedContentType,
  );

  Future<Map<String, dynamic>> getAbsolute(
    String url, {
    Map<String, dynamic>? query,
  }) => _execute(url, 'GET', query: query);

  Future<Map<String, dynamic>> _request(
    String method,
    String path, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? form,
    Map<String, dynamic>? jsonBody,
    FormData? formData,
    bool auth = true,
    String? contentType,
  }) {
    final separator = path.startsWith('/') ? '' : '/';
    final url = '$baseUrl$separator$path';
    final parameters = <String, dynamic>{...?query};
    if (auth && token != null && token!.isNotEmpty) {
      parameters['oauth_token'] = token;
    }
    return _execute(
      url,
      method,
      query: parameters,
      form: form,
      jsonBody: jsonBody,
      formData: formData,
      contentType: contentType,
    );
  }

  Future<Map<String, dynamic>> _execute(
    String url,
    String method, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? form,
    Map<String, dynamic>? jsonBody,
    FormData? formData,
    String? contentType,
  }) async {
    try {
      final response = await dio.request<dynamic>(
        url,
        queryParameters: query,
        data: formData ?? form ?? jsonBody,
        options: Options(
          method: method,
          contentType: contentType,
          responseType: ResponseType.plain,
          validateStatus: (_) => true,
        ),
      );

      final status = response.statusCode ?? 0;
      final raw = response.data;
      if (raw == null || (raw is String && raw.trim().isEmpty)) {
        if (status >= 200 && status < 300) {
          return const {};
        }
        throw ApiException('$status', 'HTTP $status');
      }

      final Map<String, dynamic> json;
      if (raw is Map) {
        json = raw.cast<String, dynamic>();
      } else {
        final text = raw.toString().trim();
        if (!text.startsWith('{')) {
          throw ApiException(null, text);
        }
        final decoded = jsonDecode(text);
        if (decoded is! Map) {
          throw ApiException(null, 'Unexpected response: $text');
        }
        json = decoded.cast<String, dynamic>();
      }

      final code = json['code']?.toString();
      if (code != null && code != '0' && code != '0000') {
        throw ApiException(code, json['msg']?.toString() ?? '请求失败');
      }
      return json;
    } on DioException catch (error) {
      throw ApiException(null, error.message ?? error.toString());
    }
  }
}
