import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Refer notification entry (`ReferArticle` in `remote/model/refer.dart`).
class ReferArticle extends ApiEnvelope {
  final String? index;
  final String? id;
  final String? groupId;
  final String? replyId;
  final String? boardName;
  final int? pos;
  final User? user;
  final String? title;
  final String? time;
  final bool? isRead;

  const ReferArticle({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.index,
    this.id,
    this.groupId,
    this.replyId,
    this.boardName,
    this.pos,
    this.user,
    this.title,
    this.time,
    this.isRead,
  });

  factory ReferArticle.fromJson(Map<String, dynamic> json) => ReferArticle(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    index: asString(json['index']),
    id: asString(json['id']),
    groupId: asString(json['group_id']),
    replyId: asString(json['reply_id']),
    boardName: asString(json['board_name']),
    pos: asInt(json['pos']),
    user: json['user'] is Map ? User.fromJson(asMap(json['user'])) : null,
    title: asString(json['title']),
    time: asString(json['time']),
    isRead: asBool(json['is_read']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'index': index,
    'id': id,
    'group_id': groupId,
    'reply_id': replyId,
    'board_name': boardName,
    'pos': pos,
    'user': user?.toJson(),
    'title': title,
    'time': time,
    'is_read': isRead,
  };
}

/// Refer info summary (`ReferInfo` in `remote/model/refer.dart`).
class ReferInfo extends ApiEnvelope {
  final String? description;
  final bool? enable;
  final int? newCount;

  const ReferInfo({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.description,
    this.enable,
    this.newCount,
  });

  factory ReferInfo.fromJson(Map<String, dynamic> json) => ReferInfo(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    description: asString(json['description']),
    enable: asBool(json['enable']),
    newCount: asInt(json['new_count']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'description': description,
    'enable': enable,
    'new_count': newCount,
  };
}

/// Refer list (`remote/model/refer.dart`).
class Refer extends ApiEnvelope {
  final Pagination? pagination;
  final List<ReferArticle> articles;

  const Refer({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.pagination,
    this.articles = const [],
  });

  factory Refer.fromJson(Map<String, dynamic> json) => Refer(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
    articles: parseList(json['article'], ReferArticle.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'pagination': pagination?.toJson(),
    'article': articles.map((e) => e.toJson()).toList(),
  };
}
