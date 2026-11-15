import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Welcome image (`remote/model/welcome.dart`).
class Welcome extends ApiEnvelope {
  final int? ver;
  final String? url;
  final String? sign;

  const Welcome({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.ver,
    this.url,
    this.sign,
  });

  factory Welcome.fromJson(Map<String, dynamic> json) => Welcome(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    ver: asInt(json['ver']),
    url: asString(json['url']),
    sign: asString(json['sign']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'ver': ver,
    'url': url,
    'sign': sign,
  };
}
