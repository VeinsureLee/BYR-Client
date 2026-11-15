import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Post/article. Fields recovered from `remote/model/article.dart`.
class Article extends ApiEnvelope {
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
  final int? replyCount;
  final String? lastReplyUserId;
  final String? lastReplyTime;
  final String? content;
  final List<Attachment> attachments;
  final String? previousId;
  final String? nextId;
  final String? threadsPreviousId;
  final String? threadsNextId;
  final int? likeSum;
  final bool? isLiked;
  final int? votedownSum;
  final bool? isVotedown;
  final bool? hiddenByVotedown;
  final int? idCount;

  const Article({
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
    this.replyCount,
    this.lastReplyUserId,
    this.lastReplyTime,
    this.content,
    this.attachments = const [],
    this.previousId,
    this.nextId,
    this.threadsPreviousId,
    this.threadsNextId,
    this.likeSum,
    this.isLiked,
    this.votedownSum,
    this.isVotedown,
    this.hiddenByVotedown,
    this.idCount,
  });

  factory Article.fromJson(Map<String, dynamic> json) => Article(
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
    replyCount: asInt(json['reply_count']),
    lastReplyUserId: asString(json['last_reply_user_id']),
    lastReplyTime: asString(json['last_reply_time']),
    content: asString(json['content']),
    attachments: parseList(json['attachment'], Attachment.fromJson),
    previousId: asString(json['previous_id']),
    nextId: asString(json['next_id']),
    threadsPreviousId: asString(json['threads_previous_id']),
    threadsNextId: asString(json['threads_next_id']),
    likeSum: asInt(json['like_sum']),
    isLiked: asBool(json['is_liked']),
    votedownSum: asInt(json['votedown_sum']),
    isVotedown: asBool(json['is_votedown']),
    hiddenByVotedown: asBool(json['hidden_by_votedown']),
    idCount: asInt(json['id_count']),
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
    'reply_count': replyCount,
    'last_reply_user_id': lastReplyUserId,
    'last_reply_time': lastReplyTime,
    'content': content,
    'attachment': attachments.map((e) => e.toJson()).toList(),
    'previous_id': previousId,
    'next_id': nextId,
    'threads_previous_id': threadsPreviousId,
    'threads_next_id': threadsNextId,
    'like_sum': likeSum,
    'is_liked': isLiked,
    'votedown_sum': votedownSum,
    'is_votedown': isVotedown,
    'hidden_by_votedown': hiddenByVotedown,
    'id_count': idCount,
  };
}
