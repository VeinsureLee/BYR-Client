import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Recommended board (`remote/model/recommend_board.dart`).
class RecommendBoard extends ApiEnvelope {
  final String? name;
  final String? title;
  final String? time;
  final List<Article> articles;

  const RecommendBoard({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.name,
    this.title,
    this.time,
    this.articles = const [],
  });

  factory RecommendBoard.fromJson(Map<String, dynamic> json) => RecommendBoard(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    name: asString(json['name']),
    title: asString(json['title']),
    time: asString(json['time']),
    articles: parseList(json['article'], Article.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'name': name,
    'title': title,
    'time': time,
    'article': articles.map((e) => e.toJson()).toList(),
  };
}
