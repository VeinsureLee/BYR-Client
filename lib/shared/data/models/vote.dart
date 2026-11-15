import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/user.dart';

/// Vote status entry (`VoteStatus` in `remote/model/vote.dart`).
class VoteStatus {
  final String? viid;
  final String? time;
  final String? label;
  final int? num;

  const VoteStatus({this.viid, this.time, this.label, this.num});

  factory VoteStatus.fromJson(Map<String, dynamic> json) => VoteStatus(
    viid: asString(json['viid']),
    time: asString(json['time']),
    label: asString(json['label']),
    num: asInt(json['num']),
  );

  Map<String, dynamic> toJson() => {
    'viid': viid,
    'time': time,
    'label': label,
    'num': num,
  };
}

/// One vote option (`Option` in `remote/model/vote.dart`).
class VoteOption {
  final String? viid;
  final String? label;
  final int? num;

  const VoteOption({this.viid, this.label, this.num});

  factory VoteOption.fromJson(Map<String, dynamic> json) => VoteOption(
    viid: asString(json['viid']),
    label: asString(json['label']),
    num: asInt(json['num']),
  );

  Map<String, dynamic> toJson() => {'viid': viid, 'label': label, 'num': num};
}

/// Vote request body (`VoteMultiplePostBody`).
class VoteMultiplePostBody {
  final String? vid;
  final List<String> options;

  const VoteMultiplePostBody({this.vid, this.options = const []});

  Map<String, dynamic> toJson() => {'vid': vid, 'options': options};
}

/// Vote (`remote/model/vote.dart`).
class Vote extends ApiEnvelope {
  final String? vid;
  final String? title;
  final String? start;
  final String? end;
  final int? userCount;
  final int? type;
  final int? limit;
  final String? aid;
  final bool? isEnd;
  final bool? isDeleted;
  final bool? isResultVoted;
  final User? user;
  final dynamic voteStatus;
  final dynamic voted;
  final List<VoteOption> options;
  final int? voteCount;

  const Vote({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.vid,
    this.title,
    this.start,
    this.end,
    this.userCount,
    this.type,
    this.limit,
    this.aid,
    this.isEnd,
    this.isDeleted,
    this.isResultVoted,
    this.user,
    this.voteStatus,
    this.voted,
    this.options = const [],
    this.voteCount,
  });

  factory Vote.fromJson(Map<String, dynamic> json) => Vote(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    vid: asString(json['vid']),
    title: asString(json['title']),
    start: asString(json['start']),
    end: asString(json['end']),
    userCount: asInt(json['user_count']),
    type: asInt(json['type']),
    limit: asInt(json['limit']),
    aid: asString(json['aid']),
    isEnd: asBool(json['is_end']),
    isDeleted: asBool(json['is_deleted']),
    isResultVoted: asBool(json['is_result_voted']),
    user: json['user'] is Map ? User.fromJson(asMap(json['user'])) : null,
    voteStatus: json['vote_status'] is Map
        ? VoteStatus.fromJson(asMap(json['vote_status']))
        : json['vote_status'],
    voted: json['voted'],
    options: parseList(json['options'], VoteOption.fromJson),
    voteCount: asInt(json['vote_count']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'vid': vid,
    'title': title,
    'start': start,
    'end': end,
    'user_count': userCount,
    'type': type,
    'limit': limit,
    'aid': aid,
    'is_end': isEnd,
    'is_deleted': isDeleted,
    'is_result_voted': isResultVoted,
    'user': user?.toJson(),
    'vote_status': voteStatus is VoteStatus
        ? (voteStatus as VoteStatus).toJson()
        : voteStatus,
    'voted': voted,
    'options': options.map((e) => e.toJson()).toList(),
    'vote_count': voteCount,
  };
}
