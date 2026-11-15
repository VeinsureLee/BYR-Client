import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Board metadata. Fields recovered from `remote/model/board.dart`.
class Board extends ApiEnvelope {
  final String? name;
  final List<User> managers;
  final String? description;
  final String? boardClass;
  final String? section;
  final bool? isFavorite;
  final int? threadsTodayCount;
  final bool? allowAttachment;
  final int? postTodayCount;
  final int? postThreadsCount;
  final int? postAllCount;
  final int? userOnlineCount;
  final bool? isReadOnly;
  final bool? isNoReply;
  final bool? allowAnonymous;
  final bool? allowOutgo;
  final bool? allowPost;
  final int? userOnlineMaxCount;
  final String? userOnlineMaxTime;
  final Pagination? pagination;
  final List<Article> articles;

  const Board({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.name,
    this.managers = const [],
    this.description,
    this.boardClass,
    this.section,
    this.isFavorite,
    this.threadsTodayCount,
    this.allowAttachment,
    this.postTodayCount,
    this.postThreadsCount,
    this.postAllCount,
    this.userOnlineCount,
    this.isReadOnly,
    this.isNoReply,
    this.allowAnonymous,
    this.allowOutgo,
    this.allowPost,
    this.userOnlineMaxCount,
    this.userOnlineMaxTime,
    this.pagination,
    this.articles = const [],
  });

  factory Board.fromJson(Map<String, dynamic> json) => Board(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    name: asString(json['name']),
    managers: _parseManagers(json['manager']),
    description: asString(json['description']),
    boardClass: asString(json['class']),
    section: _stringOrName(json['section']),
    isFavorite: asBool(json['is_favorite']),
    threadsTodayCount: asInt(json['threads_today_count']),
    allowAttachment: asBool(json['allow_attachment']),
    postTodayCount: asInt(json['post_today_count']),
    postThreadsCount: asInt(json['post_threads_count']),
    postAllCount: asInt(json['post_all_count']),
    userOnlineCount: asInt(json['user_online_count']),
    isReadOnly: asBool(json['is_read_only']),
    isNoReply: asBool(json['is_no_reply']),
    allowAnonymous: asBool(json['allow_anonymous']),
    allowOutgo: asBool(json['allow_outgo']),
    allowPost: asBool(json['allow_post']),
    userOnlineMaxCount: asInt(json['user_online_max_count']),
    userOnlineMaxTime: asString(json['user_online_max_time']),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
    articles: parseList(json['article'], Article.fromJson),
  );

  static List<User> _parseManagers(dynamic value) {
    if (value is Map) return [User.fromJson(value.cast<String, dynamic>())];
    if (value is List) {
      return value
          .map((item) {
            if (item is Map) return User.fromJson(item.cast<String, dynamic>());
            return User(userName: asString(item));
          })
          .toList(growable: false);
    }
    return const [];
  }

  static String? _stringOrName(dynamic value) {
    if (value is Map) return asString(value['name']);
    return asString(value);
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'name': name,
    'manager': managers.map((e) => e.toJson()).toList(),
    'description': description,
    'class': boardClass,
    'section': section,
    'is_favorite': isFavorite,
    'threads_today_count': threadsTodayCount,
    'allow_attachment': allowAttachment,
    'post_today_count': postTodayCount,
    'post_threads_count': postThreadsCount,
    'post_all_count': postAllCount,
    'user_online_count': userOnlineCount,
    'is_read_only': isReadOnly,
    'is_no_reply': isNoReply,
    'allow_anonymous': allowAnonymous,
    'allow_outgo': allowOutgo,
    'allow_post': allowPost,
    'user_online_max_count': userOnlineMaxCount,
    'user_online_max_time': userOnlineMaxTime,
    'pagination': pagination?.toJson(),
    'article': articles.map((e) => e.toJson()).toList(),
  };
}
