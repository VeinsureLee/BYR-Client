import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Thread (topic plus reply list). Fields recovered from
/// `remote/model/thread.dart`.
class Thread extends ApiEnvelope {
  final String? id;
  final String? groupId;
  final String? replyId;
  final String? flag;
  final int? position;
  final bool? isTop;
  final bool? isSubject;
  final bool? hasAttachment;
  final bool? isAdmin;
  final String? title;
  final User? user;
  final String? postTime;
  final String? boardName;
  final String? boardDescription;
  final int? likeSum;
  final bool? isLiked;
  final int? votedownSum;
  final bool? isVotedown;
  final bool? hiddenByVotedown;
  final bool? collect;
  final int? replyCount;
  final String? lastReplyUserId;
  final String? lastReplyTime;
  final List<Article> likeArticles;
  final Pagination? pagination;
  final List<Article> articles;

  const Thread({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.id,
    this.groupId,
    this.replyId,
    this.flag,
    this.position,
    this.isTop,
    this.isSubject,
    this.hasAttachment,
    this.isAdmin,
    this.title,
    this.user,
    this.postTime,
    this.boardName,
    this.boardDescription,
    this.likeSum,
    this.isLiked,
    this.votedownSum,
    this.isVotedown,
    this.hiddenByVotedown,
    this.collect,
    this.replyCount,
    this.lastReplyUserId,
    this.lastReplyTime,
    this.likeArticles = const [],
    this.pagination,
    this.articles = const [],
  });

  factory Thread.fromJson(Map<String, dynamic> json) => Thread(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    id: asString(json['id']),
    groupId: asString(json['group_id']),
    replyId: asString(json['reply_id']),
    flag: asString(json['flag']),
    position: asInt(json['position']),
    isTop: asBool(json['is_top']),
    isSubject: asBool(json['is_subject']),
    hasAttachment: asBool(json['has_attachment']),
    isAdmin: asBool(json['is_admin']),
    title: asString(json['title']),
    user: json['user'] is Map ? User.fromJson(asMap(json['user'])) : null,
    postTime: asString(json['post_time']),
    boardName: asString(json['board_name']),
    boardDescription: asString(json['board_description']),
    likeSum: asInt(json['like_sum']),
    isLiked: asBool(json['is_liked']),
    votedownSum: asInt(json['votedown_sum']),
    isVotedown: asBool(json['is_votedown']),
    hiddenByVotedown: asBool(json['hidden_by_votedown']),
    collect: asBool(json['collect']),
    replyCount: asInt(json['reply_count']),
    lastReplyUserId: asString(json['last_reply_user_id']),
    lastReplyTime: asString(json['last_reply_time']),
    likeArticles: parseList(json['like_articles'], Article.fromJson),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
    articles: parseList(json['article'], Article.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'id': id,
    'group_id': groupId,
    'reply_id': replyId,
    'flag': flag,
    'position': position,
    'is_top': isTop,
    'is_subject': isSubject,
    'has_attachment': hasAttachment,
    'is_admin': isAdmin,
    'title': title,
    'user': user?.toJson(),
    'post_time': postTime,
    'board_name': boardName,
    'board_description': boardDescription,
    'like_sum': likeSum,
    'is_liked': isLiked,
    'votedown_sum': votedownSum,
    'is_votedown': isVotedown,
    'hidden_by_votedown': hiddenByVotedown,
    'collect': collect,
    'reply_count': replyCount,
    'last_reply_user_id': lastReplyUserId,
    'last_reply_time': lastReplyTime,
    'like_articles': likeArticles.map((e) => e.toJson()).toList(),
    'pagination': pagination?.toJson(),
    'article': articles.map((e) => e.toJson()).toList(),
  };
}
