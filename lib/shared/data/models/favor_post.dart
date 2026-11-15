import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Favorited posts (`remote/model/favor_post.dart`).
class FavorPost extends ApiEnvelope {
  final List<Article> articles;
  final Pagination? pagination;

  const FavorPost({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.articles = const [],
    this.pagination,
  });

  factory FavorPost.fromJson(Map<String, dynamic> json) => FavorPost(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    articles: parseList(json['article'], Article.fromJson),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'article': articles.map((e) => e.toJson()).toList(),
    'pagination': pagination?.toJson(),
  };
}
