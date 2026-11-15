import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Blacklist page (`remote/model/black_list.dart`).
class BlackList extends ApiEnvelope {
  final List<User> users;
  final Pagination? pagination;

  const BlackList({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.users = const [],
    this.pagination,
  });

  factory BlackList.fromJson(Map<String, dynamic> json) => BlackList(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    users: parseList(json['user'], User.fromJson),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'user': users.map((e) => e.toJson()).toList(),
    'pagination': pagination?.toJson(),
  };
}
