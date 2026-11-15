import 'package:byr_client/shared/data/models/json_utils.dart';

/// Common response envelope used by every `/open` endpoint.
///
/// Recovered from `package:byr_flutter/remote/model/request.dart`:
/// `{ request, code, msg, error }`.
class ApiEnvelope {
  final String? request;
  final String? code;
  final String? msg;
  final String? error;

  const ApiEnvelope({this.request, this.code, this.msg, this.error});

  factory ApiEnvelope.fromJson(Map<String, dynamic> json) => ApiEnvelope(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
  );

  /// nForum treats a missing/zero code as success; `1701`/`0010` are errors.
  bool get isOk => code == null || code == '0' || code == '0000';

  Map<String, dynamic> toJson() => {
    'request': request,
    'code': code,
    'msg': msg,
    'error': error,
  };
}

/// Request body model (kept for fidelity with the original `Request` class).
class ApiRequest extends ApiEnvelope {
  const ApiRequest({super.request, super.code, super.msg, super.error});
}

class Pagination {
  final int? pageAllCount;
  final int? pageCurrentCount;
  final int? itemPageCount;
  final int? itemAllCount;

  const Pagination({
    this.pageAllCount,
    this.pageCurrentCount,
    this.itemPageCount,
    this.itemAllCount,
  });

  factory Pagination.fromJson(Map<String, dynamic> json) => Pagination(
    pageAllCount: asInt(json['page_all_count']),
    pageCurrentCount: asInt(json['page_current_count']),
    itemPageCount: asInt(json['item_page_count']),
    itemAllCount: asInt(json['item_all_count']),
  );

  Map<String, dynamic> toJson() => {
    'page_all_count': pageAllCount,
    'page_current_count': pageCurrentCount,
    'item_page_count': itemPageCount,
    'item_all_count': itemAllCount,
  };
}
