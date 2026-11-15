import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Successful login response (`remote/model/o_auth_info.dart`).
class OAuthInfo extends ApiEnvelope {
  final String? accessToken;
  final int? expiresIn;

  const OAuthInfo({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.accessToken,
    this.expiresIn,
  });

  factory OAuthInfo.fromJson(Map<String, dynamic> json) => OAuthInfo(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    accessToken: asString(json['access_token'] ?? json['oauth_token']),
    expiresIn: asInt(json['expires_in']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'access_token': accessToken,
    'expires_in': expiresIn,
  };
}

/// Request body built by `OAuthInfoRequestBody.make`.
class OAuthInfoRequestBody {
  final String appkey;
  final String source;
  final String username;
  final String password;

  const OAuthInfoRequestBody({
    required this.appkey,
    required this.source,
    required this.username,
    required this.password,
  });

  Map<String, String> toJson() => {
    'appkey': appkey,
    'source': source,
    'username': username,
    'password': password,
  };
}
