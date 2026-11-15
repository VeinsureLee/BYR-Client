import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/section.dart';

/// Article search result (`SearchArticle` in `remote/model/search.dart`).
class SearchArticle extends ApiEnvelope {
  final Pagination? pagination;
  final List<Article> articles;

  const SearchArticle({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.pagination,
    this.articles = const [],
  });

  factory SearchArticle.fromJson(Map<String, dynamic> json) => SearchArticle(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
    articles: parseList(json['article'], Article.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'pagination': pagination?.toJson(),
    'article': articles.map((e) => e.toJson()).toList(),
  };
}

/// Board search result (`SearchBoard` in `remote/model/search.dart`).
class SearchBoard extends ApiEnvelope {
  final Section? section;
  final List<Board> boards;

  const SearchBoard({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.section,
    this.boards = const [],
  });

  factory SearchBoard.fromJson(Map<String, dynamic> json) => SearchBoard(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    section: json['section'] is Map
        ? Section.fromJson(asMap(json['section']))
        : null,
    boards: parseList(json['board'], Board.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'section': section?.toJson(),
    'board': boards.map((e) => e.toJson()).toList(),
  };
}
