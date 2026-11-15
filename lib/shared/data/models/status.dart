import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Generic status response (`remote/model/status.dart`).
class Status extends ApiEnvelope {
  final String? status;

  const Status({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.status,
  });

  factory Status.fromJson(Map<String, dynamic> json) => Status(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    status: asString(json['status']),
  );

  @override
  Map<String, dynamic> toJson() => {...super.toJson(), 'status': status};
}
