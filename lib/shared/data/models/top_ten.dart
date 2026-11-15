import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Top-ten entry (`remote/model/top_ten.dart`).
class TopTen extends ApiEnvelope {
  final String? name;
  final String? title;
  final String? time;
  final List<Article> articles;

  const TopTen({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.name,
    this.title,
    this.time,
    this.articles = const [],
  });

  factory TopTen.fromJson(Map<String, dynamic> json) => TopTen(
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
