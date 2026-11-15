import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Encrypted response envelope (`remote/model/encrypted_data.dart`).
class EncryptedData extends ApiEnvelope {
  final String? content;
  final String? publicKey;
  final String? sign;

  const EncryptedData({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.content,
    this.publicKey,
    this.sign,
  });

  factory EncryptedData.fromJson(Map<String, dynamic> json) => EncryptedData(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    content: asString(json['content']),
    publicKey: asString(json['public_key']),
    sign: asString(json['sign']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'content': content,
    'public_key': publicKey,
    'sign': sign,
  };
}
