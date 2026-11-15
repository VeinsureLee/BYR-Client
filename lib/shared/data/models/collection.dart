import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// One entry of the article-favorites list (`/open/collection.json`).
///
/// The server returns these under the `article` key, but their shape differs
/// from a normal [Article]:
/// `{"gid":1241765,"bname":"WorkLife","title":"...","num":21,...}`
class CollectionArticle {
  final int? createdTime;
  final String? title;
  final int? postTime;
  final String? bname;
  final int? num;
  final int? gid;
  final User? user;

  const CollectionArticle({
    this.createdTime,
    this.title,
    this.postTime,
    this.bname,
    this.num,
    this.gid,
    this.user,
  });

  factory CollectionArticle.fromJson(Map<String, dynamic> json) =>
      CollectionArticle(
        createdTime: asInt(json['createdTime']),
        title: asString(json['title']),
        postTime: asInt(json['postTime']),
        bname: asString(json['bname']),
        num: asInt(json['num']),
        gid: asInt(json['gid']),
        user: json['user'] is Map ? User.fromJson(asMap(json['user'])) : null,
      );

  Map<String, dynamic> toJson() => {
    'createdTime': createdTime,
    'title': title,
    'postTime': postTime,
    'bname': bname,
    'num': num,
    'gid': gid,
    'user': user?.toJson(),
  };
}

class Collection extends ApiEnvelope {
  final List<CollectionArticle> entries;
  final Pagination? pagination;

  const Collection({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.entries = const [],
    this.pagination,
  });

  factory Collection.fromJson(Map<String, dynamic> json) => Collection(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    // Real responses use `article`; keep `collection` for the older shape.
    entries: parseList(
      json['article'] ?? json['collection'],
      CollectionArticle.fromJson,
    ),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'article': entries.map((e) => e.toJson()).toList(),
    'pagination': pagination?.toJson(),
  };
}
