import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// One mail message (`Mail` in `remote/model/mail_box.dart`).
class Mail extends ApiEnvelope {
  final String? index;
  final bool? isM;
  final bool? isRead;
  final bool? isReply;
  final bool? hasAttachment;
  final String? title;
  final User? user;
  final String? postTime;
  final String? boxName;
  final String? content;
  final List<Attachment> attachments;

  const Mail({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.index,
    this.isM,
    this.isRead,
    this.isReply,
    this.hasAttachment,
    this.title,
    this.user,
    this.postTime,
    this.boxName,
    this.content,
    this.attachments = const [],
  });

  factory Mail.fromJson(Map<String, dynamic> json) => Mail(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    index: asString(json['index']),
    isM: asBool(json['is_m']),
    isRead: asBool(json['is_read']),
    isReply: asBool(json['is_reply']),
    hasAttachment: asBool(json['has_attachment']),
    title: asString(json['title']),
    user: json['user'] is Map ? User.fromJson(asMap(json['user'])) : null,
    postTime: asString(json['post_time']),
    boxName: asString(json['box_name']),
    content: asString(json['content']),
    attachments: parseList(json['attachment'], Attachment.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'index': index,
    'is_m': isM,
    'is_read': isRead,
    'is_reply': isReply,
    'has_attachment': hasAttachment,
    'title': title,
    'user': user?.toJson(),
    'post_time': postTime,
    'box_name': boxName,
    'content': content,
    'attachment': attachments.map((e) => e.toJson()).toList(),
  };
}

/// Mailbox page (`remote/model/mail_box.dart`).
class MailBox extends ApiEnvelope {
  final String? description;
  final int? newNum;
  final Pagination? pagination;
  final List<Mail> mails;

  const MailBox({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.description,
    this.newNum,
    this.pagination,
    this.mails = const [],
  });

  factory MailBox.fromJson(Map<String, dynamic> json) => MailBox(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    description: asString(json['description']),
    newNum: asInt(json['new_num']),
    pagination: json['pagination'] is Map
        ? Pagination.fromJson(asMap(json['pagination']))
        : null,
    mails: parseList(json['mail'], Mail.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'description': description,
    'new_num': newNum,
    'pagination': pagination?.toJson(),
    'mail': mails.map((e) => e.toJson()).toList(),
  };
}
